// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/goisilon"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/storage/v1"
)

var _ Service = (*PowerScaleService)(nil)

const (
	// DefaultMaxPowerScaleConnections is the number of workers that can query powerscale at a time
	DefaultMaxPowerScaleConnections = 10
	// ExpectedVolumeHandleProperties is the number of properties that the VolumeHandle contains
	ExpectedVolumeHandleProperties = 4
)

// Service contains operations that would be used to interact with a PowerScale system
//go:generate mockgen -destination=mocks/service_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service Service
type Service interface {
	ExportVolumeMetrics(context.Context)
	ExportClusterMetrics(context.Context)
}

// PowerScaleClient contains operations for accessing the PowerScale API
//go:generate mockgen -destination=mocks/powerscale_client_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service PowerScaleClient
type PowerScaleClient interface {
	GetStatistics(ctx context.Context, keys []string) (goisilon.Stats, error)
	//return bytes size of Volume
	GetVolumeSize(ctx context.Context, isiPath, name string) (int64, error)
}

// PowerScaleService represents the service for getting metrics data for a PowerScale system
type PowerScaleService struct {
	MetricsWrapper           MetricsRecorder
	MaxPowerScaleConnections int
	Logger                   *logrus.Logger
	PowerScaleClients        map[string]PowerScaleClient
	ClientIsiPaths           map[string]string
	DefaultPowerScaleCluster *PowerScaleCluster
	VolumeFinder             VolumeFinder
	StorageClassFinder       StorageClassFinder
}

// VolumeFinder is used to find volume information in kubernetes
//go:generate mockgen -destination=mocks/volume_finder_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service VolumeFinder
type VolumeFinder interface {
	GetPersistentVolumes(context.Context) ([]k8s.VolumeInfo, error)
}

// StorageClassFinder is used to find storage classes in kubernetes
//go:generate mockgen -destination=mocks/storage_class_finder_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service StorageClassFinder
type StorageClassFinder interface {
	GetStorageClasses(context.Context) ([]v1.StorageClass, error)
}

// LeaderElector will elect a leader
//go:generate mockgen -destination=mocks/leader_elector_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service LeaderElector
type LeaderElector interface {
	InitLeaderElection(string, string) error
	IsLeader() bool
}

// VolumeMetricsRecord used for holding output of the Volume stat query results
type VolumeMetricsRecord struct {
	volumeMeta   *VolumeMeta
	usedCapacity int64
}

// ExportVolumeMetrics records space metrics for the given list of Volumes
func (s *PowerScaleService) ExportVolumeMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportVolumeMetrics")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportVolumeMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		s.Logger.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	pvs, err := s.VolumeFinder.GetPersistentVolumes(ctx)
	if err != nil {
		s.Logger.WithError(err).Error("getting persistent volumes")
		return
	}

	for range s.pushVolumeMetrics(ctx, s.gatherVolumeMetrics(ctx, s.volumeServer(ctx, pvs))) {
		// consume the channel until it is empty and closed
	}
}

// volumeServer will return a channel of volumes that can provide statistics about each volume
func (s *PowerScaleService) volumeServer(ctx context.Context, volumes []k8s.VolumeInfo) <-chan k8s.VolumeInfo {
	volumeChannel := make(chan k8s.VolumeInfo, len(volumes))
	go func() {
		for _, volume := range volumes {
			volumeChannel <- volume
		}
		close(volumeChannel)
	}()
	return volumeChannel
}

// gatherVolumeMetrics will return a channel of volume metrics based on the input of volumes
func (s *PowerScaleService) gatherVolumeMetrics(ctx context.Context, volumes <-chan k8s.VolumeInfo) <-chan *VolumeMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherVolumeMetrics")

	ch := make(chan *VolumeMetricsRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	go func() {
		storageClasses := make(map[string]v1.StorageClass)
		scs, err := s.StorageClassFinder.GetStorageClasses(ctx)
		if err != nil {
			s.Logger.WithError(err).Error("failed to get storage classes, skip")
		}
		for _, sc := range scs {
			storageClasses[sc.Name] = sc
		}

		for volume := range volumes {
			wg.Add(1)
			sem <- struct{}{}
			go func(volume k8s.VolumeInfo) {
				defer func() {
					wg.Done()
					<-sem
				}()

				//volumeName=_=_=exportID=_=_=accessZone=_=_=clusterName
				// VolumeHandle is of the format "volumeHandle: k8s-2217be0fe2=_=_=5=_=_=System=_=_=PIE-Isilon-X"
				volumeProperties := strings.Split(volume.VolumeHandle, "=_=_=")
				if len(volumeProperties) != ExpectedVolumeHandleProperties {
					s.Logger.WithField("volume_handle", volume.VolumeHandle).Warn("unable to get VolumeID and ClusterID from volume handle")
					return
				}

				volumeID := volumeProperties[0]
				exportID := volumeProperties[1]
				accessZone := volumeProperties[2]
				clusterName := volumeProperties[3]

				volumeMeta := &VolumeMeta{
					ID:                   volumeID,
					PersistentVolumeName: volume.PersistentVolume,
					ClusterName:          clusterName,
					AccessZone:           accessZone,
					ExportID:             exportID,
					StorageClass:         volume.StorageClass,
					Driver:               volume.Driver,
					IsiPath:              volume.IsiPath,
				}

				if volumeMeta.IsiPath == "" {
					if sc, ok := storageClasses[volumeMeta.StorageClass]; ok {
						path := sc.Parameters["IsiPath"]
						s.Logger.WithFields(logrus.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass, "isiPath": path}).Info("setting storage_class_isiPath to volume_isiPath")
						volumeMeta.IsiPath = path
					}
					if volumeMeta.IsiPath == "" {
						s.Logger.WithFields(logrus.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass}).Warn("could not find a StorageClass for Volume, setting client_isiPath to volume_isiPath")
						volumeMeta.IsiPath, _ = s.getClientIsiPath(ctx, clusterName)
					}
				}

				goPowerScaleClient, err := s.getPowerScaleClient(ctx, clusterName)
				if err != nil {
					s.Logger.WithError(err).WithField("cluster_name", clusterName).Warn("no client found for PowerScale with clsuter_name")
					return
				}

				volUsedInBytes, err := goPowerScaleClient.GetVolumeSize(ctx, volumeMeta.IsiPath, volumeID)
				if err != nil {
					s.Logger.WithError(err).WithField("volume_id", volumeMeta.ID).Error("getting volume size")
					return
				}

				s.Logger.WithFields(logrus.Fields{
					"volume_meta":    volumeMeta,
					"vol_used_bytes": volUsedInBytes,
				}).Debug("volume metrics")

				ch <- &VolumeMetricsRecord{
					volumeMeta:   volumeMeta,
					usedCapacity: volUsedInBytes,
				}
			}(volume)
		}

		wg.Wait()
		close(ch)
		close(sem)
	}()
	return ch
}

// pushVolumeMetrics will push the provided channel of volume metrics to a data collector
func (s *PowerScaleService) pushVolumeMetrics(ctx context.Context, volumeMetrics <-chan *VolumeMetricsRecord) <-chan string {
	start := time.Now()
	defer s.timeSince(start, "pushVolumeMetrics")
	var wg sync.WaitGroup

	ch := make(chan string)
	go func() {
		for metrics := range volumeMetrics {
			wg.Add(1)
			go func(metrics *VolumeMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.Record(ctx,
					metrics.volumeMeta,
					metrics.usedCapacity,
				)
				if err != nil {
					s.Logger.WithError(err).WithField("volume_id", metrics.volumeMeta.ID).Error("recording metrics for volume")
				} else {
					ch <- fmt.Sprintf(metrics.volumeMeta.ID)
				}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

func (s *PowerScaleService) getPowerScaleClient(ctx context.Context, clusterName string) (PowerScaleClient, error) {

	if goPowerScaleClient, ok := s.PowerScaleClients[clusterName]; ok {
		return goPowerScaleClient, nil
	}
	return nil, fmt.Errorf("unable to find client")
}

func (s *PowerScaleService) getClientIsiPath(ctx context.Context, clusterName string) (string, error) {
	if path, ok := s.ClientIsiPaths[clusterName]; ok {
		return path, nil
	}

	return "", fmt.Errorf("unable to find isiPath for this client, return empty isipath")
}

// timeSince will log the amount of time spent in a given function
func (s *PowerScaleService) timeSince(start time.Time, fName string) {
	s.Logger.WithFields(logrus.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Info("function duration")
}

// ExportClusterMetrics records cluster metrics(I/O or capacity)
func (s *PowerScaleService) ExportClusterMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportClusterMetrics")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportClusterMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		s.Logger.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	s.Logger.Warning("Not supported, skip export Cluster metrics")
}
