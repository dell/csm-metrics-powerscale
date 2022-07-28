// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package service_test

import (
	"context"
	"errors"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks/asyncfloat64mock"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"

	"github.com/golang/mock/gomock"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
)

func Test_Volume_Quota_Metrics_Record(t *testing.T) {
	type checkFn func(*testing.T, error)
	checkFns := func(checkFns ...checkFn) []checkFn { return checkFns }

	verifyError := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	metas := []interface{}{
		&service.VolumeMeta{
			ID: "123",
		},
	}

	tests := map[string]func(t *testing.T) ([]*service.MetricsWrapper, []checkFn){
		"success": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)

			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				subscribed, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "subscribed_quota")
				hardQuota, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_gigabytes")
				subscribedPct, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "subscribed_quota_percentage")
				hardQuotaPct, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_percentage")
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(4)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribed, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(hardQuota, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribedPct, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(hardQuotaPct, nil)

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_volume_"),
			}

			return mws, checkFns(verifyNoError)
		},
		"error creating subscribed_quota": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				subscribed, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "quota_subscribed_gigabytes")
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribed, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_volume_"),
			}

			return mws, checkFns(verifyError)
		},
		"error creating hard_quota_remaining": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				subscribed, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "quota_subscribed_gigabytes")
				hardQuota, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_gigabytes")
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(2)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribed, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(hardQuota, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_volume_"),
			}

			return mws, checkFns(verifyError)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mws, checks := tc(t)
			for i := range mws {
				err := mws[i].RecordVolumeQuota(context.Background(), metas[i], &service.VolumeQuotaMetricsRecord{})
				for _, check := range checks {
					check(t, err)
				}
			}
		})
	}
}

func Test_Cluster_Quota_Metrics_Record(t *testing.T) {
	type checkFn func(*testing.T, error)
	checkFns := func(checkFns ...checkFn) []checkFn { return checkFns }

	verifyError := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	metas := []interface{}{
		&service.ClusterMeta{
			ClusterName: "cluster-1",
		},
	}

	tests := map[string]func(t *testing.T) ([]*service.MetricsWrapper, []checkFn){
		"success": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)

			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")

				clusterSubscribed, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_directory_total_hard_quota_gigabytes")
				clusterSubscribedPct, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_directory_total_hard_quota_percentage")
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(2)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(clusterSubscribed, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(clusterSubscribedPct, nil)

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_directory"),
			}

			return mws, checkFns(verifyNoError)
		},
		"error creating cluster_subscribed_quota": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				clusterSubscribed, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_directory_total_hard_quota_gigabytes")

				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(clusterSubscribed, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_directory_"),
			}

			return mws, checkFns(verifyError)
		},
		"error creating hard_quota_remaining": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				clusterSubscribed, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_directory_total_hard_quota_gigabytes")
				clusterSubscribedPct, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_directory_total_hard_quota_percentage")
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(2)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(clusterSubscribed, nil)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(clusterSubscribedPct, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_directory_"),
			}

			return mws, checkFns(verifyError)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mws, checks := tc(t)
			for i := range mws {
				err := mws[i].RecordClusterQuota(context.Background(), metas[i], &service.ClusterQuotaRecord{})
				for _, check := range checks {
					check(t, err)
				}
			}
		})
	}
}

func Test_Volume_Metrics_Label_Update(t *testing.T) {
	metaFirst := &service.VolumeMeta{
		ID:                        "k8s-7242537ae1",
		PersistentVolumeName:      "k8s-7242537ae1",
		ClusterName:               "testCluster",
		IsiPath:                   "/ifs/data/csi",
		StorageClass:              "isilon",
		PersistentVolumeClaimName: "pvc-name",
		Namespace:                 "pvc-namespace",
	}

	metaSecond := &service.VolumeMeta{
		ID:                        "k8s-7242537ae1",
		PersistentVolumeName:      "k8s-7242537ae1",
		ClusterName:               "testCluster",
		IsiPath:                   "/ifs/data/csi",
		StorageClass:              "isilon",
		PersistentVolumeClaimName: "pvc-name",
		Namespace:                 "pvc-namespace",
	}

	metaThird := &service.VolumeMeta{
		ID:                        "k8s-7242537263",
		PersistentVolumeName:      "k8s-7242537263",
		ClusterName:               "testCluster",
		IsiPath:                   "/ifs/data/csi",
		StorageClass:              "isilon",
		PersistentVolumeClaimName: "pvc-name",
		Namespace:                 "pvc-namespace",
	}

	expectedLabels := []attribute.KeyValue{
		attribute.String("VolumeID", metaSecond.ID),
		attribute.String("ClusterName", metaSecond.ClusterName),
		attribute.String("PersistentVolumeName", metaSecond.PersistentVolumeName),
		attribute.String("IsiPath", metaSecond.IsiPath),
		attribute.String("StorageClass", metaSecond.StorageClass),
		attribute.String("PlotWithMean", "No"),
		attribute.String("PersistentVolumeClaim", metaSecond.PersistentVolumeClaimName),
		attribute.String("PersistentVolumeNameSpace", metaSecond.Namespace),
	}

	ctrl := gomock.NewController(t)

	meter := mocks.NewMockAsyncMetricCreator(ctrl)
	provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
	otMeter := global.Meter("powerscale_volume_quota_test")
	subscribed, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_volume_quota_subscribed_gigabytes")
	hardQuota, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_volume_hard_quota_remaining_gigabytes")
	subscribedPct, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_volume_quota_subscribed_quota_percentage")
	hardQuotaPct, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_volume_hard_quota_remaining_percentage")
	if err != nil {
		t.Fatal(err)
	}

	meter.EXPECT().AsyncFloat64().Return(provider).Times(8)
	provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribed, nil).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(hardQuota, nil).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(subscribedPct, nil).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(hardQuotaPct, nil).Times(2)

	mw := &service.MetricsWrapper{
		Meter: meter,
	}

	t.Run("success: metric labels updated", func(t *testing.T) {
		err := mw.RecordVolumeQuota(context.Background(), metaFirst, &service.VolumeQuotaMetricsRecord{})
		if err != nil {
			t.Errorf("expected nil error (record #1), got %v", err)
		}
		err = mw.RecordVolumeQuota(context.Background(), metaSecond, &service.VolumeQuotaMetricsRecord{})
		if err != nil {
			t.Errorf("expected nil error (record #2), got %v", err)
		}
		err = mw.RecordVolumeQuota(context.Background(), metaThird, &service.VolumeQuotaMetricsRecord{})
		if err != nil {
			t.Errorf("expected nil error (record #3), got %v", err)
		}

		newLabels, ok := mw.Labels.Load(metaFirst.ID)
		if !ok {
			t.Errorf("expected labels to exist for %v, but did not find them", metaFirst.ID)
		}
		labels := newLabels.([]attribute.KeyValue)
		for _, l := range labels {
			for _, e := range expectedLabels {
				if l.Key == e.Key {
					if l.Value.AsString() != e.Value.AsString() {
						t.Errorf("expected label %v to be updated to %v, but the value was %v", e.Key, e.Value.AsString(), l.Value.AsString())
					}
				}
			}
		}
	})
}

func Test_Cluster_Capacity_Stats_Metrics_Record(t *testing.T) {
	type checkFn func(*testing.T, error)
	checkFns := func(checkFns ...checkFn) []checkFn { return checkFns }

	verifyError := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	tests := map[string]func(t *testing.T) ([]*service.MetricsWrapper, []checkFn){
		"success": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)

			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				total, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "total_capacity_terabytes")
				avail, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "remaining_capacity_terabytes")
				usedPercent, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "used_capacity_percentage")

				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(3)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(total, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(avail, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(usedPercent, nil)

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_cluster_"),
			}

			return mws, checkFns(verifyNoError)
		},
		"error creating cluster capacity stats metrics": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				total, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "total_capacity_terabytes")

				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(total, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_cluster_"),
			}

			return mws, checkFns(verifyError)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mws, checks := tc(t)
			clusterStats := &service.ClusterCapacityStatsMetricsRecord{
				ClusterName:       "cluster-1",
				TotalCapacity:     173344948224,
				RemainingCapacity: 171467464704,
				UsedPercentage:    1.08,
			}
			for i := range mws {

				err := mws[i].RecordClusterCapacityStatsMetrics(context.Background(), clusterStats)
				for _, check := range checks {
					check(t, err)
				}
			}
		})
	}
}

func Test_Cluster_Perf_Stats_Metrics_Record(t *testing.T) {
	type checkFn func(*testing.T, error)
	checkFns := func(checkFns ...checkFn) []checkFn { return checkFns }

	verifyError := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	tests := map[string]func(t *testing.T) ([]*service.MetricsWrapper, []checkFn){
		"success": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)

			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				cpuPercentage, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "cpu_use_rate")
				diskReadOperationsRate, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "disk_read_operation_rate")
				diskWriteOperationsRate, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "disk_write_operation_rate")
				diskReadThroughputRate, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "disk_throughput_read_rate_megabytes_per_second")
				diskWriteThroughputRate, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "disk_throughput_write_rate_megabytes_per_second")

				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(5)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(cpuPercentage, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(diskReadOperationsRate, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(diskWriteOperationsRate, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(diskReadThroughputRate, nil)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(diskWriteThroughputRate, nil)

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_cluster_"),
			}

			return mws, checkFns(verifyNoError)
		},
		"error creating cluster perf stats metrics": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncMetricCreator(ctrl)
				provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				total, err := otMeter.AsyncFloat64().UpDownCounter(prefix + "disk_read_operation_rate")

				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncFloat64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any()).Return(total, errors.New("error"))

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_cluster_"),
			}

			return mws, checkFns(verifyError)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mws, checks := tc(t)
			clusterStats := &service.ClusterPerformanceStatsMetricsRecord{
				ClusterName:             "cluster-1",
				CPUPercentage:           58,
				DiskReadOperationsRate:  0.3666666666666667,
				DiskWriteOperationsRate: 0.3666666666666667,
				DiskReadThroughputRate:  187.7333333333333,
				DiskWriteThroughputRate: 187.7333333333333,
			}
			for i := range mws {

				err := mws[i].RecordClusterPerformanceStatsMetrics(context.Background(), clusterStats)
				for _, check := range checks {
					check(t, err)
				}
			}
		})
	}
}

func Test_Cluster_Stats_Metrics_Label_Update(t *testing.T) {
	metricFirst := &service.ClusterCapacityStatsMetricsRecord{
		ClusterName:   "cluster-1",
		TotalCapacity: 252239708160,
	}

	metricSecond := &service.ClusterCapacityStatsMetricsRecord{
		ClusterName:   "cluster-2",
		TotalCapacity: 252239708160,
	}

	metricThird := &service.ClusterCapacityStatsMetricsRecord{
		ClusterName:   "cluster-1",
		TotalCapacity: 352239708160,
	}

	expectedLabels := []attribute.KeyValue{
		attribute.String("ClusterName", metricSecond.ClusterName),
		attribute.String("PlotWithMean", "No"),
	}

	ctrl := gomock.NewController(t)

	meter := mocks.NewMockAsyncMetricCreator(ctrl)
	provider := asyncfloat64mock.NewMockInstrumentProvider(ctrl)
	otMeter := global.Meter("powerscale_cluster_test")
	total, err := otMeter.AsyncFloat64().UpDownCounter("powerscale_cluster_total_capacity_terabytes")
	avail, err := otMeter.AsyncFloat64().UpDownCounter("remaining_capacity_terabytes")
	usedPercent, err := otMeter.AsyncFloat64().UpDownCounter("used_capacity_percentage")
	if err != nil {
		t.Fatal(err)
	}

	meter.EXPECT().AsyncFloat64().Return(provider).Times(6)
	provider.EXPECT().UpDownCounter(gomock.Any()).Return(total, nil).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any()).Return(avail, nil).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any()).Return(usedPercent, nil).Times(2)

	mw := &service.MetricsWrapper{
		Meter: meter,
	}

	t.Run("success: cluster stats metric labels updated", func(t *testing.T) {
		err := mw.RecordClusterCapacityStatsMetrics(context.Background(), metricFirst)
		if err != nil {
			t.Errorf("expected nil error (record #1), got %v", err)
		}
		err = mw.RecordClusterCapacityStatsMetrics(context.Background(), metricSecond)
		if err != nil {
			t.Errorf("expected nil error (record #2), got %v", err)
		}
		err = mw.RecordClusterCapacityStatsMetrics(context.Background(), metricThird)
		if err != nil {
			t.Errorf("expected nil error (record #3), got %v", err)
		}

		newLabels, ok := mw.Labels.Load(metricSecond.ClusterName)
		if !ok {
			t.Errorf("expected labels to exist for %v, but did not find them", metricFirst.ClusterName)
		}
		labels := newLabels.([]attribute.KeyValue)
		for _, l := range labels {
			for _, e := range expectedLabels {
				if l.Key == e.Key {
					if l.Value.AsString() != e.Value.AsString() {
						t.Errorf("expected label %v to be updated to %v, but the value was %v", e.Key, e.Value.AsString(), l.Value.AsString())
					}
				}
			}
		}
	})
}
