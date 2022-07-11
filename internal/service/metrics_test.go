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
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"

	"github.com/golang/mock/gomock"
)

func Test_Metrics_Record(t *testing.T) {
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
				meter := mocks.NewMockAsyncInt64Creater(ctrl)
				provider := mocks.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				used, err := otMeter.AsyncInt64().UpDownCounter(prefix+"used_capacity_in_bytes", instrument.WithUnit(unit.Bytes))
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncInt64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(used, nil)

				return &service.MetricsWrapper{
					Meter: meter,
				}
			}

			mws := []*service.MetricsWrapper{
				getMeter("powerscale_volume_"),
			}

			return mws, checkFns(verifyNoError)
		},
		"error creating used_capacity_in_bytes": func(t *testing.T) ([]*service.MetricsWrapper, []checkFn) {
			ctrl := gomock.NewController(t)
			getMeter := func(prefix string) *service.MetricsWrapper {
				meter := mocks.NewMockAsyncInt64Creater(ctrl)
				provider := mocks.NewMockInstrumentProvider(ctrl)
				otMeter := global.Meter(prefix + "_test")
				used, err := otMeter.AsyncInt64().UpDownCounter(prefix+"used_capacity_in_bytes", instrument.WithUnit(unit.Bytes))
				if err != nil {
					t.Fatal(err)
				}

				meter.EXPECT().AsyncInt64().Return(provider).Times(1)
				provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(used, errors.New("error"))

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
				err := mws[i].Record(context.Background(), metas[i], 10000)
				for _, check := range checks {
					check(t, err)
				}
			}
		})
	}
}

func Test_Volume_Metrics_Label_Update(t *testing.T) {
	metaFirst := &service.VolumeMeta{
		ID:                   "k8s-7242537ae1",
		PersistentVolumeName: "k8s-7242537ae1",
		ClusterName:          "testCluster",
		IsiPath:              "/ifs/data/csi",
		StorageClass:         "isilon",
	}

	metaSecond := &service.VolumeMeta{
		ID:                   "k8s-7242537ae1",
		PersistentVolumeName: "k8s-7242537ae1",
		ClusterName:          "testCluster",
		IsiPath:              "/ifs/data/csi",
		StorageClass:         "isilon",
	}

	metaThird := &service.VolumeMeta{
		ID:                   "k8s-7242537263",
		PersistentVolumeName: "k8s-7242537263",
		ClusterName:          "testCluster",
		IsiPath:              "/ifs/data/csi",
		StorageClass:         "isilon",
	}

	expectedLables := []attribute.KeyValue{
		attribute.String("VolumeID", metaSecond.ID),
		attribute.String("ClusterName", metaSecond.ClusterName),
		attribute.String("PersistentVolumeName", metaSecond.PersistentVolumeName),
		attribute.String("IsiPath", metaSecond.IsiPath),
		attribute.String("StorageClass", metaSecond.StorageClass),
		attribute.String("PlotWithMean", "No"),
	}

	ctrl := gomock.NewController(t)

	meter := mocks.NewMockAsyncInt64Creater(ctrl)
	provider := mocks.NewMockInstrumentProvider(ctrl)
	otMeter := global.Meter("powerscale_volume_test")
	used, err := otMeter.AsyncInt64().UpDownCounter("powerscale_volume_used_capacity_in_bytes", instrument.WithUnit(unit.Bytes))
	if err != nil {
		t.Fatal(err)
	}

	meter.EXPECT().AsyncInt64().Return(provider).Times(2)
	provider.EXPECT().UpDownCounter(gomock.Any(), gomock.Any()).Return(used, nil).Times(2)

	mw := &service.MetricsWrapper{
		Meter: meter,
	}

	t.Run("success: volume metric labels updated", func(t *testing.T) {
		err := mw.Record(context.Background(), metaFirst, 1000)
		if err != nil {
			t.Errorf("expected nil error (record #1), got %v", err)
		}
		err = mw.Record(context.Background(), metaSecond, 1000)
		if err != nil {
			t.Errorf("expected nil error (record #2), got %v", err)
		}
		err = mw.Record(context.Background(), metaThird, 1000)
		if err != nil {
			t.Errorf("expected nil error (record #3), got %v", err)
		}

		newLabels, ok := mw.Labels.Load(metaFirst.ID)
		if !ok {
			t.Errorf("expected labels to exist for %v, but did not find them", metaFirst.ID)
		}
		labels := newLabels.([]attribute.KeyValue)
		for _, l := range labels {
			for _, e := range expectedLables {
				if l.Key == e.Key {
					if l.Value.AsString() != e.Value.AsString() {
						t.Errorf("expected label %v to be updated to %v, but the value was %v", e.Key, e.Value.AsString(), l.Value.AsString())
					}
				}
			}
		}
	})
}
