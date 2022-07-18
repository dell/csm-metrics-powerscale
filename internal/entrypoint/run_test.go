// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package entrypoint_test

import (
	"context"
	"fmt"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"
	exportermocks "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters/mocks"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	pScaleService "github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/golang/mock/gomock"
)

func Test_Run(t *testing.T) {

	tests := map[string]func(t *testing.T) (expectError bool, config *entrypoint.Config, exporter otlexporters.Otlexporter, pScaleSvc pScaleService.Service, prevConfigValidationFunc func(*entrypoint.Config) error, ctrl *gomock.Controller, validatingConfig bool){
		"success": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)

			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").Times(1).Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)

			config := &entrypoint.Config{
				LeaderElector: leaderElector,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)
			svc.EXPECT().ExportVolumeMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterCapacityMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterPerformanceMetrics(gomock.Any()).AnyTimes()

			return false, config, e, svc, prevConfigValidationFunc, ctrl, false
		},
		"error with invalid cluster capacity ticker interval": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			leaderElector := mocks.NewMockLeaderElector(ctrl)
			clients := make(map[string]pScaleService.PowerScaleClient)
			clients["test"] = mocks.NewMockPowerScaleClient(ctrl)
			config := &entrypoint.Config{
				CapacityMetricsEnabled:         true,
				PerformanceMetricsEnabled:      true,
				LeaderElector:                  leaderElector,
				ClusterCapacityTickInterval:    1 * time.Second,
				ClusterPerformanceTickInterval: 5 * time.Second,
				QuotaCapacityTickInterval:      5 * time.Second,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			e := exportermocks.NewMockOtlexporter(ctrl)
			svc := mocks.NewMockService(ctrl)

			return true, config, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"error with invalid cluster performance ticker interval": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			leaderElector := mocks.NewMockLeaderElector(ctrl)
			clients := make(map[string]pScaleService.PowerScaleClient)
			clients["test"] = mocks.NewMockPowerScaleClient(ctrl)
			config := &entrypoint.Config{
				CapacityMetricsEnabled:         true,
				PerformanceMetricsEnabled:      true,
				LeaderElector:                  leaderElector,
				ClusterCapacityTickInterval:    5 * time.Second,
				ClusterPerformanceTickInterval: 1 * time.Second,
				QuotaCapacityTickInterval:      5 * time.Second,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			e := exportermocks.NewMockOtlexporter(ctrl)
			svc := mocks.NewMockService(ctrl)

			return true, config, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"error with invalid quota capacity ticker interval": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			leaderElector := mocks.NewMockLeaderElector(ctrl)
			clients := make(map[string]pScaleService.PowerScaleClient)
			clients["test"] = mocks.NewMockPowerScaleClient(ctrl)
			config := &entrypoint.Config{
				CapacityMetricsEnabled:         true,
				PerformanceMetricsEnabled:      true,
				LeaderElector:                  leaderElector,
				ClusterCapacityTickInterval:    5 * time.Second,
				ClusterPerformanceTickInterval: 5 * time.Second,
				QuotaCapacityTickInterval:      1 * time.Second,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			e := exportermocks.NewMockOtlexporter(ctrl)
			svc := mocks.NewMockService(ctrl)

			return true, config, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"success with capacity false enable ": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)
			clients := make(map[string]pScaleService.PowerScaleClient)
			clients["test"] = mocks.NewMockPowerScaleClient(ctrl)
			config := &entrypoint.Config{
				CapacityMetricsEnabled:         false,
				PerformanceMetricsEnabled:      true,
				LeaderElector:                  leaderElector,
				ClusterCapacityTickInterval:    100 * time.Millisecond,
				ClusterPerformanceTickInterval: 100 * time.Millisecond,
				QuotaCapacityTickInterval:      100 * time.Millisecond,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)
			svc.EXPECT().ExportVolumeMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterCapacityMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterPerformanceMetrics(gomock.Any()).AnyTimes()

			return false, config, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"success with performance false enable ": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)
			clients := make(map[string]pScaleService.PowerScaleClient)
			clients["test"] = mocks.NewMockPowerScaleClient(ctrl)
			config := &entrypoint.Config{
				CapacityMetricsEnabled:         true,
				PerformanceMetricsEnabled:      false,
				LeaderElector:                  leaderElector,
				ClusterCapacityTickInterval:    100 * time.Millisecond,
				ClusterPerformanceTickInterval: 100 * time.Millisecond,
				QuotaCapacityTickInterval:      100 * time.Millisecond,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)
			svc.EXPECT().ExportVolumeMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterCapacityMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterPerformanceMetrics(gomock.Any()).AnyTimes()
			return false, config, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"error nil config": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)
			e := exportermocks.NewMockOtlexporter(ctrl)

			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			svc := mocks.NewMockService(ctrl)

			return true, nil, e, svc, prevConfigValidationFunc, ctrl, true
		},
		"error initializing exporter": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)

			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)

			config := &entrypoint.Config{
				LeaderElector: leaderElector,
			}

			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(fmt.Errorf("An error occurred while initializing the exporter"))
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)

			return true, config, e, svc, prevConfigValidationFunc, ctrl, false
		},
		"success even if leader is false": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)

			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").Times(1).Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(false)

			config := &entrypoint.Config{
				LeaderElector: leaderElector,
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)

			return false, config, e, svc, prevConfigValidationFunc, ctrl, false
		},
		"success using TLS": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)

			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").Times(1).Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)

			config := &entrypoint.Config{
				LeaderElector:     leaderElector,
				CollectorCertPath: "testdata/test-cert.crt",
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)
			svc.EXPECT().ExportVolumeMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterCapacityMetrics(gomock.Any()).AnyTimes()
			svc.EXPECT().ExportClusterPerformanceMetrics(gomock.Any()).AnyTimes()

			return false, config, e, svc, prevConfigValidationFunc, ctrl, false
		},
		"error reading certificate": func(*testing.T) (bool, *entrypoint.Config, otlexporters.Otlexporter, pScaleService.Service, func(*entrypoint.Config) error, *gomock.Controller, bool) {
			ctrl := gomock.NewController(t)

			leaderElector := mocks.NewMockLeaderElector(ctrl)
			leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").AnyTimes().Return(nil)
			leaderElector.EXPECT().IsLeader().AnyTimes().Return(true)

			config := &entrypoint.Config{
				LeaderElector:     leaderElector,
				CollectorCertPath: "testdata/bad-cert.crt",
			}
			prevConfigValidationFunc := entrypoint.ConfigValidatorFunc
			entrypoint.ConfigValidatorFunc = noCheckConfig

			e := exportermocks.NewMockOtlexporter(ctrl)
			e.EXPECT().InitExporter(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			e.EXPECT().StopExporter().Return(nil)

			svc := mocks.NewMockService(ctrl)

			return true, config, e, svc, prevConfigValidationFunc, ctrl, false
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			expectError, config, exporter, svc, prevConfValidation, ctrl, validateConfig := test(t)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if config != nil {
				config.Logger = logrus.New()
				if !validateConfig {
					// The configuration is not nil and the test is not attempting to validate the configuration.
					// In this case, we can use smaller intervals for testing purposes.
					config.ClusterCapacityTickInterval = 100 * time.Millisecond
					config.ClusterPerformanceTickInterval = 100 * time.Millisecond
					config.QuotaCapacityTickInterval = 100 * time.Millisecond
				}
			}
			err := entrypoint.Run(ctx, config, exporter, svc)
			errorOccurred := err != nil
			if expectError != errorOccurred {
				t.Errorf("Unexpected result from test \"%v\": wanted error (%v), but got (%v)", name, expectError, errorOccurred)
			}
			entrypoint.ConfigValidatorFunc = prevConfValidation
			ctrl.Finish()
		})
	}
}

func noCheckConfig(_ *entrypoint.Config) error {
	return nil
}
