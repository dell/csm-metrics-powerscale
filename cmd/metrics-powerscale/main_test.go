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
			viper.Set("provisioner_names", tt.provisioners)

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
