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
	"github.com/dell/goisilon"
)

// VolumeMeta is the details of a volume
type VolumeMeta struct {
	ID                        string
	PersistentVolumeName      string
	ClusterName               string
	ExportID                  string
	AccessZone                string
	StorageClass              string
	Driver                    string
	IsiPath                   string
	PersistentVolumeClaimName string
	Namespace                 string
}

// ClusterMeta is the details of a cluster
type ClusterMeta struct {
	ClusterName string
}

// PowerScaleCluster is a struct that stores all PowerScale connection information.
// It stores gopowerscale client that can be directly used to invoke PowerSale API calls.
// This structure is supposed to be parsed from config and mainly is created by GetPowerScaleClusters function.
type PowerScaleCluster struct {
	Endpoint                 string `yaml:"endpoint"`
	EndpointPort             string `yaml:"endpointPort"`
	ClusterName              string `yaml:"clusterName"`
	Username                 string `yaml:"username"`
	Password                 string `yaml:"password"`
	Insecure                 bool   `yaml:"skipCertificateValidation"`
	IsDefault                bool   `yaml:"isDefault"`
	IsiPath                  string `yaml:"isiPath"`
	IsiVolumePathPermissions string `yaml:"isiVolumePathPermissions"`
	ReplicationCertificateID string `yaml:"replicationCertificateID"`

	Verbose     uint
	IsiAuthType uint8

	MountEndpoint string `yaml:"mountEndpoint,omitempty"`
	Client        *goisilon.Client
	EndpointURL   string
	IP            string
}

type TopologyMeta struct {
	Namespace               string
	PersistentVolumeClaim   string
	PersistentVolumeStatus  string
	VolumeClaimName         string
	PersistentVolume        string
	StorageClass            string
	Driver                  string
	ProvisionedSize         string
	StorageSystemVolumeName string
	StoragePoolName         string
	StorageSystem           string
	Protocol                string
	CreatedTime             string
}
