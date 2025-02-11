package main

import (
	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/service"
	otlexporters "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters"
	"github.com/sirupsen/logrus"
	"testing"
)

func Test_updateCollectorAddress(t *testing.T) {
	type args struct {
		config   *entrypoint.Config
		exporter *otlexporters.OtlCollectorExporter
		logger   *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateCollectorAddress(tt.args.config, tt.args.exporter, tt.args.logger)
		})
	}
}

func Test_updateMetricsEnabled(t *testing.T) {
	type args struct {
		config *entrypoint.Config
		logger *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateMetricsEnabled(tt.args.config, tt.args.logger)
		})
	}
}

func Test_updatePowerScaleConnection(t *testing.T) {
	type args struct {
		powerScaleSvc      *service.PowerScaleService
		storageClassFinder *k8s.StorageClassFinder
		volumeFinder       *k8s.VolumeFinder
		logger             *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatePowerScaleConnection(tt.args.powerScaleSvc, tt.args.storageClassFinder, tt.args.volumeFinder, tt.args.logger)
		})
	}
}

func Test_updateProvisionerNames(t *testing.T) {
	type args struct {
		volumeFinder       *k8s.VolumeFinder
		storageClassFinder *k8s.StorageClassFinder
		logger             *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateProvisionerNames(tt.args.volumeFinder, tt.args.storageClassFinder, tt.args.logger)
		})
	}
}

func Test_updateService(t *testing.T) {
	type args struct {
		pscaleSvc *service.PowerScaleService
		logger    *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateService(tt.args.pscaleSvc, tt.args.logger)
		})
	}
}

func Test_updateTickIntervals(t *testing.T) {
	type args struct {
		config *entrypoint.Config
		logger *logrus.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateTickIntervals(tt.args.config, tt.args.logger)
		})
	}
}
