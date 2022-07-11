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
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/storage/v1"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ExportVolumeMetrics(t *testing.T) {
	tests := map[string]func(t *testing.T) (service.PowerScaleService, *gomock.Controller){
		"success": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(3)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1000), nil).Times(2)
			client2 := mocks.NewMockPowerScaleClient(ctrl)
			client2.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(2000), nil).Times(1)
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
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1000), nil).Times(1)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if volume isiPath is wrong": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
					IsiPath:                "invalid_path",
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
						"IsiPath":                  "/ifs/data/csi",
					},
				},
			}, nil).Times(1)
			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("error")).Times(1)
			clients["cluster1"] = client1

			service := service.PowerScaleService{
				MetricsWrapper:     metrics,
				VolumeFinder:       volFinder,
				StorageClassFinder: scFinder,
				PowerScaleClients:  clients,
			}
			return service, ctrl
		},
		"metrics not pushed if error getting volume size": func(*testing.T) (service.PowerScaleService, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			metrics := mocks.NewMockMetricsRecorder(ctrl)
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("error")).Times(1)
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
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			volFinder := mocks.NewMockVolumeFinder(ctrl)
			volFinder.EXPECT().GetPersistentVolumes(gomock.Any()).Return(nil, errors.New("error")).Times(1)
			scFinder := mocks.NewMockStorageClassFinder(ctrl)
			scFinder.EXPECT().GetStorageClasses(gomock.Any()).Times(0)

			clients := make(map[string]service.PowerScaleClient)
			client1 := mocks.NewMockPowerScaleClient(ctrl)
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
			metrics.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(2000), nil).Times(1)
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
			client1.EXPECT().GetVolumeSize(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			clients["cluster1"] = client1

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

			clients := make(map[string]service.PowerScaleClient)
			c := mocks.NewMockPowerScaleClient(ctrl)
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
			service.ExportClusterMetrics(context.Background())
			ctrl.Finish()
		})
	}
}
