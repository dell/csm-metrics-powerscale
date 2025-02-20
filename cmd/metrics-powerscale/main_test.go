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
	"os"
	"testing"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockEntrypoint is a mock for the entrypoint.Run function
// type MockEntrypoint struct {
// 	mock.Mock
// }

// func (m *MockEntrypoint) Run(ctx context.Context, config *entrypoint.Config, exporter otlexporters.Otlexporter, powerScaleSvc service.Service) error {
// 	args := m.Called(ctx, config, exporter, powerScaleSvc)
// 	return args.Error(0)
// }

// // entrypointRun is a function variable to mock entrypoint.Run
// var entrypointRun = entrypoint.Run

// // osExit is a function variable to mock os.Exit
// var osExit = os.Exit

// func TestMainFunction(t *testing.T) {
// 	// Backup original functions and restore them after the test
// 	oldEntrypointRun := entrypointRun
// 	oldOsExit := osExit
// 	defer func() {
// 		entrypointRun = oldEntrypointRun
// 		osExit = oldOsExit
// 	}()

// 	// Setup mock for entrypoint.Run
// 	mockEntrypoint := new(MockEntrypoint)
// 	entrypointRun = mockEntrypoint.Run

// 	// Setup mock for os.Exit to capture exit codes
// 	exitCode := -1
// 	osExit = func(code int) {
// 		exitCode = code
// 	}

// 	// Set test environment variables
// 	os.Setenv("LOG_FORMAT", "json")
// 	os.Setenv("LOG_LEVEL", "debug")
// 	os.Setenv("COLLECTOR_ADDR", "localhost:4317")
// 	os.Setenv("POWERSCALE_CAPACITY_METRICS_ENABLED", "true")
// 	os.Setenv("POWERSCALE_PERFORMANCE_METRICS_ENABLED", "true")

// 	// Mock Viper configuration
// 	viper.SetConfigFile(defaultConfigFile)
// 	viper.ReadInConfig()
// 	viper.WatchConfig()
// 	viper.OnConfigChange(func(e fsnotify.Event) {})

// 	// Create a test context with timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 	defer cancel()

// 	// Simulate the main function execution in a goroutine
// 	go func() {
// 		main()
// 		cancel() // Ensure test exits
// 	}()

// 	// Wait for context to be done or timeout
// 	<-ctx.Done()

// 	// Verify entrypoint.Run was called with expected arguments
// 	mockEntrypoint.AssertCalled(t, "Run", mock.Anything, mock.AnythingOfType("*entrypoint.Config"), mock.AnythingOfType("*otlexporters.OtlCollectorExporter"), mock.AnythingOfType("*service.PowerScaleService"))

//		// Verify no fatal errors occurred (os.Exit not called)
//		assert.Equal(t, -1, exitCode, "main() exited unexpectedly")
//	}
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
		t.Run(tt.name, func(t *testing.T) {
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
		name            string
		quotaFreq       string
		clusterCapFreq  string
		clusterPerfFreq string
		expectedQuota   time.Duration
		expectedCap     time.Duration
		expectedPerf    time.Duration
		expectPanic     bool
	}{
		{
			name:            "Valid Values",
			quotaFreq:       "30",
			clusterCapFreq:  "25",
			clusterPerfFreq: "15",
			expectedQuota:   30 * time.Second,
			expectedCap:     25 * time.Second,
			expectedPerf:    15 * time.Second,
			expectPanic:     false,
		},
		{
			name:            "Invalid Quota",
			quotaFreq:       "invalid",
			clusterCapFreq:  "",
			clusterPerfFreq: "",
			expectedQuota:   defaultTickInterval,
			expectedCap:     defaultTickInterval,
			expectedPerf:    defaultTickInterval,
			expectPanic:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("POWERSCALE_QUOTA_CAPACITY_POLL_FREQUENCY", tt.quotaFreq)
			viper.Set("POWERSCALE_CLUSTER_CAPACITY_POLL_FREQUENCY", tt.clusterCapFreq)
			viper.Set("POWERSCALE_CLUSTER_PERFORMANCE_POLL_FREQUENCY", tt.clusterPerfFreq)

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

func TestSetupConfigWatchers(t *testing.T) {
	logger := logrus.New()
	config := &entrypoint.Config{}
	var exporter *otlexporters.OtlCollectorExporter
	exporter = &otlexporters.OtlCollectorExporter{}
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
