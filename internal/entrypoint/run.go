// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package entrypoint

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	pscaleService "github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"google.golang.org/grpc/credentials"
)

const (
	// MaximumTickInterval is the maximum allowed interval when querying metrics
	MaximumTickInterval = 10 * time.Minute
	// MinimumTickInterval is the minimum allowed interval when querying metrics
	MinimumTickInterval = 5 * time.Second
	// DefaultEndPoint for leader election path
	DefaultEndPoint = "karavi-metrics-powerscale"
	// DefaultNameSpace for PowerScale pod running metrics collection
	DefaultNameSpace = "karavi"
)

var (
	// ConfigValidatorFunc is used to override config validation in testing
	ConfigValidatorFunc func(*Config) error = ValidateConfig
)

// Config holds data that will be used by the service
type Config struct {
	VolumeTickInterval   time.Duration
	ClusterTickInterval  time.Duration
	LeaderElector        pscaleService.LeaderElector
	VolumeMetricsEnabled bool
	CollectorAddress     string
	CollectorCertPath    string
	Logger               *logrus.Logger
}

// Run is the entry point for starting the service
func Run(ctx context.Context, config *Config, exporter otlexporters.Otlexporter, powerScaleSvc pscaleService.Service) error {
	err := ConfigValidatorFunc(config)
	if err != nil {
		return err
	}
	logger := config.Logger

	errCh := make(chan error, 1)

	go func() {
		powerscaleEndpoint := os.Getenv("POWERSCALE_METRICS_ENDPOINT")
		if powerscaleEndpoint == "" {
			powerscaleEndpoint = DefaultEndPoint
		}
		powerscaleNamespace := os.Getenv("POWERSCALE_METRICS_NAMESPACE")
		if powerscaleNamespace == "" {
			powerscaleNamespace = DefaultNameSpace
		}
		errCh <- config.LeaderElector.InitLeaderElection(powerscaleEndpoint, powerscaleNamespace)
	}()

	go func() {
		options := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(config.CollectorAddress),
		}

		if config.CollectorCertPath != "" {

			transportCreds, err := credentials.NewClientTLSFromFile(config.CollectorCertPath, "")
			if err != nil {
				errCh <- err
			}
			options = append(options, otlpmetricgrpc.WithTLSCredentials(transportCreds))
		} else {
			options = append(options, otlpmetricgrpc.WithInsecure())
		}

		errCh <- exporter.InitExporter(options...)
	}()

	defer exporter.StopExporter()

	runtime.GOMAXPROCS(runtime.NumCPU())

	//set initial tick intervals
	VolumeTickInterval := config.VolumeTickInterval
	volumeTicker := time.NewTicker(VolumeTickInterval)
	ClusterTickInterval := config.ClusterTickInterval
	clusterTicker := time.NewTicker(ClusterTickInterval)

	for {
		select {
		case <-volumeTicker.C:
			if !config.LeaderElector.IsLeader() {
				logger.Info("not leader pod to collect metrics")
				continue
			}
			if !config.VolumeMetricsEnabled {
				logger.Info("powerscale volume metrics collection is disabled")
				continue
			}

			powerScaleSvc.ExportVolumeMetrics(ctx)
		case <-clusterTicker.C:
			if !config.LeaderElector.IsLeader() {
				logger.Info("not leader pod to collect metrics")
				continue
			}
			if !config.VolumeMetricsEnabled {
				logger.Info("powerscale cluster metrics collection is disabled")
				continue
			}
			powerScaleSvc.ExportClusterMetrics(ctx)
		case err := <-errCh:
			if err == nil {
				continue
			}
			return err
		case <-ctx.Done():
			return nil
		}

		//check if tick interval config settings have changed
		if VolumeTickInterval != config.VolumeTickInterval {
			VolumeTickInterval = config.VolumeTickInterval
			volumeTicker = time.NewTicker(VolumeTickInterval)
		}
		if ClusterTickInterval != config.ClusterTickInterval {
			ClusterTickInterval = config.ClusterTickInterval
			clusterTicker = time.NewTicker(ClusterTickInterval)
		}
	}
}

// ValidateConfig will validate the configuration and return any errors
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("no config provided")
	}

	if config.VolumeTickInterval > MaximumTickInterval || config.VolumeTickInterval < MinimumTickInterval {
		return fmt.Errorf("volume polling frequency not within allowed range of %v and %v", MinimumTickInterval.String(), MaximumTickInterval.String())
	}

	if config.ClusterTickInterval > MaximumTickInterval || config.ClusterTickInterval < MinimumTickInterval {
		return fmt.Errorf("cluster polling frequency not within allowed range of %v and %v", MinimumTickInterval.String(), MaximumTickInterval.String())
	}

	return nil
}
