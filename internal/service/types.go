// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package service

import (
	"github.com/dell/goisilon"
)

// VolumeMeta is the details of a volume in an SDC
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
	NameSpace                 string
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
