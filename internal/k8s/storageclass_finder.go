/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/storage/v1"
)

// StorageClassGetter is an interface for getting a list of storage class information
//
//go:generate mockgen -destination=mocks/storage_class_getter_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/k8s StorageClassGetter
type StorageClassGetter interface {
	GetStorageClasses() (*v1.StorageClassList, error)
}

// ClusterName contains Name, whether is default and associated drivernames
type ClusterName struct {
	ID          string
	IsDefault   bool
	DriverNames []string
}

// StorageClassFinder is a storage class finder that will query the Kubernetes API for storage classes provisioned by a matching DriverName and StorageSystemID
type StorageClassFinder struct {
	API          StorageClassGetter
	ClusterNames []ClusterName
	Logger       *logrus.Logger
}

// GetStorageClasses will return a list of storage classes that match the given DriverName in Kubernetes
func (f *StorageClassFinder) GetStorageClasses(_ context.Context) ([]v1.StorageClass, error) {
	var storageClasses []v1.StorageClass

	classes, err := f.API.GetStorageClasses()
	if err != nil {
		f.Logger.WithError(err).Warn("getting storage classes")
		return nil, err
	}

	for _, class := range classes.Items {
		if f.isMatch(class) {
			storageClasses = append(storageClasses, class)
		}
	}

	return storageClasses, nil
}

func (f *StorageClassFinder) isMatch(class v1.StorageClass) bool {
	for _, cluster := range f.ClusterNames {
		if !Contains(cluster.DriverNames, class.Provisioner) {
			continue
		}

		clusterName, ok := class.Parameters["ClusterName"]

		if (clusterName == cluster.ID) || (!ok && cluster.IsDefault) {
			return true
		}
	}

	return false
}

// Contains will return true if the slice contains the given value
func Contains(slice []string, value string) bool {
	for _, element := range slice {
		if element == value {
			return true
		}
	}
	return false
}
