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

package common

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/dell/goisilon"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
)

const (
	defaultEndpointPort  = "8080"
	defaultIsiPath       = "/ifs/data/csi"
	defaultIsiPermission = "0777"
	// defaultVerbose:0 is Verbose_High to log full content of the HTTP request and response
	defaultVerbose     = 0
	defaultIsiAuthType = 1
	defaultInsecure    = true
)

// GetPowerScaleClusters parses config.yaml file, initializes goisilon Clients and composes map of clusters for ease of access.
// It will return cluster that can be used as default as a second return parameter.
// If config does not have any cluster as a default then the first will be returned as a default.
func GetPowerScaleClusters(filePath string, logger *logrus.Logger) (map[string]*service.PowerScaleCluster, *service.PowerScaleCluster, error) {
	type config struct {
		Clusters []*service.PowerScaleCluster `yaml:"isilonClusters"`
	}

	data, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		logger.WithError(err).Errorf("cannot read file %s", filePath)
		return nil, nil, err
	}

	var cfg config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		logger.WithError(err).Errorf("cannot unmarshal data")
		return nil, nil, err
	}

	clusterMap := make(map[string]*service.PowerScaleCluster)
	var defaultCluster *service.PowerScaleCluster
	foundDefault := false

	if len(cfg.Clusters) == 0 {
		return clusterMap, defaultCluster, nil
	}

	// Safeguard if user doesn't set any cluster as default, we just use first one
	defaultCluster = cfg.Clusters[0]

	// Convert to map for convenience and init goisilon.Client
	for _, cluster := range cfg.Clusters {
		cluster := cluster
		if cluster == nil {
			return clusterMap, defaultCluster, nil
		}
		if cluster.ClusterName == "" {
			return nil, nil, errors.New("no ClusterName field found in config.yaml, update config.yaml according to the documentation")
		}

		if strings.HasPrefix(cluster.Endpoint, "https://") {
			cluster.Endpoint = strings.TrimPrefix(cluster.Endpoint, "https://")
		}

		if cluster.EndpointPort == "" {
			logger.Warningf("endpoint port is empty, use default EndpointPort: %s", defaultEndpointPort)
			cluster.EndpointPort = defaultEndpointPort
		}
		if cluster.IsiPath == "" {
			logger.Warningf("IsiPath is empty, use defaultIsiPath: %s", defaultIsiPath)
			cluster.IsiPath = defaultIsiPath
		}
		if cluster.IsiVolumePathPermissions == "" {
			logger.Warningf("IsiVolumePathPermissions is empty, use defaultIsiPermission: %s", defaultIsiPermission)
			cluster.IsiVolumePathPermissions = defaultIsiPermission
		}

		cluster.Verbose = uint(viper.GetInt("POWERSCALE_ISICLIENT_VERBOSE"))
		if cluster.Verbose != 0 && cluster.Verbose != 1 && cluster.Verbose != 2 {
			logger.Warningf("POWERSCALE_ISICLIENT_VERBOSE is invalid,setting to defaultVerbose: %d", defaultVerbose)
			cluster.Verbose = defaultVerbose
		}

		cluster.IsiAuthType = uint8(viper.GetInt("POWERSCALE_ISICLIENT_AUTH_TYPE"))
		if cluster.IsiAuthType != 1 && cluster.IsiAuthType != 0 {
			logger.Warningf("POWERSCALE_ISICLIENT_AUTH_TYPE is invalid, setting it to defaultIsiAuthType: %d", defaultIsiAuthType)
			cluster.IsiAuthType = defaultIsiAuthType
		}

		insecure := viper.GetString("POWERSCALE_ISICLIENT_INSECURE")
		if insecure == "false" {
			cluster.Insecure = false
		} else if insecure == "true" {
			cluster.Insecure = true
		} else {
			logger.Warningf("POWERSCALE_ISICLIENT_INSECURE is invalid, setting it to defaultInsecure: %t", defaultInsecure)
			cluster.Insecure = defaultInsecure
		}

		cluster.EndpointURL = fmt.Sprintf("https://%s:%s", cluster.Endpoint, cluster.EndpointPort)

		logger.WithFields(logrus.Fields{
			"endpoint": cluster.EndpointURL,
			"insecure": cluster.Insecure,
			"username": cluster.Username,
			"authtype": cluster.IsiAuthType,
			"verbose":  cluster.Verbose,
		}).Infof("setting client options")

		c, err := goisilon.NewClientWithArgs(
			context.Background(),
			cluster.EndpointURL,
			cluster.Insecure,
			cluster.Verbose,
			cluster.Username,
			"",
			cluster.Password,
			cluster.IsiPath,
			cluster.IsiVolumePathPermissions,
			false,
			cluster.IsiAuthType)

		if err != nil {
			return nil, nil, status.Errorf(codes.FailedPrecondition, "unable to create PowerScale client: %s", err.Error())
		}
		cluster.Client = c

		clusterMap[cluster.ClusterName] = cluster
		if cluster.IsDefault && !foundDefault {
			defaultCluster = cluster
			foundDefault = true
		}
	}

	return clusterMap, defaultCluster, nil
}
