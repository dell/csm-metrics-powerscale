/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/dell/goisilon"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitializeComponents(t *testing.T) {
	// Mock getPowerScaleClusters to avoid file I/O
	originalGetPowerScaleClusters := getPowerScaleClusters
	defer func() { getPowerScaleClusters = originalGetPowerScaleClusters }()
	getPowerScaleClusters = func(_ string, _ *logrus.Logger) (map[string]*service.PowerScaleCluster, *service.PowerScaleCluster, error) {
		return map[string]*service.PowerScaleCluster{
				"cluster1": {
					ClusterName: "cluster1",
					Client:      &goisilon.Client{},
					IsiPath:     "/ifs/data/csi",
					IsDefault:   true,
				},
			}, &service.PowerScaleCluster{
				ClusterName: "cluster1",
				Client:      &goisilon.Client{},
				IsiPath:     "/ifs/data/csi",
				IsDefault:   true,
			}, nil
	}

	// Mock Viper to avoid reading from the actual config file
	viper.Reset()
	viper.SetConfigType("yaml")
	viper.SetConfigFile(defaultConfigFile)

	// Mock the config file content
	configContent := `
LOG_LEVEL: debug
COLLECTOR_ADDR: localhost:4317
PROVISIONER_NAMES: csi-isilon
POWERSCALE_CAPACITY_METRICS_ENABLED: true
POWERSCALE_PERFORMANCE_METRICS_ENABLED: true
TLS_ENABLED: false
`
	err := viper.ReadConfig(strings.NewReader(configContent))
	if err != nil {
		// Handle the error or log it
		log.Printf("Error reading config: %v", err)
	}

	tests := []struct {
		name                  string
		envVars               map[string]string
		expectedLogLevel      logrus.Level
		expectedCollectorAddr string
		expectedProvisioners  []string
		expectedCertPath      string
	}{
		{
			name: "SuccessfulInitializationWithDefaults",
			envVars: map[string]string{
				"LOG_LEVEL":                              "debug",
				"COLLECTOR_ADDR":                         "localhost:4317",
				"PROVISIONER_NAMES":                      "csi-isilon",
				"POWERSCALE_CAPACITY_METRICS_ENABLED":    "true",
				"POWERSCALE_PERFORMANCE_METRICS_ENABLED": "true",
				"TLS_ENABLED":                            "false",
			},
			expectedLogLevel:      logrus.DebugLevel,
			expectedCollectorAddr: "localhost:4317",
			expectedProvisioners:  []string{"csi-isilon"},
			expectedCertPath:      otlexporters.DefaultCollectorCertPath,
		},
		{
			name: "TLSEnabledWithCustomCertPath",
			envVars: map[string]string{
				"LOG_LEVEL":           "info",
				"COLLECTOR_ADDR":      "collector:4317",
				"PROVISIONER_NAMES":   "csi-isilon",
				"TLS_ENABLED":         "true",
				"COLLECTOR_CERT_PATH": "/custom/cert/path",
			},
			expectedLogLevel:      logrus.InfoLevel,
			expectedCollectorAddr: "collector:4317",
			expectedProvisioners:  []string{"csi-isilon"},
			expectedCertPath:      "/custom/cert/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset Viper and set environment variables for each test case
			viper.Reset()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Mock the config file content for each test case
			viper.SetConfigType("yaml")
			viper.SetConfigFile(defaultConfigFile)

			err := viper.ReadConfig(strings.NewReader(configContent))
			if err != nil {
				// Handle the error or log it
				log.Printf("Error reading config: %v", err)
			}
			logger, config, exporter, svc := initializeComponents()

			// Assert components are initialized
			assert.NotNil(t, logger)
			assert.NotNil(t, config)
			assert.NotNil(t, exporter)
			assert.NotNil(t, svc)
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{"Valid log level", "info", false},
		{"Invalid log level", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("LOG_LEVEL", tt.logLevel)

			logger := setupLogger()

			// Test if logger is setup correctly and if any error occurs.
			if tt.wantErr {
				assert.Equal(t, logrus.InfoLevel, logger.Level)
			} else {
				assert.NotNil(t, logger)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"Valid config", false},
		{"Invalid config", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Simulating different config file conditions
			if tt.wantErr {
				viper.SetConfigFile("/invalid/path")
			} else {
				viper.SetConfigFile(defaultConfigFile)
			}

			// Call loadConfig
			mockLogger := logrus.New()
			loadConfig(mockLogger) // This will just load the config
			// No error handling needed because loadConfig doesn't return error; it just prints it
		})
	}
}

func TestSetupConfigFileListener(t *testing.T) {
	tests := []struct {
		name          string
		expectedError bool
	}{
		{"Valid Config File Listener", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener := setupConfigFileListener()
			assert.NotNil(t, listener, "Expected valid config file listener")
		})
	}
}

func TestSetupConfig(t *testing.T) {
	tests := []struct {
		name          string
		expectedError bool
	}{
		{"Valid Config Setup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			leaderElector := &k8s.LeaderElector{}
			config := setupConfig(logger, leaderElector)
			assert.NotNil(t, config, "Expected valid config")
		})
	}
}

func TestGetCollectorCertPath(t *testing.T) {
	t.Run("Valid Cert Path", func(t *testing.T) {
		os.Setenv("TLS_ENABLED", "true")
		os.Setenv("COLLECTOR_CERT_PATH", "/path/to/cert")
		path := getCollectorCertPath()
		assert.Equal(t, "/path/to/cert", path)
	})

	t.Run("TLS Enabled But No Cert Path", func(t *testing.T) {
		os.Setenv("TLS_ENABLED", "true")
		os.Setenv("COLLECTOR_CERT_PATH", "") // Explicitly setting it to empty
		path := getCollectorCertPath()
		assert.Equal(t, otlexporters.DefaultCollectorCertPath, path)
	})

	t.Run("TLS Disabled", func(t *testing.T) {
		os.Setenv("TLS_ENABLED", "false")
		path := getCollectorCertPath()
		assert.Equal(t, otlexporters.DefaultCollectorCertPath, path)
	})

	t.Run("TLS Not Set", func(t *testing.T) {
		os.Unsetenv("TLS_ENABLED")
		os.Unsetenv("COLLECTOR_CERT_PATH")
		path := getCollectorCertPath()
		assert.Equal(t, otlexporters.DefaultCollectorCertPath, path)
	})
}

func TestSetupPowerScaleService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"Valid setup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger
			mockLogger := logrus.New()

			// Call the actual service setup function
			got := setupPowerScaleService(mockLogger)

			// Check if we got a valid PowerScaleService instance
			if tt.wantErr {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.IsType(t, &service.PowerScaleService{}, got)
			}
		})
	}
}

func TestApplyInitialConfigUpdates(t *testing.T) {
	logger := logrus.New()
	leaderElector := &k8s.LeaderElector{}
	config := setupConfig(logger, leaderElector)

	// Mock the dependencies
	exporter := &otlexporters.OtlCollectorExporter{}
	powerScaleSvc := &service.PowerScaleService{
		Logger:             logger,
		MetricsWrapper:     &service.MetricsWrapper{},
		StorageClassFinder: &k8s.StorageClassFinder{}, // Ensure non-nil values
		VolumeFinder:       &k8s.VolumeFinder{},
	}

	// Mock getPowerScaleClusters to avoid file dependency
	originalGetPowerScaleClusters := getPowerScaleClusters
	defer func() { getPowerScaleClusters = originalGetPowerScaleClusters }()

	getPowerScaleClusters = func(_ string, _ *logrus.Logger) (map[string]*service.PowerScaleCluster, *service.PowerScaleCluster, error) {
		return map[string]*service.PowerScaleCluster{}, nil, nil
	}

	// Mock configurations
	viper.Set("COLLECTOR_ADDR", "localhost:4317")
	viper.Set("PROVISIONER_NAMES", "isilon")
	viper.Set("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY", "30") // Must be a valid number
	viper.Set("POWERSCALE_MAX_CONCURRENT_QUERIES", "5")

	// Ensure applyInitialConfigUpdates does not panic
	assert.NotPanics(t, func() {
		applyInitialConfigUpdates(config, exporter, powerScaleSvc, logger)
	}, "applyInitialConfigUpdates() should not panic")

	// Validate config values were updated
	assert.Equal(t, "localhost:4317", config.CollectorAddress)
	assert.True(t, config.CapacityMetricsEnabled)
	assert.True(t, config.PerformanceMetricsEnabled)
}

func TestSetupConfigWatchers(t *testing.T) {
	logger := logrus.New()
	config := &entrypoint.Config{}
	exporter := &otlexporters.OtlCollectorExporter{}
	powerScaleSvc := &service.PowerScaleService{}
	configFileListener := setupConfigFileListener()

	tests := []struct {
		name          string
		expectedError bool
	}{
		{"Valid Config Watchers Setup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				setupConfigWatchers(configFileListener, config, exporter, powerScaleSvc, logger)
			}, "Expected setupConfigWatchers to not panic")
		})
	}
}

type MockGetPowerScaleClusters struct {
	mock.Mock
}

func (m *MockGetPowerScaleClusters) GetPowerScaleClusters(filePath string, logger *logrus.Logger) (map[string]*service.PowerScaleCluster, *service.PowerScaleCluster, error) {
	args := m.Called(filePath, logger)
	return args.Get(0).(map[string]*service.PowerScaleCluster), args.Get(1).(*service.PowerScaleCluster), args.Error(2)
}

func TestUpdatePowerScaleConnection(t *testing.T) {
	originalGetPowerScaleClusters := getPowerScaleClusters
	defer func() { getPowerScaleClusters = originalGetPowerScaleClusters }()

	tests := []struct {
		name               string
		clusters           map[string]*service.PowerScaleCluster
		defaultCluster     *service.PowerScaleCluster
		getClustersError   error
		expectedError      bool
		expectedClientLen  int
		expectedIsiPathLen int
	}{
		{
			name: "Success - Single Cluster",
			clusters: map[string]*service.PowerScaleCluster{
				"cluster1": {
					ClusterName: "cluster1",
					Client:      &goisilon.Client{},
					IsiPath:     "/ifs/data/csi",
					IsDefault:   true,
				},
			},
			defaultCluster: &service.PowerScaleCluster{
				ClusterName: "cluster1",
				Client:      &goisilon.Client{},
				IsiPath:     "/ifs/data/csi",
				IsDefault:   true,
			},
			getClustersError:   nil,
			expectedError:      false,
			expectedClientLen:  1,
			expectedIsiPathLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockGetter := new(MockGetPowerScaleClusters)
			mockGetter.On("GetPowerScaleClusters", mock.Anything, mock.Anything).Return(tt.clusters, tt.defaultCluster, tt.getClustersError)

			// Override the function variable
			getPowerScaleClusters = mockGetter.GetPowerScaleClusters

			// Initialize service and dependencies
			logger := logrus.New()
			powerScaleSvc := &service.PowerScaleService{
				PowerScaleClients: make(map[string]service.PowerScaleClient),
				ClientIsiPaths:    make(map[string]string),
			}
			storageClassFinder := &k8s.StorageClassFinder{}
			volumeFinder := &k8s.VolumeFinder{}

			viper.Set("PROVISIONER_NAMES", "isilon")

			// Execute
			updatePowerScaleConnection(powerScaleSvc, storageClassFinder, volumeFinder, logger)

			assert.Equal(t, tt.expectedClientLen, len(powerScaleSvc.PowerScaleClients))
			assert.Equal(t, tt.expectedIsiPathLen, len(powerScaleSvc.ClientIsiPaths))
			assert.Equal(t, tt.defaultCluster, powerScaleSvc.DefaultPowerScaleCluster)
			mockGetter.AssertCalled(t, "GetPowerScaleClusters", mock.Anything, mock.Anything)
		})
	}
}

func TestUpdateCollectorAddress(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		expectPanic bool
	}{
		{
			name:        "Valid Address",
			addr:        "localhost:8080",
			expectPanic: false,
		},
		{
			name:        "Empty Address",
			addr:        "",
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("COLLECTOR_ADDR", tt.addr)

			logger := logrus.New()
			logger.ExitFunc = func(int) { panic("fatal") }
			config := &entrypoint.Config{Logger: logger}
			exporter := &otlexporters.OtlCollectorExporter{}

			if tt.expectPanic {
				assert.Panics(t, func() { updateCollectorAddress(config, exporter, logger) })
			} else {
				assert.NotPanics(t, func() { updateCollectorAddress(config, exporter, logger) })
				assert.Equal(t, tt.addr, config.CollectorAddress)
				assert.Equal(t, tt.addr, exporter.CollectorAddr)
			}
		})
	}
}

func TestUpdateMetricsEnabled(t *testing.T) {
	tests := []struct {
		name                              string
		capacityMetricsEnabled            string
		performanceMetricsEnabled         string
		expectedCapacityMetricsEnabled    bool
		expectedPerformanceMetricsEnabled bool
	}{
		{
			name:                              "Both metrics enabled",
			capacityMetricsEnabled:            "true",
			performanceMetricsEnabled:         "true",
			expectedCapacityMetricsEnabled:    true,
			expectedPerformanceMetricsEnabled: true,
		},
		{
			name:                              "Capacity metrics disabled",
			capacityMetricsEnabled:            "false",
			performanceMetricsEnabled:         "true",
			expectedCapacityMetricsEnabled:    false,
			expectedPerformanceMetricsEnabled: true,
		},
		{
			name:                              "Performance metrics disabled",
			capacityMetricsEnabled:            "true",
			performanceMetricsEnabled:         "false",
			expectedCapacityMetricsEnabled:    true,
			expectedPerformanceMetricsEnabled: false,
		},
		{
			name:                              "Both metrics disabled",
			capacityMetricsEnabled:            "false",
			performanceMetricsEnabled:         "false",
			expectedCapacityMetricsEnabled:    false,
			expectedPerformanceMetricsEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("POWERSCALE_CAPACITY_METRICS_ENABLED", tt.capacityMetricsEnabled)
			viper.Set("POWERSCALE_PERFORMANCE_METRICS_ENABLED", tt.performanceMetricsEnabled)
			config := &entrypoint.Config{}
			logger := logrus.New()
			updateMetricsEnabled(config, logger)

			assert.Equal(t, tt.expectedCapacityMetricsEnabled, config.CapacityMetricsEnabled, "Capacity metrics enabled should be set correctly")
			assert.Equal(t, tt.expectedPerformanceMetricsEnabled, config.PerformanceMetricsEnabled, "Performance metrics enabled should be set correctly")
		})
	}
}

func TestUpdateProvisionerNames(t *testing.T) {
	tests := []struct {
		name         string
		provisioners string
		expected     []string
		expectPanic  bool
	}{
		{
			name:         "Single Provisioner",
			provisioners: "csi-isilon-dellemc-com",
			expected:     []string{"csi-isilon-dellemc-com"},
			expectPanic:  false,
		},
		{
			name:         "Multiple Provisioners",
			provisioners: "csi-isilon-dellemc-com1,csi-isilon-dellemc-com2",
			expected:     []string{"csi-isilon-dellemc-com1", "csi-isilon-dellemc-com2"},
			expectPanic:  false,
		},
		{
			name:         "Empty Provisioners",
			provisioners: "",
			expected:     nil,
			expectPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("PROVISIONER_NAMES", tt.provisioners)

			vf := &k8s.VolumeFinder{}
			scf := &k8s.StorageClassFinder{}
			logger := logrus.New()
			logger.ExitFunc = func(int) { panic("fatal") }

			if tt.expectPanic {
				assert.Panics(t, func() { updateProvisionerNames(vf, scf, logger) })
			} else {
				assert.NotPanics(t, func() { updateProvisionerNames(vf, scf, logger) })
				assert.Equal(t, tt.expected, vf.DriverNames)
				for _, cluster := range scf.ClusterNames {
					assert.Equal(t, tt.expected, cluster.DriverNames)
				}
			}
		})
	}
}

func TestUpdateTickIntervals(t *testing.T) {
	tests := []struct {
		name               string
		quotaFreq          string
		clusterCapFreq     string
		clusterPerfFreq    string
		topologyMetricFreq string
		expectedQuota      time.Duration
		expectedCap        time.Duration
		expectedPerf       time.Duration
		expectPanic        bool
	}{
		{
			name:               "Valid Values",
			quotaFreq:          "30",
			clusterCapFreq:     "25",
			clusterPerfFreq:    "15",
			topologyMetricFreq: "5",
			expectedQuota:      30 * time.Second,
			expectedCap:        25 * time.Second,
			expectedPerf:       15 * time.Second,
			expectPanic:        false,
		},
		{
			name:               "Invalid Quota",
			quotaFreq:          "invalid",
			clusterCapFreq:     "",
			clusterPerfFreq:    "",
			topologyMetricFreq: "",
			expectedQuota:      defaultTickInterval,
			expectedCap:        defaultTickInterval,
			expectedPerf:       defaultTickInterval,
			expectPanic:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY", tt.quotaFreq)
			viper.Set("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY", tt.clusterCapFreq)
			viper.Set("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY", tt.clusterPerfFreq)
			viper.Set("POWERSCALE_TOPOLOGY_METRICS_POLL_FREQUENCY", tt.topologyMetricFreq)

			config := &entrypoint.Config{}
			logger := logrus.New()
			logger.ExitFunc = func(int) { panic("fatal") }

			if tt.expectPanic {
				assert.Panics(t, func() { updateTickIntervals(config, logger) })
			} else {
				assert.NotPanics(t, func() { updateTickIntervals(config, logger) })
				assert.Equal(t, tt.expectedQuota, config.QuotaCapacityTickInterval)
				assert.Equal(t, tt.expectedCap, config.ClusterCapacityTickInterval)
				assert.Equal(t, tt.expectedPerf, config.ClusterPerformanceTickInterval)
			}
		})
	}
}

func TestUpdateService(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent string
		expected      int
		expectPanic   bool
	}{
		{
			name:          "Valid Value",
			maxConcurrent: "10",
			expected:      10,
			expectPanic:   false,
		},
		{
			name:          "Invalid Value",
			maxConcurrent: "invalid",
			expected:      service.DefaultMaxPowerScaleConnections,
			expectPanic:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("POWERSCALE_MAX_CONCURRENT_QUERIES", tt.maxConcurrent)

			svc := &service.PowerScaleService{}
			logger := logrus.New()
			logger.ExitFunc = func(int) { panic("fatal") }

			if tt.expectPanic {
				assert.Panics(t, func() { updateService(svc, logger) })
			} else {
				assert.NotPanics(t, func() { updateService(svc, logger) })
				assert.Equal(t, tt.expected, svc.MaxPowerScaleConnections)
			}
		})
	}
}
