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

package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"
	"github.com/dell/gopowerscale"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// gatherObsMetric returns the MetricFamily for the given name from reg, or nil.
func gatherObsMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// gaugeObsValue returns the gauge value for the given label set, or (0, false).
func gaugeObsValue(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

// counterObsValue returns the counter value for the given label set, or (0, false).
func counterObsValue(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

// histogramObsSum returns the sum value for the given label set, or (0, false).
func histogramObsSum(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetHistogram().GetSampleSum(), true
		}
	}
	return 0, false
}

// histogramObsCount returns the sample count for the given label set, or (0, false).
func histogramObsCount(mf *dto.MetricFamily, labels map[string]string) (uint64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetHistogram().GetSampleCount(), true
		}
	}
	return 0, false
}

// U-OBS-PS-10: ExportClusterCapacityMetrics with a healthy cluster sets connectivity=1.
func TestObsWiring_ExportClusterCapacityMetrics_Connected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordClusterCapacityStatsMetrics(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	statsFile := "testdata/recordings/platform-3-statistics-current.json"
	contentBytes, _ := os.ReadFile(statsFile)
	var stats gopowerscale.FloatStats
	_ = json.Unmarshal(contentBytes, &stats)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(stats, nil).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:  metricsRecorder,
		ObsInstrumenter: inst,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportClusterCapacityMetrics(context.Background())

	labels := map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"}

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf, "dell_csm_obs_array_connectivity must be present")
	v, ok := gaugeObsValue(mf, labels)
	require.True(t, ok, "cluster1 label must be present in connectivity metric")
	assert.Equal(t, 1.0, v, "connected cluster must have connectivity=1")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "collection rate must be positive when metrics are collected")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_processing_latency_seconds")
	require.NotNil(t, mf)
	count, ok := histogramObsCount(mf, labels)
	require.True(t, ok)
	assert.Equal(t, uint64(1), count, "one latency observation per cycle")
	latencySum, ok := histogramObsSum(mf, labels)
	require.True(t, ok)
	assert.Less(t, latencySum, 1.0, "processing latency must exclude collection API time and be sub-second with mock client")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "successful export must increment success counter")
}

// U-OBS-PS-11: ExportClusterCapacityMetrics with a failing cluster sets connectivity=0.
func TestObsWiring_ExportClusterCapacityMetrics_Disconnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(nil, errors.New("connection refused")).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:  metricsRecorder,
		ObsInstrumenter: inst,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportClusterCapacityMetrics(context.Background())

	labels := map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"}

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Equal(t, 0.0, v, "disconnected cluster must have connectivity=0")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Equal(t, 0.0, v, "collection rate must be zero when no metrics are collected")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "error"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "failed export must increment error counter")
}

// U-OBS-PS-11A: Backend export failure must set status=error even when the cluster is reachable.
func TestObsWiring_ExportClusterCapacityMetrics_ExportFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordClusterCapacityStatsMetrics(gomock.Any(), gomock.Any()).Return(errors.New("backend write failed")).AnyTimes()

	statsFile := "testdata/recordings/platform-3-statistics-current.json"
	contentBytes, _ := os.ReadFile(statsFile)
	var stats gopowerscale.FloatStats
	_ = json.Unmarshal(contentBytes, &stats)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(stats, nil).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:  metricsRecorder,
		ObsInstrumenter: inst,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportClusterCapacityMetrics(context.Background())

	labels := map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"}

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "reachable cluster must keep connectivity=1 even if export fails")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "collection rate must stay positive when metrics were collected before export failure")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "backend export failure must increment the failure counter")
}

// U-OBS-PS-12: ExportClusterPerformanceMetrics with a healthy cluster sets connectivity=1.
func TestObsWiring_ExportClusterPerformanceMetrics_Connected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordClusterPerformanceStatsMetrics(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	statsFile := "testdata/recordings/platform-3-statistics-current.json"
	contentBytes, _ := os.ReadFile(statsFile)
	var stats gopowerscale.FloatStats
	_ = json.Unmarshal(contentBytes, &stats)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(stats, nil).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:  metricsRecorder,
		ObsInstrumenter: inst,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportClusterPerformanceMetrics(context.Background())

	labels := map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"}

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "connected cluster must have connectivity=1")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, labels)
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "collection rate must be positive when performance metrics are collected")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "successful export must increment success counter")
}

// U-OBS-PS-13: ExportQuotaMetrics with a failing client sets connectivity=0.
func TestObsWiring_ExportQuotaMetrics_Disconnected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordVolumeQuota(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	metricsRecorder.EXPECT().RecordClusterQuota(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	volFinder := mocks.NewMockVolumeFinder(ctrl)
	volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return(nil, nil).AnyTimes()

	scFinder := mocks.NewMockStorageClassFinder(ctrl)
	scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return(nil, nil).AnyTimes()

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetAllQuotas(gomock.Any()).Return(nil, errors.New("timeout")).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:     metricsRecorder,
		ObsInstrumenter:    inst,
		VolumeFinder:       volFinder,
		StorageClassFinder: scFinder,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportQuotaMetrics(context.Background())

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"})
	require.True(t, ok)
	assert.Equal(t, 0.0, v, "disconnected cluster must have connectivity=0")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"})
	require.True(t, ok)
	assert.Equal(t, 0.0, v, "collection rate must be zero when quota collection fails")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "error"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "failed quota export must increment error counter")
}

// U-OBS-PS-14: ExportQuotaMetrics with a healthy client sets connectivity=1.
func TestObsWiring_ExportQuotaMetrics_Connected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := prometheus.NewRegistry()
	inst := service.NewPSCObsInstrumenter(reg)

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordVolumeQuota(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	metricsRecorder.EXPECT().RecordClusterQuota(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	volFinder := mocks.NewMockVolumeFinder(ctrl)
	volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return(nil, nil).AnyTimes()

	scFinder := mocks.NewMockStorageClassFinder(ctrl)
	scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return(nil, nil).AnyTimes()

	quotaFile := "testdata/recordings/client2-quotas.json"
	quotaBytes, _ := os.ReadFile(quotaFile)
	var quotas gopowerscale.QuotaList
	_ = json.Unmarshal(quotaBytes, &quotas)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetAllQuotas(gomock.Any()).Return(quotas, nil).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:     metricsRecorder,
		ObsInstrumenter:    inst,
		VolumeFinder:       volFinder,
		StorageClassFinder: scFinder,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	svc.ExportQuotaMetrics(context.Background())

	mf := gatherObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "connected cluster must have connectivity=1")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf)
	v, ok = gaugeObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1"})
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "collection rate must be positive when quota metrics are collected")

	mf = gatherObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)
	v, ok = counterObsValue(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster1", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v, "successful quota export must increment success counter")
}

// U-OBS-PS-15: nil ObsInstrumenter does not panic during export functions.
func TestObsWiring_NilInstrumenter_NoPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metricsRecorder := mocks.NewMockMetricsRecorder(ctrl)
	metricsRecorder.EXPECT().RecordClusterCapacityStatsMetrics(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	statsFile := "testdata/recordings/platform-3-statistics-current.json"
	contentBytes, _ := os.ReadFile(statsFile)
	var stats gopowerscale.FloatStats
	_ = json.Unmarshal(contentBytes, &stats)

	client := mocks.NewMockPowerScaleClient(ctrl)
	client.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(stats, nil).AnyTimes()

	svc := service.PowerScaleService{
		MetricsWrapper:  metricsRecorder,
		ObsInstrumenter: nil,
		PowerScaleClients: map[string]service.PowerScaleClient{
			"cluster1": client,
		},
	}

	assert.NotPanics(t, func() {
		svc.ExportClusterCapacityMetrics(context.Background())
	})
}
