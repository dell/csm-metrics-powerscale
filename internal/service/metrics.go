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

package service

import (
	"context"
	"errors"
	"sync"

	"github.com/dell/csm-metrics-powerscale/internal/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
)

// MetricsRecorder supports recording volume and cluster metric
//
//go:generate mockgen -destination=mocks/metrics_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service MetricsRecorder,AsyncMetricCreator
type MetricsRecorder interface {
	RecordVolumeQuota(ctx context.Context, meta interface{}, metric *VolumeQuotaMetricsRecord) error
	RecordClusterQuota(ctx context.Context, meta interface{}, metric *ClusterQuotaRecord) error
	RecordClusterCapacityStatsMetrics(ctx context.Context, metric *ClusterCapacityStatsMetricsRecord) error
	RecordClusterPerformanceStatsMetrics(ctx context.Context, metric *ClusterPerformanceStatsMetricsRecord) error
}

// AsyncMetricCreator to create AsyncInt64/AsyncFloat64 InstrumentProvider
//
//go:generate mockgen -destination=mocks/asyncint64mock/instrument_asyncint64_provider_mocks.go -package=asyncint64mock go.opentelemetry.io/otel/metric/instrument/asyncint64 InstrumentProvider
//go:generate mockgen -destination=mocks/asyncfloat64mock/instrument_asyncfloat64_provider_mocks.go -package=asyncfloat64mock go.opentelemetry.io/otel/metric/instrument/asyncfloat64 InstrumentProvider
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
	VolumeQuotaMetrics             sync.Map
	ClusterQuotaMetrics            sync.Map
}

// VolumeQuotaMetrics contains volume quota metrics data
type VolumeQuotaMetrics struct {
	QuotaSubscribed       asyncfloat64.UpDownCounter
	HardQuotaRemaining    asyncfloat64.UpDownCounter
	QuotaSubscribedPct    asyncfloat64.UpDownCounter
	HardQuotaRemainingPct asyncfloat64.UpDownCounter
}

// ClusterQuotaMetrics contains quota capacity in all directories
type ClusterQuotaMetrics struct {
	TotalHardQuotaGigabytes asyncfloat64.UpDownCounter
	TotalHardQuotaPct       asyncfloat64.UpDownCounter
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

type (
	loadMetricsFunc func(metaID string) (value any, ok bool)
	initMetricsFunc func(prefix, metaID string, labels []attribute.KeyValue) (any, error)
)

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

func (mw *MetricsWrapper) initClusterQuotaMetrics(prefix, metaID string, labels []attribute.KeyValue) (*ClusterQuotaMetrics, error) {
	totalHardQuota, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "total_hard_quota_gigabytes")
	if err != nil {
		return nil, err
	}
	TotalHardQuotaPct, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "total_hard_quota_percentage")
	if err != nil {
		return nil, err
	}

	metrics := &ClusterQuotaMetrics{
		TotalHardQuotaGigabytes: totalHardQuota,
		TotalHardQuotaPct:       TotalHardQuotaPct,
	}

	mw.ClusterQuotaMetrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}

// RecordClusterQuota will publish cluster Quota metrics data
func (mw *MetricsWrapper) RecordClusterQuota(ctx context.Context, meta interface{}, metric *ClusterQuotaRecord) error {
	var prefix string
	var metaID string
	var labels []attribute.KeyValue
	switch v := meta.(type) {
	case *ClusterMeta:
		prefix, metaID = "powerscale_directory_", v.ClusterName
		labels = []attribute.KeyValue{
			attribute.String("ClusterName", v.ClusterName),
			attribute.String("PlotWithMean", "No"),
		}
	default:
		return errors.New("unknown MetaData type")
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.ClusterQuotaMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initClusterQuotaMetrics(prefix, metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)
	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*ClusterQuotaMetrics)
	metrics.TotalHardQuotaGigabytes.Observe(ctx, utils.UnitsConvert(metric.totalHardQuota, utils.BYTES, utils.GB), labels...)
	metrics.TotalHardQuotaPct.Observe(ctx, metric.totalHardQuotaPct, labels...)

	return nil
}

func (mw *MetricsWrapper) initVolumeQuotaMetrics(prefix, metaID string, labels []attribute.KeyValue) (*VolumeQuotaMetrics, error) {
	quotaSubscribed, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "quota_subscribed_gigabytes")
	if err != nil {
		return nil, err
	}
	hardQuotaRemaining, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_gigabytes")
	if err != nil {
		return nil, err
	}
	quotaSubscribedPct, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "quota_subscribed_percentage")
	if err != nil {
		return nil, err
	}
	hardQuotaRemainingPct, err := mw.Meter.AsyncFloat64().UpDownCounter(prefix + "hard_quota_remaining_percentage")
	if err != nil {
		return nil, err
	}

	metrics := &VolumeQuotaMetrics{
		QuotaSubscribed:       quotaSubscribed,
		HardQuotaRemaining:    hardQuotaRemaining,
		QuotaSubscribedPct:    quotaSubscribedPct,
		HardQuotaRemainingPct: hardQuotaRemainingPct,
	}

	mw.VolumeQuotaMetrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}

// RecordVolumeQuota will publish volume Quota metrics data
func (mw *MetricsWrapper) RecordVolumeQuota(ctx context.Context, meta interface{}, metric *VolumeQuotaMetricsRecord) error {
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
			attribute.String("Namespace", v.Namespace),
		}
	default:
		return errors.New("unknown MetaData type")
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.VolumeQuotaMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initVolumeQuotaMetrics(prefix, metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)
	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*VolumeQuotaMetrics)
	metrics.QuotaSubscribed.Observe(ctx, utils.UnitsConvert(metric.quotaSubscribed, utils.BYTES, utils.GB), labels...)
	metrics.HardQuotaRemaining.Observe(ctx, utils.UnitsConvert(metric.hardQuotaRemaining, utils.BYTES, utils.GB), labels...)
	metrics.QuotaSubscribedPct.Observe(ctx, metric.quotaSubscribedPct, labels...)
	metrics.HardQuotaRemainingPct.Observe(ctx, metric.hardQuotaRemainingPct, labels...)

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
