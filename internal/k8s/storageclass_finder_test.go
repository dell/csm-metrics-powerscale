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

package k8s_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csm-metrics-powerscale/internal/k8s/mocks"
	"github.com/sirupsen/logrus"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_K8sStorageClassFinder(t *testing.T) {
	type checkFn func(*testing.T, []v1.StorageClass, error)
	check := func(fns ...checkFn) []checkFn { return fns }

	hasNoError := func(t *testing.T, storageClasses []v1.StorageClass, err error) {
		if err != nil {
			t.Fatalf("expected no error")
		}
	}

	checkExpectedOutput := func(expectedOutput []v1.StorageClass) func(t *testing.T, storageClasses []v1.StorageClass, err error) {
		return func(t *testing.T, storageClasses []v1.StorageClass, err error) {
			assert.Equal(t, expectedOutput, storageClasses)
		}
	}

	hasError := func(t *testing.T, volumes []v1.StorageClass, err error) {
		if err == nil {
			t.Fatalf("expected error")
		}
	}

	tests := map[string]func(t *testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller){
		"success not selecting storageclass that is not in config": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)

			storageClasses := &v1.StorageClassList{
				Items: []v1.StorageClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "notExisting",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon-another",
						},
						Provisioner: "csi-driver.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
				},
			}

			expected := &v1.StorageClassList{
				Items: []v1.StorageClass{
					storageClasses.Items[len(storageClasses.Items)-1],
				},
			}

			api.EXPECT().GetStorageClasses().Times(1).Return(storageClasses, nil)
			ids := make([]k8s.ClusterName, 1)
			ids[0] = k8s.ClusterName{ID: "pieisi93x", DriverNames: []string{"csi-driver.dellemc.com"}, IsDefault: false}

			finder := k8s.StorageClassFinder{API: api, ClusterNames: ids}
			return finder, check(hasNoError, checkExpectedOutput(expected.Items)), ctrl
		},
		"success selecting the matching driver name with storage classes": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)

			storageClasses := &v1.StorageClassList{
				Items: []v1.StorageClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
				},
			}

			api.EXPECT().GetStorageClasses().Times(1).Return(storageClasses, nil)
			ids := make([]k8s.ClusterName, 1)
			ids[0] = k8s.ClusterName{ID: "pieisi93x", DriverNames: []string{"csi-isilon.dellemc.com"}, IsDefault: false}

			finder := k8s.StorageClassFinder{API: api, ClusterNames: ids}
			return finder, check(hasNoError, checkExpectedOutput(storageClasses.Items)), ctrl
		},
		"success selecting storage classes matching multiple driver names": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)

			storageClasses := &v1.StorageClassList{
				Items: []v1.StorageClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon-another",
						},
						Provisioner: "another-csi-driver.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
				},
			}

			api.EXPECT().GetStorageClasses().Times(1).Return(storageClasses, nil)
			ids := make([]k8s.ClusterName, 1)
			ids[0] = k8s.ClusterName{ID: "pieisi93x", DriverNames: []string{"csi-isilon.dellemc.com", "another-csi-driver.dellemc.com"}, IsDefault: false}

			finder := k8s.StorageClassFinder{API: api, ClusterNames: ids}

			return finder, check(hasNoError, checkExpectedOutput(storageClasses.Items)), ctrl
		},
		"success matching storage classes without ClusterName based on a default cluster being used": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)

			storageClasses := &v1.StorageClassList{
				Items: []v1.StorageClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "another-isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
				},
			}

			api.EXPECT().GetStorageClasses().Times(1).Return(storageClasses, nil)

			ids := make([]k8s.ClusterName, 1)
			ids[0] = k8s.ClusterName{ID: "pieisi93x", DriverNames: []string{"csi-isilon.dellemc.com", "another-csi-driver.dellemc.com"}, IsDefault: true}

			finder := k8s.StorageClassFinder{API: api, ClusterNames: ids}

			return finder, check(hasNoError, checkExpectedOutput([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "another-isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "pieisi93x",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
			},
			)), ctrl
		},
		"success selecting storage classes matching one of two driver names": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)

			storageClasses := &v1.StorageClassList{
				Items: []v1.StorageClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon",
						},
						Provisioner: "csi-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "isilon-another",
						},
						Provisioner: "another-isilon.dellemc.com",
						Parameters: map[string]string{
							"AccessZone":               "System",
							"ClusterName":              "pieisi93x",
							"IsiPath":                  "/ifs/data/csi",
							"IsiVolumePathPermissions": "0777",
						},
					},
				},
			}

			api.EXPECT().GetStorageClasses().Times(1).Return(storageClasses, nil)

			ids := make([]k8s.ClusterName, 1)
			ids[0] = k8s.ClusterName{ID: "pieisi93x", DriverNames: []string{"csi-isilon.dellemc.com"}}

			finder := k8s.StorageClassFinder{API: api, ClusterNames: ids}

			return finder, check(hasNoError, checkExpectedOutput([]v1.StorageClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "isilon",
					},
					Provisioner: "csi-isilon.dellemc.com",
					Parameters: map[string]string{
						"AccessZone":               "System",
						"ClusterName":              "pieisi93x",
						"IsiPath":                  "/ifs/data/csi",
						"IsiVolumePathPermissions": "0777",
					},
				},
			},
			)), ctrl
		},
		"error calling k8s": func(*testing.T) (k8s.StorageClassFinder, []checkFn, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			api := mocks.NewMockStorageClassGetter(ctrl)
			api.EXPECT().GetStorageClasses().Times(1).Return(nil, errors.New("error"))
			finder := k8s.StorageClassFinder{API: api, Logger: logrus.New()}
			return finder, check(hasError), ctrl
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			finder, checkFns, ctrl := tc(t)
			storageClasses, err := finder.GetStorageClasses(context.Background())
			for _, checkFn := range checkFns {
				checkFn(t, storageClasses, err)
			}
			ctrl.Finish()
		})
	}
}
