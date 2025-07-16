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
	otelMetric "go.opentelemetry.io/otel/metric"
)

// MetricsRecorder supports recording volume and cluster metric
//
//go:generate mockgen -destination=mocks/metrics_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service MetricsRecorder,MeterCreator
type MetricsRecorder interface {
	RecordVolumeQuota(ctx context.Context, meta interface{}, metric *VolumeQuotaMetricsRecord) error
	RecordClusterQuota(ctx context.Context, meta interface{}, metric *ClusterQuotaRecord) error
	RecordClusterCapacityStatsMetrics(ctx context.Context, metric *ClusterCapacityStatsMetricsRecord) error
	RecordClusterPerformanceStatsMetrics(ctx context.Context, metric *ClusterPerformanceStatsMetricsRecord) error
	RecordTopologyMetrics(ctx context.Context, meta interface{}, metric *TopologyMetricsRecord) error
}

// MeterCreator interface is used to create and provide Meter instances, which are used to report measurements
//
//go:generate mockgen -destination=mocks/meter_mocks.go -package=mocks go.opentelemetry.io/otel/metric Meter
type MeterCreator interface {
	MeterProvider() (otelMetric.Meter, error)
}

// MetricsWrapper contains data used for pushing metrics data
type MetricsWrapper struct {
	Meter                          otelMetric.Meter
	Labels                         sync.Map
	VolumeMetrics                  sync.Map
	ClusterCapacityStatsMetrics    sync.Map
	ClusterPerformanceStatsMetrics sync.Map
	VolumeQuotaMetrics             sync.Map
	ClusterQuotaMetrics            sync.Map
	TopologyMetrics                sync.Map
}

// VolumeQuotaMetrics contains volume quota metrics data
type VolumeQuotaMetrics struct {
	QuotaSubscribed       otelMetric.Float64ObservableUpDownCounter
	HardQuotaRemaining    otelMetric.Float64ObservableUpDownCounter
	QuotaSubscribedPct    otelMetric.Float64ObservableUpDownCounter
	HardQuotaRemainingPct otelMetric.Float64ObservableUpDownCounter
}

// TopologyMetrics contains topology metrics when PV is available on cluster
type TopologyMetrics struct {
	PvAvailable otelMetric.Float64ObservableUpDownCounter
}

// ClusterQuotaMetrics contains quota capacity in all directories
type ClusterQuotaMetrics struct {
	TotalHardQuotaGigabytes otelMetric.Float64ObservableUpDownCounter
	TotalHardQuotaPct       otelMetric.Float64ObservableUpDownCounter
}

// ClusterCapacityStatsMetrics contains the capacity stats metrics related to a cluster
type ClusterCapacityStatsMetrics struct {
	TotalCapacity     otelMetric.Float64ObservableUpDownCounter
	RemainingCapacity otelMetric.Float64ObservableUpDownCounter
	UsedPercentage    otelMetric.Float64ObservableUpDownCounter
}

// ClusterPerformanceStatsMetrics contains the performance stats metrics related to a cluster
type ClusterPerformanceStatsMetrics struct {
	CPUPercentage           otelMetric.Float64ObservableUpDownCounter
	DiskReadOperationsRate  otelMetric.Float64ObservableUpDownCounter
	DiskWriteOperationsRate otelMetric.Float64ObservableUpDownCounter
	DiskReadThroughputRate  otelMetric.Float64ObservableUpDownCounter
	DiskWriteThroughputRate otelMetric.Float64ObservableUpDownCounter
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
		// If VolumeSpaceMetrics for this MetricsWrapper exist, then update the labels
		currentLabels, ok := mw.Labels.Load(metaID)
		if ok {
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

	done := make(chan struct{})
	defer close(done)

	return metricsMapValue, nil
}

func (mw *MetricsWrapper) initClusterQuotaMetrics(prefix, metaID string, labels []attribute.KeyValue) (*ClusterQuotaMetrics, error) {
	totalHardQuota, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "total_hard_quota_gigabytes")

	TotalHardQuotaPct, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "total_hard_quota_percentage")

	metrics := &ClusterQuotaMetrics{
		TotalHardQuotaGigabytes: totalHardQuota,
		TotalHardQuotaPct:       TotalHardQuotaPct,
	}

	mw.ClusterQuotaMetrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}

// RecordClusterQuota will publish cluster Quota metrics data
func (mw *MetricsWrapper) RecordClusterQuota(_ context.Context, meta interface{}, metric *ClusterQuotaRecord) error {
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

	hardQuotaGigabytes := utils.UnitsConvert(metric.totalHardQuota, utils.BYTES, utils.GB)

	metrics := metricsMapValue.(*ClusterQuotaMetrics)
	done := make(chan struct{})
	reg, err := mw.Meter.RegisterCallback(func(_ context.Context, obs otelMetric.Observer) error {
		obs.ObserveFloat64(metrics.TotalHardQuotaGigabytes, hardQuotaGigabytes, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.TotalHardQuotaPct, metric.totalHardQuotaPct, otelMetric.WithAttributes(labels...))
		go func() {
			done <- struct{}{}
		}()
		return nil
	},
		metrics.TotalHardQuotaGigabytes,
		metrics.TotalHardQuotaPct)
	if err != nil {
		return err
	}

	<-done
	_ = reg.Unregister()

	return nil
}

func (mw *MetricsWrapper) initVolumeQuotaMetrics(prefix, metaID string, labels []attribute.KeyValue) (*VolumeQuotaMetrics, error) {
	quotaSubscribed, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "quota_subscribed_gigabytes")

	hardQuotaRemaining, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "hard_quota_remaining_gigabytes")

	quotaSubscribedPct, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "quota_subscribed_percentage")

	hardQuotaRemainingPct, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "hard_quota_remaining_percentage")

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
func (mw *MetricsWrapper) RecordVolumeQuota(_ context.Context, meta interface{}, metric *VolumeQuotaMetricsRecord) error {
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

	quotaSub := utils.UnitsConvert(metric.quotaSubscribed, utils.BYTES, utils.GB)
	hardQuotaRemSub := utils.UnitsConvert(metric.hardQuotaRemaining, utils.BYTES, utils.GB)

	metrics := metricsMapValue.(*VolumeQuotaMetrics)

	done := make(chan struct{})
	reg, err := mw.Meter.RegisterCallback(func(_ context.Context, obs otelMetric.Observer) error {
		obs.ObserveFloat64(metrics.QuotaSubscribed, quotaSub, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.HardQuotaRemaining, hardQuotaRemSub, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.QuotaSubscribedPct, metric.quotaSubscribedPct, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.HardQuotaRemainingPct, metric.hardQuotaRemainingPct, otelMetric.WithAttributes(labels...))
		go func() {
			done <- struct{}{}
		}()
		return nil
	},
		metrics.QuotaSubscribed,
		metrics.HardQuotaRemaining,
		metrics.QuotaSubscribedPct,
		metrics.HardQuotaRemainingPct)
	if err != nil {
		return err
	}

	<-done
	_ = reg.Unregister()

	return nil
}

// RecordClusterCapacityStatsMetrics will publish cluster capacity stats metrics
func (mw *MetricsWrapper) RecordClusterCapacityStatsMetrics(_ context.Context, metric *ClusterCapacityStatsMetricsRecord) error {
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

	totalCapacityInitial := utils.UnitsConvert(metric.TotalCapacity, utils.BYTES, utils.TB)
	remainingCapacity := utils.UnitsConvert(metric.RemainingCapacity, utils.BYTES, utils.TB)
	totalCapacityFinal := 100 * (metric.TotalCapacity - metric.RemainingCapacity) / metric.TotalCapacity

	metrics := metricsMapValue.(*ClusterCapacityStatsMetrics)

	done := make(chan struct{})
	reg, err := mw.Meter.RegisterCallback(func(_ context.Context, obs otelMetric.Observer) error {
		obs.ObserveFloat64(metrics.TotalCapacity, totalCapacityInitial, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.RemainingCapacity, remainingCapacity, otelMetric.WithAttributes(labels...))

		if metric.TotalCapacity != 0 {
			obs.ObserveFloat64(metrics.UsedPercentage, totalCapacityFinal, otelMetric.WithAttributes(labels...))
		}

		go func() {
			done <- struct{}{}
		}()
		return nil
	},
		metrics.TotalCapacity,
		metrics.RemainingCapacity,
		metrics.UsedPercentage)
	if err != nil {
		return err
	}

	<-done
	_ = reg.Unregister()

	return nil
}

// RecordClusterPerformanceStatsMetrics will publish cluster performance stats metrics
func (mw *MetricsWrapper) RecordClusterPerformanceStatsMetrics(_ context.Context, metric *ClusterPerformanceStatsMetricsRecord) error {
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

	cpuPercentage := metric.CPUPercentage / 10
	diskReadThroughputRate := utils.UnitsConvert(metric.DiskReadThroughputRate, utils.BYTES, utils.MB)
	diskWriteThroughputRate := utils.UnitsConvert(metric.DiskWriteThroughputRate, utils.BYTES, utils.MB)

	metrics := metricsMapValue.(*ClusterPerformanceStatsMetrics)

	done := make(chan struct{})
	reg, err := mw.Meter.RegisterCallback(func(_ context.Context, obs otelMetric.Observer) error {
		obs.ObserveFloat64(metrics.CPUPercentage, cpuPercentage, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.DiskReadOperationsRate, metric.DiskReadOperationsRate, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.DiskWriteOperationsRate, metric.DiskWriteOperationsRate, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.DiskReadThroughputRate, diskReadThroughputRate, otelMetric.WithAttributes(labels...))
		obs.ObserveFloat64(metrics.DiskWriteThroughputRate, diskWriteThroughputRate, otelMetric.WithAttributes(labels...))

		go func() {
			done <- struct{}{}
		}()

		return nil
	},
		metrics.CPUPercentage,
		metrics.DiskReadOperationsRate,
		metrics.DiskWriteOperationsRate,
		metrics.DiskReadThroughputRate,
		metrics.DiskWriteThroughputRate)
	if err != nil {
		return err
	}

	<-done
	_ = reg.Unregister()

	return nil
}

func (mw *MetricsWrapper) initClusterCapacityStatsMetrics(prefix string, id string, labels []attribute.KeyValue) (*ClusterCapacityStatsMetrics, error) {
	totalCapacity, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "total_capacity_terabytes")

	remainingCapacity, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "remaining_capacity_terabytes")

	usedPercentage, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "used_capacity_percentage")

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
	cpuPercentage, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "cpu_use_rate")

	diskReadOperationsRate, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "disk_read_operation_rate")

	diskWriteOperationsRate, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "disk_write_operation_rate")

	diskReadThroughput, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "disk_throughput_read_rate_megabytes_per_second")

	diskWriteThroughput, _ := mw.Meter.Float64ObservableUpDownCounter(prefix + "disk_throughput_write_rate_megabytes_per_second")

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

// RecordTopologyMetrics will publish topology data to Otel
func (mw *MetricsWrapper) RecordTopologyMetrics(_ context.Context, meta interface{}, metric *TopologyMetricsRecord) error {
	var prefix string
	var metaID string
	var labels []attribute.KeyValue

	switch v := meta.(type) {
	case *TopologyMeta:
		metaID = v.PersistentVolume
		labels = []attribute.KeyValue{
			attribute.String("PersistentVolumeClaim", v.PersistentVolumeClaim),
			attribute.String("Driver", v.Driver),
			attribute.String("PersistentVolume", v.PersistentVolume),
			attribute.String("PersistentVolumeStatus", v.PersistentVolumeStatus),
			attribute.String("StorageClass", v.StorageClass),
			attribute.String("PlotWithMean", "No"),
			attribute.String("StorageClass", v.StorageClass),
			attribute.String("Namespace", v.Namespace),
			attribute.String("ProvisionedSize", v.ProvisionedSize),
			attribute.String("StorageSystemVolumeName", v.StorageSystemVolumeName),
			attribute.String("StorageSystem", v.StorageSystem),
			attribute.String("Protocol", v.Protocol),
			attribute.String("CreatedTime", v.CreatedTime),
		}
	default:
		return errors.New("unknown MetaData type")
	}

	loadMetricsFunc := func(metaID string) (any, bool) {
		return mw.TopologyMetrics.Load(metaID)
	}

	initMetricsFunc := func(prefix string, metaID string, labels []attribute.KeyValue) (any, error) {
		return mw.initTopologyMetrics("", metaID, labels)
	}

	metricsMapValue, err := updateLabels(prefix, metaID, labels, mw, loadMetricsFunc, initMetricsFunc)
	if err != nil {
		return err
	}

	metrics := metricsMapValue.(*TopologyMetrics)

	done := make(chan struct{})
	reg, err := mw.Meter.RegisterCallback(func(_ context.Context, obs otelMetric.Observer) error {

		obs.ObserveFloat64(metrics.PvAvailable, float64(metric.pvAvailable), otelMetric.WithAttributes(labels...))
		go func() {
			done <- struct{}{}
		}()
		return nil
	},
		metrics.PvAvailable)
	if err != nil {
		return err
	}

	<-done

	_ = reg.Unregister()

	return nil
}

func (mw *MetricsWrapper) initTopologyMetrics(prefix string, metaID string, labels []attribute.KeyValue) (*TopologyMetrics, error) {
	pvAvailable, _ := mw.Meter.Float64ObservableUpDownCounter("karavi_topology_metrics")

	metrics := &TopologyMetrics{
		PvAvailable: pvAvailable,
	}

	mw.TopologyMetrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}
