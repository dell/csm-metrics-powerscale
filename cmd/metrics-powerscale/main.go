// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/common"
	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"

	"github.com/sirupsen/logrus"

	"os"

	"go.opentelemetry.io/otel/metric/global"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

const (
	defaultTickInterval            = 20 * time.Second
	defaultConfigFile              = "/etc/config/karavi-metrics-powerscale.yaml"
	defaultStorageSystemConfigFile = "/isilon-creds/config"
)

func main() {

	logger := logrus.New()

	viper.SetConfigFile(defaultConfigFile)

	err := viper.ReadInConfig()
	// if unable to read configuration file, proceed in case we use environment variables
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to read Config file: %v", err)
	}

	configFileListener := viper.New()
	configFileListener.SetConfigFile(defaultStorageSystemConfigFile)

	leaderElectorGetter := &k8s.LeaderElector{
		API: &k8s.LeaderElector{},
	}

	updateLoggingSettings := func(logger *logrus.Logger) {
		logFormat := viper.GetString("LOG_FORMAT")
		if strings.EqualFold(logFormat, "json") {
			logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			// use text formatter by default
			logger.SetFormatter(&logrus.TextFormatter{})
		}
		logLevel := viper.GetString("LOG_LEVEL")
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			// use INFO level by default
			level = logrus.InfoLevel
		}
		logger.SetLevel(level)
	}

	updateLoggingSettings(logger)

	volumeFinder := &k8s.VolumeFinder{
		API:    &k8s.API{},
		Logger: logger,
	}

	storageClassFinder := &k8s.StorageClassFinder{
		API:    &k8s.API{},
		Logger: logger,
	}

	var collectorCertPath string
	if tls := os.Getenv("TLS_ENABLED"); tls == "true" {
		collectorCertPath = os.Getenv("COLLECTOR_CERT_PATH")
		if len(strings.TrimSpace(collectorCertPath)) < 1 {
			collectorCertPath = otlexporters.DefaultCollectorCertPath
		}
	}

	config := &entrypoint.Config{
		LeaderElector:     leaderElectorGetter,
		CollectorCertPath: collectorCertPath,
		Logger:            logger,
	}

	exporter := &otlexporters.OtlCollectorExporter{}

	powerScaleSvc := &service.PowerScaleService{
		MetricsWrapper: &service.MetricsWrapper{
			Meter: global.Meter("powerscale"),
		},
		Logger:             logger,
		VolumeFinder:       volumeFinder,
		StorageClassFinder: storageClassFinder,
	}

	updatePowerScaleConnection(powerScaleSvc, storageClassFinder, volumeFinder, logger)
	updateCollectorAddress(config, exporter, logger)
	updateMetricsEnabled(config, logger)
	updateTickIntervals(config, logger)
	updateService(powerScaleSvc, logger)

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		updateLoggingSettings(logger)
		updateCollectorAddress(config, exporter, logger)
		updatePowerScaleConnection(powerScaleSvc, storageClassFinder, volumeFinder, logger)
		updateMetricsEnabled(config, logger)
		updateTickIntervals(config, logger)
		updateService(powerScaleSvc, logger)
	})

	configFileListener.WatchConfig()
	configFileListener.OnConfigChange(func(e fsnotify.Event) {
		updatePowerScaleConnection(powerScaleSvc, storageClassFinder, volumeFinder, logger)
	})

	if err := entrypoint.Run(context.Background(), config, exporter, powerScaleSvc); err != nil {
		logger.WithError(err).Fatal("running service")
	}
}

func updatePowerScaleConnection(powerScaleSvc *service.PowerScaleService, storageClassFinder *k8s.StorageClassFinder, volumeFinder *k8s.VolumeFinder, logger *logrus.Logger) {
	clusters, defaultCluster, err := common.GetPowerScaleClusters(defaultStorageSystemConfigFile, logger)
	if err != nil {
		logger.WithError(err).Fatal("initialize clusters in controller service")
	}
	powerScaleClients := make(map[string]service.PowerScaleClient)
	clientIsiPaths := make(map[string]string)
	clusterNames := make([]k8s.ClusterName, len(clusters))

	for clusterName, cluster := range clusters {
		powerScaleClients[clusterName] = cluster.Client
		logger.WithField("cluster_name", clusterName).Debug("setting powerscale client from configuration")
		clientIsiPaths[clusterName] = cluster.IsiPath

		var clusterName = k8s.ClusterName{
			ID:        clusterName,
			IsDefault: cluster.IsDefault,
		}
		clusterNames = append(clusterNames, clusterName)
	}

	storageClassFinder.ClusterNames = clusterNames
	powerScaleSvc.PowerScaleClients = powerScaleClients
	powerScaleSvc.ClientIsiPaths = clientIsiPaths
	powerScaleSvc.DefaultPowerScaleCluster = defaultCluster

	updateProvisionerNames(volumeFinder, storageClassFinder, logger)

}

func updateCollectorAddress(config *entrypoint.Config, exporter *otlexporters.OtlCollectorExporter, logger *logrus.Logger) {
	collectorAddress := viper.GetString("COLLECTOR_ADDR")
	if collectorAddress == "" {
		logger.Fatal("COLLECTOR_ADDR is required")
	}
	config.CollectorAddress = collectorAddress
	exporter.CollectorAddr = collectorAddress
	logger.WithField("collector_address", collectorAddress).Debug("setting collector address")
}

func updateProvisionerNames(volumeFinder *k8s.VolumeFinder, storageClassFinder *k8s.StorageClassFinder, logger *logrus.Logger) {
	provisionerNamesValue := viper.GetString("provisioner_names")
	if provisionerNamesValue == "" {
		logger.Fatal("PROVISIONER_NAMES is required")
	}
	provisionerNames := strings.Split(provisionerNamesValue, ",")
	volumeFinder.DriverNames = provisionerNames

	for i := range storageClassFinder.ClusterNames {
		storageClassFinder.ClusterNames[i].DriverNames = provisionerNames
	}

	logger.WithField("provisioner_names", provisionerNamesValue).Debug("setting provisioner names")
}

func updateMetricsEnabled(config *entrypoint.Config, logger *logrus.Logger) {
	powerscaleVolumeMetricsEnabled := true
	powerscaleVolumeMetricsEnabledValue := viper.GetString("POWERSCALE_VOLUME_METRICS_ENABLED")
	if powerscaleVolumeMetricsEnabledValue == "false" {
		powerscaleVolumeMetricsEnabled = false
	}
	config.VolumeMetricsEnabled = powerscaleVolumeMetricsEnabled
	logger.WithField("volume_metrics_enabled", powerscaleVolumeMetricsEnabled).Debug("setting volume metrics enabled")
}

func updateTickIntervals(config *entrypoint.Config, logger *logrus.Logger) {
	volumeTickInterval := defaultTickInterval
	volIoPollFrequencySeconds := viper.GetString("POWERSCALE_VOLUME_IO_POLL_FREQUENCY")
	if volIoPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(volIoPollFrequencySeconds)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_VOLUME_IO_POLL_FREQUENCY was not set to a valid number")
		}
		volumeTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.VolumeTickInterval = volumeTickInterval
	logger.WithField("volume_tick_interval", fmt.Sprintf("%v", volumeTickInterval)).Debug("setting volume tick interval")

	clusterTickInterval := defaultTickInterval
	clusterPollFrequencySeconds := viper.GetString("POWERSCALE_CLUSTER_POLL_FREQUENCY")
	if clusterPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(clusterPollFrequencySeconds)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_CLUSTER_POLL_FREQUENCY was not set to a valid number")
		}
		clusterTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.ClusterTickInterval = clusterTickInterval
	logger.WithField("cluster_tick_interval", fmt.Sprintf("%v", clusterTickInterval)).Debug("setting cluster tick interval")
}

func updateService(pscaleSvc *service.PowerScaleService, logger *logrus.Logger) {
	maxPowerScaleConcurrentRequests := service.DefaultMaxPowerScaleConnections
	maxPowerScaleConcurrentRequestsVar := viper.GetString("POWERSCALE_MAX_CONCURRENT_QUERIES")
	if maxPowerScaleConcurrentRequestsVar != "" {
		maxPowerScaleConcurrentRequests, err := strconv.Atoi(maxPowerScaleConcurrentRequestsVar)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_MAX_CONCURRENT_QUERIES was not set to a valid number")
		}
		if maxPowerScaleConcurrentRequests <= 0 {
			logger.WithError(err).Fatal("POWERSCALE_MAX_CONCURRENT_QUERIES value was invalid (<= 0)")
		}
	}
	pscaleSvc.MaxPowerScaleConnections = maxPowerScaleConcurrentRequests
	logger.WithField("max_connections", maxPowerScaleConcurrentRequests).Debug("setting max powerscale connections")
}
