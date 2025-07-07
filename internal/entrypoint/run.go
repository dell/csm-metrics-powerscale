/*
 Copyright (c) 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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

// ConfigValidatorFunc is used to override config validation in testing
var ConfigValidatorFunc = ValidateConfig

// Config holds data that will be used by the service
type Config struct {
	LeaderElector                  pscaleService.LeaderElector
	ClusterCapacityTickInterval    time.Duration
	ClusterPerformanceTickInterval time.Duration
	QuotaCapacityTickInterval      time.Duration
	CapacityMetricsEnabled         bool
	PerformanceMetricsEnabled      bool
	CollectorAddress               string
	CollectorCertPath              string
	Logger                         *logrus.Logger
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

	// set initial tick intervals
	ClusterCapacityTickInterval := config.ClusterCapacityTickInterval
	clusterCapacityTicker := time.NewTicker(ClusterCapacityTickInterval)
	ClusterPerformanceTickInterval := config.ClusterPerformanceTickInterval
	clusterPerformanceTicker := time.NewTicker(ClusterPerformanceTickInterval)
	QuotaCapacityTickInterval := config.QuotaCapacityTickInterval
	quotaCapacityTicker := time.NewTicker(QuotaCapacityTickInterval)

	fmt.Printf("QuotaCapacityTickInterval %v\n", QuotaCapacityTickInterval)

	for {
		select {
		case <-clusterCapacityTicker.C:
			if !config.LeaderElector.IsLeader() {
				logger.Info("not leader pod to collect metrics")
				continue
			}
			if !config.CapacityMetricsEnabled {
				logger.Info("powerscale cluster capacity metrics collection is disabled")
				continue
			}

			powerScaleSvc.ExportClusterCapacityMetrics(ctx)
		case <-clusterPerformanceTicker.C:
			if !config.LeaderElector.IsLeader() {
				logger.Info("not leader pod to collect metrics")
				continue
			}
			if !config.PerformanceMetricsEnabled {
				logger.Info("powerscale cluster performance metrics collection is disabled")
				continue
			}
			powerScaleSvc.ExportClusterPerformanceMetrics(ctx)
		case <-quotaCapacityTicker.C:
			if !config.LeaderElector.IsLeader() {
				logger.Info("not leader pod to collect metrics")
				continue
			}
			if !config.CapacityMetricsEnabled {
				logger.Info("powerscale quota capacity metrics collection is disabled")
				continue
			}
			powerScaleSvc.ExportQuotaMetrics(ctx)
			powerScaleSvc.ExportTopologyMetrics(ctx)
		case err := <-errCh:
			if err == nil {
				continue
			}
			return err
		case <-ctx.Done():
			return nil
		}

		// check if tick interval config settings have changed
		if ClusterCapacityTickInterval != config.ClusterCapacityTickInterval {
			ClusterCapacityTickInterval = config.ClusterCapacityTickInterval
			clusterCapacityTicker = time.NewTicker(ClusterCapacityTickInterval)
		}
		if ClusterPerformanceTickInterval != config.ClusterPerformanceTickInterval {
			ClusterPerformanceTickInterval = config.ClusterPerformanceTickInterval
			clusterPerformanceTicker = time.NewTicker(ClusterPerformanceTickInterval)
		}
		if QuotaCapacityTickInterval != config.QuotaCapacityTickInterval {
			QuotaCapacityTickInterval = config.QuotaCapacityTickInterval
			quotaCapacityTicker = time.NewTicker(QuotaCapacityTickInterval)
		}
	}
}

// ValidateConfig will validate the configuration and return any errors
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("no config provided")
	}

	if config.ClusterCapacityTickInterval > MaximumTickInterval || config.ClusterCapacityTickInterval < MinimumTickInterval {
		return fmt.Errorf("cluster capacity polling frequency not within allowed range of %v and %v", MinimumTickInterval.String(), MaximumTickInterval.String())
	}

	if config.ClusterPerformanceTickInterval > MaximumTickInterval || config.ClusterPerformanceTickInterval < MinimumTickInterval {
		return fmt.Errorf("cluster performance polling frequency not within allowed range of %v and %v", MinimumTickInterval.String(), MaximumTickInterval.String())
	}

	if config.QuotaCapacityTickInterval > MaximumTickInterval || config.QuotaCapacityTickInterval < MinimumTickInterval {
		return fmt.Errorf("quota capacity polling frequency not within allowed range of %v and %v", MinimumTickInterval.String(), MaximumTickInterval.String())
	}

	return nil
}
