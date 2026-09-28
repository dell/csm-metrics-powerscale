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

package service

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gatherPSCObsMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
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

func metricLabelsMatchPSCObs(m *dto.Metric, labels map[string]string) bool {
	got := make(map[string]string)
	for _, lp := range m.GetLabel() {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range labels {
		if got[k] != v {
			return false
		}
	}
	return true
}

func gaugePSCObs(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		if metricLabelsMatchPSCObs(m, labels) {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

func counterPSCObs(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		if metricLabelsMatchPSCObs(m, labels) {
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

func histogramPSCObsCount(mf *dto.MetricFamily, labels map[string]string) (uint64, bool) {
	for _, m := range mf.GetMetric() {
		if metricLabelsMatchPSCObs(m, labels) {
			return m.GetHistogram().GetSampleCount(), true
		}
	}
	return 0, false
}

// U-OBS-PS-01: ObsInstrumenter with cluster_name label sets collection rate.
func TestPSCObsInstrumenter_RecordCollectionRate_ClusterNameLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	inst := NewPSCObsInstrumenter(reg)

	inst.RecordCollectionRate("isilon-cluster-1", 5.0)

	mf := gatherPSCObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, mf, "dell_csm_obs_collection_rate must be registered")

	v, ok := gaugePSCObs(mf, map[string]string{
		"module": "metrics-powerscale", "cluster_name": "isilon-cluster-1",
	})
	require.True(t, ok, "cluster_name label must be present")
	assert.Equal(t, 5.0, v, "collection rate must match")
}

func TestPSCObsInstrumenter_RecordsAllSelfMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	inst := NewPSCObsInstrumenter(reg)

	inst.RecordCollectionRate("cluster-1", 2.5)
	inst.RecordExportSuccess("cluster-1", "success")
	inst.SetArrayConnectivity("cluster-1", true)
	inst.RecordProcessingLatency("cluster-1", 0.25)

	labels := map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster-1"}

	rateMetric := gatherPSCObsMetric(t, reg, "dell_csm_obs_collection_rate")
	require.NotNil(t, rateMetric)
	rate, ok := gaugePSCObs(rateMetric, labels)
	require.True(t, ok)
	assert.Equal(t, 2.5, rate)

	exportMetric := gatherPSCObsMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, exportMetric)
	exportCount, ok := counterPSCObs(exportMetric, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster-1", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, exportCount)

	connectivityMetric := gatherPSCObsMetric(t, reg, "dell_csm_obs_array_connectivity")
	require.NotNil(t, connectivityMetric)
	connectivity, ok := gaugePSCObs(connectivityMetric, labels)
	require.True(t, ok)
	assert.Equal(t, 1.0, connectivity)

	latencyMetric := gatherPSCObsMetric(t, reg, "dell_csm_obs_processing_latency_seconds")
	require.NotNil(t, latencyMetric)
	latencyCount, ok := histogramPSCObsCount(latencyMetric, labels)
	require.True(t, ok)
	assert.Equal(t, uint64(1), latencyCount)
}

func TestPSCObsInstrumenter_NilReceiverDoesNotPanic(t *testing.T) {
	var inst *PSCObsInstrumenter

	assert.NotPanics(t, func() { inst.RecordCollectionRate("cluster-1", 1.0) })
	assert.NotPanics(t, func() { inst.RecordExportSuccess("cluster-1", "success") })
	assert.NotPanics(t, func() { inst.SetArrayConnectivity("cluster-1", true) })
	assert.NotPanics(t, func() { inst.RecordProcessingLatency("cluster-1", 0.1) })
}
