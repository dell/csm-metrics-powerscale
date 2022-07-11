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
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/unit"
)

// MetricsRecorder supports recording volume space metric
//go:generate mockgen -destination=mocks/metrics_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service MetricsRecorder,AsyncInt64Creater
type MetricsRecorder interface {
	Record(ctx context.Context, meta interface{}, usedCapacity int64) error
}

//go:generate mockgen -destination=mocks/instrument_provider_mocks.go -package=mocks go.opentelemetry.io/otel/metric/instrument/asyncint64 InstrumentProvider
// AsyncInt64Creater to create AsyncInt64 InstrumentProvider
type AsyncInt64Creater interface {
	AsyncInt64() asyncint64.InstrumentProvider
}

// MetricsWrapper contains data used for pushing metrics data
type MetricsWrapper struct {
	Meter   AsyncInt64Creater
	Metrics sync.Map
	Labels  sync.Map
}

// Metrics contains the list of metrics data that is collected
type Metrics struct {
	UsedCapacity asyncint64.UpDownCounter
}

func (mw *MetricsWrapper) initMetrics(prefix, metaID string, labels []attribute.KeyValue) (*Metrics, error) {
	used, err := mw.Meter.AsyncInt64().UpDownCounter(prefix+"used_capacity_in_bytes", instrument.WithUnit(unit.Bytes))
	if err != nil {
		return nil, err
	}

	metrics := &Metrics{
		UsedCapacity: used,
	}

	mw.Metrics.Store(metaID, metrics)
	mw.Labels.Store(metaID, labels)

	return metrics, nil
}

// Record will publish metrics data for a given instance
func (mw *MetricsWrapper) Record(ctx context.Context, meta interface{}, usedCapcity int64) error {
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
		}
	default:
		return errors.New("unknown MetaData type")
	}

	metricsMapValue, ok := mw.Metrics.Load(metaID)
	if !ok {
		newMetrics, err := mw.initMetrics(prefix, metaID, labels)
		if err != nil {
			return err
		}
		metricsMapValue = newMetrics
	} else {
		// If Metrics for this MetricsWrapper exist, then check if any labels have changed and update them
		currentLabels, ok := mw.Labels.Load(metaID)
		if !ok {
			newMetrics, err := mw.initMetrics(prefix, metaID, labels)
			if err != nil {
				return err
			}
			metricsMapValue = newMetrics
		} else {
			currentLabels := currentLabels.([]attribute.KeyValue)
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
			if haveLabelsChanged {
				newMetrics, err := mw.initMetrics(prefix, metaID, updatedLabels)
				if err != nil {
					return err
				}
				metricsMapValue = newMetrics
			}
		}
	}

	metrics := metricsMapValue.(*Metrics)
	metrics.UsedCapacity.Observe(ctx, usedCapcity, labels...)

	return nil
}
