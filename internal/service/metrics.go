// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package service

import (
	"context"
	"errors"
	"github.com/dell/csm-metrics-powerscale/internal/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"sync"
)

// MetricsRecorder supports recording volume and cluster metric
//go:generate mockgen -destination=mocks/metrics_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service MetricsRecorder,AsyncMetricCreator
type MetricsRecorder interface {
	RecordVolumeSpace(ctx context.Context, meta interface{}, subscribedQuota, hardQuota int64) error
	RecordClusterCapacityStatsMetrics(ctx context.Context, metric *ClusterCapacityStatsMetricsRecord) error
	RecordClusterPerformanceStatsMetrics(ctx context.Context, metric *ClusterPerformanceStatsMetricsRecord) error
}

//go:generate mockgen -destination=mocks/asyncint64mock/instrument_asyncint64_provider_mocks.go -package=asyncint64mock go.opentelemetry.io/otel/metric/instrument/asyncint64 InstrumentProvider
//go:generate mockgen -destination=mocks/asyncfloat64mock/instrument_asyncfloat64_provider_mocks.go -package=asyncfloat64mock go.opentelemetry.io/otel/metric/instrument/asyncfloat64 InstrumentProvider
// AsyncMetricCreator to create AsyncInt64/AsyncFloat64 InstrumentProvider
type AsyncMetricCreator interface {
	AsyncInt64() asyncint64.InstrumentProvider
	AsyncFloat64() asyncfloat64.InstrumentProvider
}

// MetricsWrapper contains data used for pushing metrics data
type MetricsWrapper struct {
	Meter                          AsyncMetricCreator
	Labels                         sync.Map
	VolumeMetrics                  sync.Map
	ClusterCapacityStatsMetrics    sync.Map
	ClusterPerformanceStatsMetrics sync.Map
}

// VolumeSpaceMetrics contains the volume metrics data
type VolumeSpaceMetrics struct {
	QuotaSubscribed    asyncfloat64.UpDownCounter
	HardQuotaRemaining asyncfloat64.UpDownCounter
}

// ClusterCapacityStatsMetrics contains the capacity stats metrics related to a cluster
type ClusterCapacityStatsMetrics struct {
	TotalCapacity     asyncfloat64.UpDownCounter
	RemainingCapacity asyncfloat64.UpDownCounter
	UsedPercentage    asyncfloat64.UpDownCounter
}

// ClusterPerformanceStatsMetrics contains the performance stats metrics related to a cluster
type ClusterPerformanceStatsMetrics struct {
	CPUPercentage           asyncfloat64.UpDownCounter
	DiskReadOperationsRate  asyncfloat64.UpDownCounter
	DiskWriteOperationsRate asyncfloat64.UpDownCounter
	DiskReadThroughputRate  asyncfloat64.UpDownCounter
	DiskWriteThroughputRate asyncfloat64.UpDownCounter
}

type loadMetricsFunc func(metaID string) (value any, ok bool)
type initMetricsFunc func(prefix, metaID string, labels []attribute.KeyValue) (any, error)

// haveLabelsChanged checks if labels have been changed
func haveLabelsChanged(currentLabels []attribute.KeyValue, labels []attribute.KeyValue) (bool, []attribute.KeyValue) {
	updatedLabels := currentLabels
	haveLabelsChanged := false
	for i, current := range currentLabels {
		for _, new := range labels {
			if current.Key == new.Key {
				if current.Value != new.Value {
					updatedLabels[i].Value = new.Value
					haveLabelsChanged = true
				}
			}
		}
	}
	return haveLabelsChanged, updatedLabels
}

func updateLabels(prefix, metaID string, labels []attribute.KeyValue, mw *MetricsWrapper, loadMetrics loadMetricsFunc, initMetrics initMetricsFunc) (any, error) {
	metricsMapValue, ok := loadMetrics(metaID)
	if !ok {
		newMetrics, err := initMetrics(prefix, metaID, labels)
		if err != nil {
			return nil, err
		}
		metricsMapValue = newMetrics
	} else {
		// If VolumeSpaceMetrics for this MetricsWrapper exist, then check if any labels have changed and update them
		currentLabels, ok := mw.Labels.Load(metaID)
		if !ok {
			newMetrics, err := initMetrics(prefix, metaID, labels)
			if err != nil {
				return nil, err
			}
			metricsMapValue = newMetrics
		} else {
			haveLabelsChanged, updatedLabels := haveLabelsChanged(currentLabels.([]attribute.KeyValue), labels)
			if haveLabelsChanged {
				newMetrics, err := initMetrics(prefix, metaID, updatedLabels)
				if err != nil {
					return nil, err
				}
				metricsMapValue = newMetrics
			}
		}
	}
	return metricsMapValue, nil
}

func (mw *MetricsWrapper) initVolumeMetrics(prefix, metaID string, labels []attribute.KeyValue) (*VolumeSpaceMetrics, error) {
	quotaSubscribed, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "quota_subscribed_gigabytes")
	if err != nil {
		return nil, err
	}
	hardQuotaRemaining, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_gigabytes")
	if err != nil {
		return nil, err
	}

	metrics := &VolumeSpaceMetrics{
		QuotaSubscribed:    quotaSubscribed,
		HardQuotaRemaining: hardQuotaRemaining,
	}

	mw.VolumeMetrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}

// RecordVolumeSpace will publish volume metrics data
func (mw *MetricsWrapper) RecordVolumeSpace(ctx context.Context, meta interface{}, subscribedQuota, hardQuotaRemaining int64) error {
	var prefix string
	var metaID string
	var labels []attribute.KeyValue
	switch v := meta.(type) {
	case *VolumeMeta:
		prefix, metaID = "powerscale_volume_", v.ID
		labels = []attribute.KeyValue{
			attribute.String("VolumeID", v.ID),
			attribute.String("ClusterName", v.ClusterName),
			attribute.String("PersistentVolumeName", v.PersistentVolumeName),
			attribute.String("IsiPath", v.IsiPath),
			attribute.String("StorageClass", v.StorageClass),
			attribute.String("PlotWithMean", "No"),
			attribute.String("PersistentVolumeClaim", v.PersistentVolumeClaimName),
			attribute.String("PersistentVolumeNameSpace", v.NameSpace),
		}
	default:
		return errors.New("unknown MetaData type")
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.VolumeMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initVolumeMetrics(prefix, metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)

	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*VolumeSpaceMetrics)
	metrics.QuotaSubscribed.Observe(ctx, utils.UnitsConvert(subscribedQuota, utils.BYTES, utils.GB), labels...)
	metrics.HardQuotaRemaining.Observe(ctx, utils.UnitsConvert(hardQuotaRemaining, utils.BYTES, utils.GB), labels...)

	return nil
}

// RecordClusterCapacityStatsMetrics will publish cluster capacity stats metrics
func (mw *MetricsWrapper) RecordClusterCapacityStatsMetrics(ctx context.Context, metric *ClusterCapacityStatsMetricsRecord) error {
	var prefix string
	var metaID string
	var labels []attribute.KeyValue

	prefix, metaID = "powerscale_cluster_", metric.ClusterName
	labels = []attribute.KeyValue{
		attribute.String("ClusterName", metric.ClusterName),
		attribute.String("PlotWithMean", "No"),
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.ClusterCapacityStatsMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initClusterCapacityStatsMetrics(prefix, metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)

	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*ClusterCapacityStatsMetrics)
	metrics.TotalCapacity.Observe(ctx, utils.UnitsConvert(metric.TotalCapacity, utils.BYTES, utils.TB), labels...)
	metrics.RemainingCapacity.Observe(ctx, utils.UnitsConvert(metric.RemainingCapacity, utils.BYTES, utils.TB), labels...)

	if metric.TotalCapacity != 0 {
		metrics.UsedPercentage.Observe(ctx, 100*(metric.TotalCapacity-metric.RemainingCapacity)/metric.TotalCapacity, labels...)
	}

	return nil
}

// RecordClusterPerformanceStatsMetrics will publish cluster performance stats metrics
func (mw *MetricsWrapper) RecordClusterPerformanceStatsMetrics(ctx context.Context, metric *ClusterPerformanceStatsMetricsRecord) error {
	var prefix string
	var metaID string
	var labels []attribute.KeyValue

	prefix, metaID = "powerscale_cluster_", metric.ClusterName
	labels = []attribute.KeyValue{
		attribute.String("ClusterName", metric.ClusterName),
		attribute.String("PlotWithMean", "No"),
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.ClusterPerformanceStatsMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initClusterPerformanceStatsMetrics(prefix, metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)

	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*ClusterPerformanceStatsMetrics)
	metrics.CPUPercentage.Observe(ctx, metric.CPUPercentage/10, labels...)
	metrics.DiskReadOperationsRate.Observe(ctx, metric.DiskReadOperationsRate, labels...)
	metrics.DiskWriteOperationsRate.Observe(ctx, metric.DiskWriteOperationsRate, labels...)
	metrics.DiskReadThroughputRate.Observe(ctx, utils.UnitsConvert(metric.DiskReadThroughputRate, utils.BYTES, utils.MB), labels...)
	metrics.DiskWriteThroughputRate.Observe(ctx, utils.UnitsConvert(metric.DiskWriteThroughputRate, utils.BYTES, utils.MB), labels...)

	return nil
}

func (mw *MetricsWrapper) initClusterCapacityStatsMetrics(prefix string, id string, labels []attribute.KeyValue) (*ClusterCapacityStatsMetrics, error) {
	totalCapacity, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "total_capacity_terabytes")
	if err != nil {
		return nil, err
	}
	remainingCapacity, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "remaining_capacity_terabytes")
	if err != nil {
		return nil, err
	}
	usedPercentage, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "used_capacity_percentage")
	if err != nil {
		return nil, err
	}

	metrics := &ClusterCapacityStatsMetrics{
		TotalCapacity:     totalCapacity,
		RemainingCapacity: remainingCapacity,
		UsedPercentage:    usedPercentage,
	}

	mw.ClusterCapacityStatsMetrics.Store(id, metrics)
	mw.Labels.Store(id, labels)

	return metrics, nil
}

func (mw *MetricsWrapper) initClusterPerformanceStatsMetrics(prefix string, id string, labels []attribute.KeyValue) (*ClusterPerformanceStatsMetrics, error) {
	cpuPercentage, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "cpu_use_rate")
	if err != nil {
		return nil, err
	}
	diskReadOperationsRate, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "disk_read_operation_rate")
	if err != nil {
		return nil, err
	}
	diskWriteOperationsRate, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "disk_write_operation_rate")
	if err != nil {
		return nil, err
	}
	diskReadThroughput, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "disk_throughput_read_rate_megabytes_per_second")
	if err != nil {
		return nil, err
	}
	diskWriteThroughput, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "disk_throughput_write_rate_megabytes_per_second")
	if err != nil {
		return nil, err
	}

	metrics := &ClusterPerformanceStatsMetrics{
		CPUPercentage:           cpuPercentage,
		DiskReadOperationsRate:  diskReadOperationsRate,
		DiskWriteOperationsRate: diskWriteOperationsRate,
		DiskReadThroughputRate:  diskReadThroughput,
		DiskWriteThroughputRate: diskWriteThroughput,
	}

	mw.ClusterPerformanceStatsMetrics.Store(id, metrics)
	mw.Labels.Store(id, labels)

	return metrics, nil
}
