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

package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
)

// VolumeGetter is an interface for getting a list of persistent volume information
//
//go:generate mockgen -destination=mocks/volume_getter_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/k8s VolumeGetter
type VolumeGetter interface {
	GetPersistentVolumes() (*corev1.PersistentVolumeList, error)
}

// VolumeFinder is a volume finder that will query the Kubernetes API for Persistent Volumes created by a matching DriverName and StorageSystemID
type VolumeFinder struct {
	API         VolumeGetter
	DriverNames []string
	Logger      *logrus.Logger
}

// VolumeInfo contains information about mapping a Persistent Volume to the volume created on a storage system
type VolumeInfo struct {
	Namespace              string `json:"namespace"`
	PersistentVolumeClaim  string `json:"persistent_volume_claim"`
	PersistentVolumeStatus string `json:"volume_status"`
	VolumeClaimName        string `json:"volume_claim_name"`
	PersistentVolume       string `json:"persistent_volume"`
	StorageClass           string `json:"storage_class"`
	Driver                 string `json:"driver"`
	ProvisionedSize        string `json:"provisioned_size"`
	CreatedTime            string `json:"created_time"`
	VolumeHandle           string `json:"volume_handle"`
	IsiPath                string `json:"isipath"`
}

// GetPersistentVolumes will return a list of persistent volume information
func (f VolumeFinder) GetPersistentVolumes(ctx context.Context) ([]VolumeInfo, error) {
	volumeInfo := make([]VolumeInfo, 0)

	volumes, err := f.API.GetPersistentVolumes()
	if err != nil {
		return nil, err
	}

	for _, volume := range volumes.Items {
		if volume.Spec.CSI == nil {
			f.Logger.Debugf("The PV, %s , is not provisioned by a CSI driver\n", volume.GetName())
			continue
		}
		if contains(f.DriverNames, volume.Spec.CSI.Driver) {
			capacity := volume.Spec.Capacity[corev1.ResourceStorage]
			claim := volume.Spec.ClaimRef
			status := volume.Status

			var isiPath string
			//fullPath sample: /ifs/data/csi/k8s-286356f237
			if fullPath, ok := volume.Spec.CSI.VolumeAttributes["Path"]; ok {
				isiPath = strings.TrimSuffix(fullPath, fmt.Sprintf("/%s", volume.Name))
			}

			info := VolumeInfo{
				Namespace:              claim.Namespace,
				PersistentVolumeClaim:  string(claim.UID),
				VolumeClaimName:        claim.Name,
				PersistentVolumeStatus: string(status.Phase),
				PersistentVolume:       volume.Name,
				StorageClass:           volume.Spec.StorageClassName,
				Driver:                 volume.Spec.CSI.Driver,
				ProvisionedSize:        capacity.String(),
				CreatedTime:            volume.CreationTimestamp.String(),
				VolumeHandle:           volume.Spec.CSI.VolumeHandle,
				IsiPath:                isiPath,
			}
			volumeInfo = append(volumeInfo, info)
		}
	}

	return volumeInfo, nil
}

func contains(slice []string, value string) bool {
	for _, element := range slice {
		if element == value {
			return true
		}
	}
	return false
}
