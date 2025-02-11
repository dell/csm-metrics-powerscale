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

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/dell/csm-metrics-powerscale/internal/common"
	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/sirupsen/logrus"

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
			Meter: otel.Meter("powerscale"),
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
	viper.OnConfigChange(func(_ fsnotify.Event) {
		updateLoggingSettings(logger)
		updateCollectorAddress(config, exporter, logger)
		updatePowerScaleConnection(powerScaleSvc, storageClassFinder, volumeFinder, logger)
		updateMetricsEnabled(config, logger)
		updateTickIntervals(config, logger)
		updateService(powerScaleSvc, logger)
	})

	configFileListener.WatchConfig()
	configFileListener.OnConfigChange(func(_ fsnotify.Event) {
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

		clusterName := k8s.ClusterName{
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
		logger.Error("COLLECTOR_ADDR is required")
		return
	}
	config.CollectorAddress = collectorAddress
	exporter.CollectorAddr = collectorAddress
	logger.WithField("collector_address", collectorAddress).Debug("setting collector address")
}

func updateProvisionerNames(volumeFinder *k8s.VolumeFinder, storageClassFinder *k8s.StorageClassFinder, logger *logrus.Logger) {
	provisionerNamesValue := viper.GetString("provisioner_names")
	if provisionerNamesValue == "" {
		logger.Error("PROVISIONER_NAMES is required")
		return
	}
	provisionerNames := strings.Split(provisionerNamesValue, ",")
	volumeFinder.DriverNames = provisionerNames

	for i := range storageClassFinder.ClusterNames {
		storageClassFinder.ClusterNames[i].DriverNames = provisionerNames
	}

	logger.WithField("provisioner_names", provisionerNamesValue).Debug("setting provisioner names")
}

func updateMetricsEnabled(config *entrypoint.Config, logger *logrus.Logger) {
	capacityMetricsEnabled := true
	capacityMetricsEnabledValue := viper.GetString("POWERSCALE_CAPACITY_METRICS_ENABLED")
	if capacityMetricsEnabledValue == "false" {
		capacityMetricsEnabled = false
	}
	config.CapacityMetricsEnabled = capacityMetricsEnabled
	logger.WithField("capacity_metrics_enabled", capacityMetricsEnabled).Debug("setting capacity metrics enabled")

	performanceMetricsEnabled := true
	performanceMetricsEnabledValue := viper.GetString("POWERSCALE_PERFORMANCE_METRICS_ENABLED")
	if performanceMetricsEnabledValue == "false" {
		performanceMetricsEnabled = false
	}
	config.PerformanceMetricsEnabled = performanceMetricsEnabled
	logger.WithField("performance_metrics_enabled", performanceMetricsEnabled).Debug("setting performance metrics enabled")
}

func updateTickIntervals(config *entrypoint.Config, logger *logrus.Logger) {
	quotaCapacityTickInterval := defaultTickInterval
	quotaCapacityPollFrequencySeconds := viper.GetString("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY")
	if quotaCapacityPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(quotaCapacityPollFrequencySeconds)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY was not set to a valid number")
		}
		quotaCapacityTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.QuotaCapacityTickInterval = quotaCapacityTickInterval
	logger.WithField("quota_capacity_tick_interval", fmt.Sprintf("%v", quotaCapacityTickInterval)).Debug("setting quota capacity tick interval")

	clusterCapacityTickInterval := defaultTickInterval
	clusterCapacityPollFrequencySeconds := viper.GetString("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY")
	if clusterCapacityPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(clusterCapacityPollFrequencySeconds)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY was not set to a valid number")
		}
		clusterCapacityTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.ClusterCapacityTickInterval = clusterCapacityTickInterval
	logger.WithField("cluster_capacity_tick_interval", fmt.Sprintf("%v", clusterCapacityTickInterval)).Debug("setting cluster capacity tick interval")

	clusterPerformanceTickInterval := defaultTickInterval
	clusterPerformancePollFrequencySeconds := viper.GetString("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY")
	if clusterPerformancePollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(clusterPerformancePollFrequencySeconds)
		if err != nil {
			logger.WithError(err).Fatal("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY was not set to a valid number")
		}
		clusterPerformanceTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.ClusterPerformanceTickInterval = clusterPerformanceTickInterval
	logger.WithField("cluster_performance_tick_interval", fmt.Sprintf("%v", clusterPerformanceTickInterval)).Debug("setting cluster performance tick interval")
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
