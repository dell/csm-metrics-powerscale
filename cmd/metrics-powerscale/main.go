/*
 Copyright (c) 2025 Dell Inc. or its subsidiaries. All Rights Reserved.

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

	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/pscaleresource"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/dell/csmlog"
	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

const (
	defaultTickInterval            = 20 * time.Second
	defaultConfigFile              = "/etc/config/karavi-metrics-powerscale.yaml"
	defaultStorageSystemConfigFile = "/isilon-creds/config"
)

// Added for testing purposes
var getPowerScaleClusters = pscaleresource.GetPowerScaleClusters

func main() {
	config, exporter, powerScaleSvc := initializeComponents()

	if err := entrypoint.Run(context.Background(), config, exporter, powerScaleSvc); err != nil {
		csmlog.Fatalf("running service: %v", err)
	}
}

func initializeComponents() (*entrypoint.Config, *otlexporters.OtlCollectorExporter, *service.PowerScaleService) {
	initLogging()

	configFileListener := setupConfigFileListener()
	leaderElector := &k8s.LeaderElector{API: &k8s.LeaderElector{}}
	config := setupConfig(leaderElector)
	exporter := &otlexporters.OtlCollectorExporter{}
	powerScaleSvc := setupPowerScaleService()

	applyInitialConfigUpdates(config, exporter, powerScaleSvc)

	// Watch for config changes and update settings dynamically
	setupConfigWatchers(configFileListener, config, exporter, powerScaleSvc)

	return config, exporter, powerScaleSvc
}

// initLogging loads config and applies initial logging settings.
func initLogging() {
	loadConfig()
	updateLoggingSettings()
}

// loadConfig loads the primary configuration file.
func loadConfig() {
	viper.SetConfigFile(defaultConfigFile)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to read Config file: %v", err)
	}
}

// setupConfigFileListener initializes a secondary config watcher for storage system configs.
func setupConfigFileListener() *viper.Viper {
	configFileListener := viper.New()
	configFileListener.SetConfigFile(defaultStorageSystemConfigFile)
	return configFileListener
}

// setupConfig creates the main configuration structure.
func setupConfig(leaderElector *k8s.LeaderElector) *entrypoint.Config {
	return &entrypoint.Config{
		LeaderElector:     leaderElector,
		CollectorCertPath: getCollectorCertPath(),
	}
}

// getCollectorCertPath retrieves the certificate path for the OpenTelemetry collector.
func getCollectorCertPath() string {
	if tls := os.Getenv("TLS_ENABLED"); tls == "true" {
		if certPath := strings.TrimSpace(os.Getenv("COLLECTOR_CERT_PATH")); certPath != "" {
			return certPath
		}
	}
	return otlexporters.DefaultCollectorCertPath
}

// setupPowerScaleService initializes the PowerScale service.
func setupPowerScaleService() *service.PowerScaleService {
	return &service.PowerScaleService{
		MetricsWrapper: &service.MetricsWrapper{
			Meter: otel.Meter("powerscale"),
		},
		VolumeFinder:       &k8s.VolumeFinder{API: &k8s.API{}},
		StorageClassFinder: &k8s.StorageClassFinder{API: &k8s.API{}},
	}
}

// applyInitialConfigUpdates applies all necessary updates before starting the service.
func applyInitialConfigUpdates(config *entrypoint.Config, exporter *otlexporters.OtlCollectorExporter, powerScaleSvc *service.PowerScaleService) {
	updateLoggingSettings()
	updateCollectorAddress(config, exporter)
	updateMetricsEnabled(config)
	updateTickIntervals(config)
	updatePowerScaleConnection(powerScaleSvc, powerScaleSvc.StorageClassFinder.(*k8s.StorageClassFinder), powerScaleSvc.VolumeFinder.(*k8s.VolumeFinder))
	updateService(powerScaleSvc)
	updateObservabilityMetrics(config, powerScaleSvc)
}

// setupConfigWatchers sets up dynamic updates when config files change.
func setupConfigWatchers(configFileListener *viper.Viper, config *entrypoint.Config, exporter *otlexporters.OtlCollectorExporter, powerScaleSvc *service.PowerScaleService) {
	viper.WatchConfig()
	viper.OnConfigChange(func(_ fsnotify.Event) {
		applyInitialConfigUpdates(config, exporter, powerScaleSvc)
	})

	configFileListener.WatchConfig()
	configFileListener.OnConfigChange(func(_ fsnotify.Event) {
		updatePowerScaleConnection(powerScaleSvc, powerScaleSvc.StorageClassFinder.(*k8s.StorageClassFinder), powerScaleSvc.VolumeFinder.(*k8s.VolumeFinder))
	})
}

// updateLoggingSettings updates logging format and level dynamically.
func updateLoggingSettings() {
	logFormat := viper.GetString("LOG_FORMAT")
	if strings.EqualFold(logFormat, "json") {
		csmlog.SetFormat("json")
	} else {
		csmlog.SetFormat("text")
	}

	logLevel := viper.GetString("LOG_LEVEL")
	level, err := csmlog.ParseLevel(logLevel)
	if err != nil {
		level = csmlog.InfoLevel
	}
	csmlog.SetLevel(level)
}

func updatePowerScaleConnection(powerScaleSvc *service.PowerScaleService, storageClassFinder *k8s.StorageClassFinder, volumeFinder *k8s.VolumeFinder) {
	clusters, defaultCluster, err := getPowerScaleClusters(defaultStorageSystemConfigFile)
	if err != nil {
		csmlog.Fatalf("initialize clusters in controller service: %v", err)
	}
	powerScaleClients := make(map[string]service.PowerScaleClient)
	clientIsiPaths := make(map[string]string)
	clusterNames := make([]k8s.ClusterName, len(clusters))

	for clusterName, cluster := range clusters {
		powerScaleClients[clusterName] = cluster.Client
		csmlog.WithFields(csmlog.Fields{"cluster_name": clusterName}).Debug("setting powerscale client from configuration")
		clientIsiPaths[clusterName] = cluster.IsiPath

		clusterName := k8s.ClusterName{
			ID:        clusterName,
			IsDefault: cluster.IsDefault,
		}
		clusterNames = append(clusterNames, clusterName)
	}

	storageClassFinder.ClusterNames = clusterNames
	powerScaleSvc.SetPowerScaleClients(powerScaleClients)
	powerScaleSvc.ClientIsiPaths = clientIsiPaths
	powerScaleSvc.DefaultPowerScaleCluster = defaultCluster

	updateProvisionerNames(volumeFinder, storageClassFinder)
}

func updateCollectorAddress(config *entrypoint.Config, exporter *otlexporters.OtlCollectorExporter) {
	collectorAddress := viper.GetString("COLLECTOR_ADDR")
	if collectorAddress == "" {
		csmlog.Fatal("COLLECTOR_ADDR is required")
	}
	config.CollectorAddress = collectorAddress
	exporter.CollectorAddr = collectorAddress
	csmlog.WithFields(csmlog.Fields{"collector_address": collectorAddress}).Debug("setting collector address")
}

func updateProvisionerNames(volumeFinder *k8s.VolumeFinder, storageClassFinder *k8s.StorageClassFinder) {
	provisionerNamesValue := viper.GetString("provisioner_names")
	if provisionerNamesValue == "" {
		csmlog.Fatal("PROVISIONER_NAMES is required")
	}
	provisionerNames := strings.Split(provisionerNamesValue, ",")
	volumeFinder.DriverNames = provisionerNames

	for i := range storageClassFinder.ClusterNames {
		storageClassFinder.ClusterNames[i].DriverNames = provisionerNames
	}

	csmlog.WithFields(csmlog.Fields{"provisioner_names": provisionerNamesValue}).Debug("setting provisioner names")
}

func updateMetricsEnabled(config *entrypoint.Config) {
	capacityMetricsEnabled := true
	capacityMetricsEnabledValue := viper.GetString("POWERSCALE_CAPACITY_METRICS_ENABLED")
	if capacityMetricsEnabledValue == "false" {
		capacityMetricsEnabled = false
	}
	config.CapacityMetricsEnabled = capacityMetricsEnabled
	csmlog.WithFields(csmlog.Fields{"capacity_metrics_enabled": capacityMetricsEnabled}).Debug("setting capacity metrics enabled")

	performanceMetricsEnabled := true
	performanceMetricsEnabledValue := viper.GetString("POWERSCALE_PERFORMANCE_METRICS_ENABLED")
	if performanceMetricsEnabledValue == "false" {
		performanceMetricsEnabled = false
	}
	config.PerformanceMetricsEnabled = performanceMetricsEnabled
	csmlog.WithFields(csmlog.Fields{"performance_metrics_enabled": performanceMetricsEnabled}).Debug("setting performance metrics enabled")

	topologyMetricsEnabled := true
	topologyMetricsEnabledValue := viper.GetString("POWERSCALE_TOPOLOGY_METRICS_ENABLED")
	if topologyMetricsEnabledValue == "false" {
		topologyMetricsEnabled = false
	}
	config.TopologyMetricsEnabled = topologyMetricsEnabled
	csmlog.WithFields(csmlog.Fields{"topology_metrics_enabled": topologyMetricsEnabled}).Debug("setting topology metrics enabled")
}

func updateTickIntervals(config *entrypoint.Config) {
	quotaCapacityTickInterval := defaultTickInterval
	quotaCapacityPollFrequencySeconds := viper.GetString("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY")
	if quotaCapacityPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(quotaCapacityPollFrequencySeconds)
		if err != nil {
			csmlog.Fatalf("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY was not set to a valid number: %v", err)
		}
		quotaCapacityTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.QuotaCapacityTickInterval = quotaCapacityTickInterval
	csmlog.WithFields(csmlog.Fields{"quota_capacity_tick_interval": fmt.Sprintf("%v", quotaCapacityTickInterval)}).Debug("setting quota capacity tick interval")

	clusterCapacityTickInterval := defaultTickInterval
	clusterCapacityPollFrequencySeconds := viper.GetString("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY")
	if clusterCapacityPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(clusterCapacityPollFrequencySeconds)
		if err != nil {
			csmlog.Fatalf("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY was not set to a valid number: %v", err)
		}
		clusterCapacityTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.ClusterCapacityTickInterval = clusterCapacityTickInterval
	csmlog.WithFields(csmlog.Fields{"cluster_capacity_tick_interval": fmt.Sprintf("%v", clusterCapacityTickInterval)}).Debug("setting cluster capacity tick interval")

	clusterPerformanceTickInterval := defaultTickInterval
	clusterPerformancePollFrequencySeconds := viper.GetString("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY")
	if clusterPerformancePollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(clusterPerformancePollFrequencySeconds)
		if err != nil {
			csmlog.Fatalf("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY was not set to a valid number: %v", err)
		}
		clusterPerformanceTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.ClusterPerformanceTickInterval = clusterPerformanceTickInterval
	csmlog.WithFields(csmlog.Fields{"cluster_performance_tick_interval": fmt.Sprintf("%v", clusterPerformanceTickInterval)}).Debug("setting cluster performance tick interval")

	topologyMetricsTickInterval := defaultTickInterval
	topologyMetricsPollFrequencySeconds := viper.GetString("POWERSCALE_TOPOLOGY_METRICS_POLL_FREQUENCY")
	if topologyMetricsPollFrequencySeconds != "" {
		numSeconds, err := strconv.Atoi(topologyMetricsPollFrequencySeconds)
		if err != nil {
			csmlog.Fatalf("POWERSCALE_TOPOLOGY_METRICS_POLL_FREQUENCY was not set to a valid number: %v", err)
		}
		topologyMetricsTickInterval = time.Duration(numSeconds) * time.Second
	}
	config.TopologyMetricsTickInterval = topologyMetricsTickInterval
	csmlog.WithFields(csmlog.Fields{"cluster_performance_tick_interval": fmt.Sprintf("%v", topologyMetricsTickInterval)}).Debug("setting cluster performance tick interval")
}

func getObservabilityMetricsListenAddress() string {
	port := viper.GetString("X_CSI_METRICS_PORT")
	if port == "" {
		return entrypoint.DefaultPrometheusListenAddress
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

func getObservabilityMetricsTLSFiles() (string, string) {
	certFile := strings.TrimSpace(os.Getenv("X_CSI_METRICS_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("X_CSI_METRICS_TLS_KEY_FILE"))
	return certFile, keyFile
}

func updateObservabilityMetrics(config *entrypoint.Config, powerScaleSvc *service.PowerScaleService) {
	if viper.GetString("X_CSI_METRICS_ENABLED") != "true" {
		config.PrometheusMetricsEnabled = false
		config.PrometheusRegistry = nil
		config.PrometheusListenAddress = ""
		config.PrometheusCertFile = ""
		config.PrometheusKeyFile = ""
		powerScaleSvc.ObsInstrumenter = nil
		return
	}
	if config.PrometheusRegistry == nil {
		config.PrometheusRegistry = prometheus.NewRegistry()
		powerScaleSvc.ObsInstrumenter = service.NewPSCObsInstrumenter(config.PrometheusRegistry)
		csmlog.Info("observability self-metrics enabled: prometheus registry and instrumenter initialized")
	}
	config.PrometheusMetricsEnabled = true
	config.PrometheusListenAddress = getObservabilityMetricsListenAddress()
	config.PrometheusCertFile, config.PrometheusKeyFile = getObservabilityMetricsTLSFiles()
}

func updateService(pscaleSvc *service.PowerScaleService) {
	maxPowerScaleConcurrentRequests := service.DefaultMaxPowerScaleConnections
	maxPowerScaleConcurrentRequestsVar := viper.GetString("POWERSCALE_MAX_CONCURRENT_QUERIES")
	if maxPowerScaleConcurrentRequestsVar != "" {
		maxPowerScaleConcurrentRequests, err := strconv.Atoi(maxPowerScaleConcurrentRequestsVar)
		if err != nil {
			csmlog.Fatalf("POWERSCALE_MAX_CONCURRENT_QUERIES was not set to a valid number: %v", err)
		}
		if maxPowerScaleConcurrentRequests <= 0 {
			csmlog.Fatalf("POWERSCALE_MAX_CONCURRENT_QUERIES value was invalid (<= 0): %v", err)
		}
	}
	pscaleSvc.MaxPowerScaleConnections = maxPowerScaleConcurrentRequests
	csmlog.WithFields(csmlog.Fields{"max_connections": maxPowerScaleConcurrentRequests}).Debug("setting max powerscale connections")
}
