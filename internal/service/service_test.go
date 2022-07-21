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
	"github.com/dell/goisilon"
	"github.com/dell/goisilon/api/json"
	isiV1 "github.com/dell/goisilon/api/v1"
	"io/ioutil"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/storage/v1"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ExportVolumeMetrics(t *testing.T) {
	// Mock data for volume space
	mockQuota := &isiV1.IsiQuota{
		Usage: struct {
			Inodes   int64 `json:"inodes"`
			Logical  int64 `json:"logical"`
			Physical int64 `json:"physical"`
		}{Logical: 1000, Physical: 2000},
		Thresholds: struct {
			Advisory             int64       `json:"advisory"`
			AdvisoryExceeded     bool        `json:"advisory_exceeded"`
			AdvisoryLastExceeded interface{} `json:"advisory_last_exceeded"`
			Hard                 int64       `json:"hard"`
			HardExceeded         bool        `json:"hard_exceeded"`
			HardLastExceeded     interface{} `json:"hard_last_exceeded"`
			Soft                 int64       `json:"soft"`
			SoftExceeded         bool        `json:"soft_exceeded"`
			SoftLastExceeded     interface{} `json:"soft_last_exceeded"`
		}{Hard: 2000},
	}

	tests := map[string]func(t *testing.T) (service.PowerScaleService, *gomock.Controller){
		"success": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(3)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
					IsiPath:                "/ifs/data/csi",
				},
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-2",
					StorageClass:           "isilon-another",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster2",
					IsiPath:                "/ifs/data/csi",
				},
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-3",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
					IsiPath:                "/ifs/data/csi",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon-another",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster2",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)
			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)

			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Return(mockQuota, nil).Times(2)
			client2 := mocks.NewMockPowerScaleClient(ctrl)
			client2.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Return(mockQuota, nil).Times(1)
			clients["cluster1"] = client1
			clients["cluster2"] = client2

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"success but volume isiPath is defaultIsiPath": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)
			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Return(mockQuota, nil).Times(1)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if error getting quota": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}

			return service, ctrl
		},
		"metrics not pushed if no cluster name in volume handle": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Times(0)
			clients["cluster2"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if volume handle is invalid": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "invalid-volume-handle",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Times(0)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if volume finder returns error": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return(nil, errors.New("error")).Times(1)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Times(0)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Times(0)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if storage class finder returns error": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{
				{
					Namespace:              "karavi",
					PersistentVolumeClaim:  "pvc-uid",
					PersistentVolumeStatus: "Bound",
					VolumeClaimName:        "pvc-name",
					PersistentVolume:       "pv-1",
					StorageClass:           "isilon",
					Driver:                 "csi-isilon.dellemc.com",
					ProvisionedSize:        "16Gi",
					VolumeHandle:           "k8s-7242537ae1=_=_=19=_=_=System=_=_=cluster1",
				},
			}, nil).Times(1)

			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return(nil, errors.New("error")).Times(1)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Return(mockQuota, nil).Times(1)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if metrics wrapper is nil": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Times(0)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Times(0)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetQuotaWithPath(gomock.Any(), gomock.Any()).Times(0)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     nil,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed with 0 volumes": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Return([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "cluster1",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
			}, nil).Times(1)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return([]k8s.VolumeInfo{}, nil)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Times(0)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			metrics.EXPECT().RecordVolumeSpace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			return service, ctrl
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			service, ctrl := tc(t)
			service.Logger = logrus.New()
			service.ExportVolumeMetrics(context.Background())
			ctrl.Finish()
		})
	}
}

func Test_ExportClusterMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (service.PowerScaleService, *gomock.Controller){
		"success": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			volFinder := mocks.NewMockVolumeFinder(ctrl)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)

			metrics.EXPECT().RecordClusterCapacityStatsMetrics(gomock.Any(), gomock.Any()).Times(1)
			metrics.EXPECT().RecordClusterPerformanceStatsMetrics(gomock.Any(), gomock.Any()).Times(1)

			file := "testdata/recordings/platform-3-statistics-current.json"
			contentBytes, _ := ioutil.ReadFile(file)
			var stats goisilon.FloatStats
			json.Unmarshal(contentBytes, &stats)

			clients := make(map[string]service.PowerScaleClient)
			c := mocks.NewMockPowerScaleClient(ctrl)
			c.EXPECT().GetFloatStatistics(gomock.Any(), gomock.Any()).Return(stats, nil).Times(2)
			clients["cluster1"] = c

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if metrics wrapper is nil": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			volFinder := mocks.NewMockVolumeFinder(ctrl)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)

			clients := make(map[string]service.PowerScaleClient)
			c := mocks.NewMockPowerScaleClient(ctrl)
			clients["cluster1"] = c

			service := service.PowerScaleService{
				MetricsWrapper:     nil,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			service, ctrl := tc(t)
			service.Logger = logrus.New()
			service.ExportClusterCapacityMetrics(context.Background())
			service.ExportClusterPerformanceMetrics(context.Background())
			ctrl.Finish()
		})
	}
}
