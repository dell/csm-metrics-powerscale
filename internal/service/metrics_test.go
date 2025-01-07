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

package service_test

import (
	"context"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	TestLabelClusterName  = "ClusterName"
	TestLabelPlotWithMean = "PlotWithMean"
	TestPlotWithMeanNo    = "No"
	TestCluster1          = "cluster-1"
)

func TestMetricsWrapper_RecordClusterQuota(t *testing.T) {
	mw := &service.MetricsWrapper{
		Meter: otel.Meter("powersscale-test"),
	}
	clusterMetas := []interface{}{
		&service.ClusterMeta{
			ClusterName: TestCluster1,
		},
	}
	volumeMetas := []interface{}{
		&service.VolumeMeta{
			ClusterName: TestCluster1,
		},
	}
	clusterQuotaRecordMetric := &service.ClusterQuotaRecord{}
	type args struct {
		ctx    context.Context
		meta   interface{}
		metric *service.ClusterQuotaRecord
	}
	tests := []struct {
		name    string
		mw      *service.MetricsWrapper
		args    args
		wantErr bool
	}{
		{
			name: "success",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				meta:   clusterMetas[0],
				metric: clusterQuotaRecordMetric,
			},
			wantErr: false,
		},
		{
			name: "fail",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				meta:   volumeMetas[0],
				metric: clusterQuotaRecordMetric,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.mw.RecordClusterQuota(tt.args.ctx, tt.args.meta, tt.args.metric); (err != nil) != tt.wantErr {
				t.Errorf("MetricsWrapper.RecordClusterQuota() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetricsWrapper_RecordVolumeQuota(t *testing.T) {
	mw := &service.MetricsWrapper{
		Meter: otel.Meter("powersscale-test"),
	}
	clusterMetas := []interface{}{
		&service.ClusterMeta{
			ClusterName: TestCluster1,
		},
	}
	volumeMetas := []interface{}{
		&service.VolumeMeta{
			ID:          "123",
			ClusterName: TestCluster1,
		},
	}
	VolumeQuotaMetricsRecordMetric := &service.VolumeQuotaMetricsRecord{}
	type args struct {
		ctx    context.Context
		meta   interface{}
		metric *service.VolumeQuotaMetricsRecord
	}
	tests := []struct {
		name    string
		mw      *service.MetricsWrapper
		args    args
		wantErr bool
	}{
		{
			name: "success",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				meta:   volumeMetas[0],
				metric: VolumeQuotaMetricsRecordMetric,
			},
			wantErr: false,
		},
		{
			name: "fail",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				meta:   clusterMetas[0],
				metric: VolumeQuotaMetricsRecordMetric,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.mw.RecordVolumeQuota(tt.args.ctx, tt.args.meta, tt.args.metric); (err != nil) != tt.wantErr {
				t.Errorf("MetricsWrapper.RecordVolumeQuota() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetricsWrapper_RecordClusterCapacityStatsMetrics(t *testing.T) {
	mw := &service.MetricsWrapper{
		Meter: otel.Meter("powersscale-test"),
	}
	type args struct {
		ctx    context.Context
		metric *service.ClusterCapacityStatsMetricsRecord
	}
	ClusterCapacityStatsMetricsRecordMetric := &service.ClusterCapacityStatsMetricsRecord{
		ClusterName:       TestCluster1,
		TotalCapacity:     173344948224,
		RemainingCapacity: 171467464704,
		UsedPercentage:    1.08,
	}
	tests := []struct {
		name    string
		mw      *service.MetricsWrapper
		args    args
		wantErr bool
	}{
		{
			name: "success",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				metric: ClusterCapacityStatsMetricsRecordMetric,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.mw.RecordClusterCapacityStatsMetrics(tt.args.ctx, tt.args.metric); (err != nil) != tt.wantErr {
				t.Errorf("MetricsWrapper.RecordClusterCapacityStatsMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetricsWrapper_RecordClusterPerformanceStatsMetrics(t *testing.T) {
	mw := &service.MetricsWrapper{
		Meter: otel.Meter("powersscale-test"),
	}
	type args struct {
		ctx    context.Context
		metric *service.ClusterPerformanceStatsMetricsRecord
	}
	ClusterPerformanceStatsMetricsRecordMetric := &service.ClusterPerformanceStatsMetricsRecord{
		ClusterName:             TestCluster1,
		CPUPercentage:           58,
		DiskReadOperationsRate:  0.3666666666666667,
		DiskWriteOperationsRate: 0.3666666666666667,
		DiskReadThroughputRate:  187.7333333333333,
		DiskWriteThroughputRate: 187.7333333333333,
	}
	tests := []struct {
		name    string
		mw      *service.MetricsWrapper
		args    args
		wantErr bool
	}{
		{
			name: "success",
			mw:   mw,
			args: args{
				ctx:    context.Background(),
				metric: ClusterPerformanceStatsMetricsRecordMetric,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.mw.RecordClusterPerformanceStatsMetrics(tt.args.ctx, tt.args.metric); (err != nil) != tt.wantErr {
				t.Errorf("MetricsWrapper.RecordClusterPerformanceStatsMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func assertEqual(a, b []attribute.KeyValue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMetricsWrapper_RecordClusterQuota_UpdateLabels(t *testing.T) {
	mw := &service.MetricsWrapper{
		Meter: otel.Meter("powerscale-test"),
	}

	tests := []struct {
		name                string
		initialMetaID       string
		initialLabels       []attribute.KeyValue
		inputClusterName    string
		inputLabels         []attribute.KeyValue
		expectLabelChange   bool
		expectedLabelsAfter []attribute.KeyValue
	}{
		{
			name:             "MetaID does not exist",
			initialMetaID:    "",
			initialLabels:    nil,
			inputClusterName: TestCluster1,
			inputLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
			expectLabelChange: true,
			expectedLabelsAfter: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
		},
		{
			name:             "MetaID exists but labels missing",
			initialMetaID:    TestCluster1,
			initialLabels:    nil,
			inputClusterName: TestCluster1,
			inputLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
			expectLabelChange: true,
			expectedLabelsAfter: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
		},
		{
			name:          "MetaID exists with mismatched labels",
			initialMetaID: TestCluster1,
			initialLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, "Yes"),
			},
			inputClusterName: TestCluster1,
			inputLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
			expectLabelChange: true,
			expectedLabelsAfter: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
		},
		{
			name:          "MetaID exists with matching labels",
			initialMetaID: TestCluster1,
			initialLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
			inputClusterName: TestCluster1,
			inputLabels: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
			expectLabelChange: false,
			expectedLabelsAfter: []attribute.KeyValue{
				attribute.String(TestLabelClusterName, TestCluster1),
				attribute.String(TestLabelPlotWithMean, TestPlotWithMeanNo),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.initialLabels != nil {
				mw.Labels.Store(tt.initialMetaID, tt.initialLabels)
			}

			clusterMeta := &service.ClusterMeta{
				ClusterName: tt.inputClusterName,
			}
			metric := &service.ClusterQuotaRecord{}

			err := mw.RecordClusterQuota(context.Background(), clusterMeta, metric)
			if err != nil {
				t.Fatalf("RecordClusterQuota() returned an unexpected error: %v", err)
			}

			updatedLabels, _ := mw.Labels.Load(tt.inputClusterName)
			if !assertEqual(updatedLabels.([]attribute.KeyValue), tt.expectedLabelsAfter) {
				t.Errorf("Expected labels %v, got %v", tt.expectedLabelsAfter, updatedLabels)
			}
		})
	}
}
