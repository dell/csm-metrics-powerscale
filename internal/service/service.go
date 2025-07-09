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
	"context"
	"fmt"
	"strconv"
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
	// DirectoryQuotaType is the type of Quota corresponding to a volume
	DirectoryQuotaType = "directory"
)

// Service contains operations that would be used to interact with a PowerScale system
//
//go:generate mockgen -destination=mocks/service_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service Service
type Service interface {
	ExportQuotaMetrics(context.Context)
	ExportClusterCapacityMetrics(context.Context)
	ExportClusterPerformanceMetrics(context.Context)
	ExportTopologyMetrics(context.Context)
}

// PowerScaleClient contains operations for accessing the PowerScale API
//
//go:generate mockgen -destination=mocks/powerscale_client_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service PowerScaleClient
type PowerScaleClient interface {
	GetFloatStatistics(ctx context.Context, keys []string) (goisilon.FloatStats, error)
	GetAllQuotas(ctx context.Context) (goisilon.QuotaList, error)
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

type TopologyDataUpdate struct {
	PersistentVolumeClaim  string
	PersistentVolumeStatus string
	VolumeClaimName        string
	PersistentVolume       string
	ProvisionedSize        string
	VolumeHandle           string
}

var CurrentTopologyData []TopologyDataUpdate

var DeleteTopologyData string

// VolumeFinder is used to find volume information in kubernetes
//
//go:generate mockgen -destination=mocks/volume_finder_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service VolumeFinder
type VolumeFinder interface {
	GetPersistentVolumes(context.Context) ([]k8s.VolumeInfo, error)
}

// StorageClassFinder is used to find storage classes in kubernetes
//
//go:generate mockgen -destination=mocks/storage_class_finder_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service StorageClassFinder
type StorageClassFinder interface {
	GetStorageClasses(context.Context) ([]v1.StorageClass, error)
}

// LeaderElector will elect a leader
//
//go:generate mockgen -destination=mocks/leader_elector_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service LeaderElector
type LeaderElector interface {
	InitLeaderElection(string, string) error
	IsLeader() bool
}

// ClusterCapacityStatsMetricsRecord used for holding output of the capacity statistics for cluster
type ClusterCapacityStatsMetricsRecord struct {
	ClusterName       string
	TotalCapacity     float64
	RemainingCapacity float64
	UsedPercentage    float64
}

// ClusterPerformanceStatsMetricsRecord used for holding output of the performance statistics for cluster
type ClusterPerformanceStatsMetricsRecord struct {
	ClusterName                       string
	CPUPercentage                     float64
	DiskReadOperationsRate            float64
	DiskWriteOperationsRate           float64
	DiskReadThroughputRate            float64
	DiskWriteThroughputRate           float64
	DirectoryTotalHardQuota           float64
	DirectoryTotalHardQuotaPercentage float64
}

// VolumeQuotaMetricsRecord used for holding output of the Volume stat query results
type VolumeQuotaMetricsRecord struct {
	volumeMeta            *VolumeMeta
	quotaSubscribed       int64
	hardQuotaRemaining    int64
	quotaSubscribedPct    float64
	hardQuotaRemainingPct float64
	// pvcSize               int64
}

// ClusterQuotaRecord used for holding output of the Volume stat query results
type ClusterQuotaRecord struct {
	clusterMeta       *ClusterMeta
	totalHardQuota    int64
	totalHardQuotaPct float64
}

type TopologyMetricsRecord struct {
	topologyMeta *TopologyMeta
	pvcSize      int64
}

// ExportQuotaMetrics records quota metrics for the given list of Volumes
func (s *PowerScaleService) ExportQuotaMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportQuotaMetrics")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportQuotaMetrics")
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

	cluster2Quotas := make(map[string]goisilon.QuotaList)
	for clusterName, client := range s.PowerScaleClients {
		quotaList, err := client.GetAllQuotas(ctx)
		if err != nil {
			s.Logger.WithError(err).WithField("cluster_name", clusterName).Error("getting quotas")
			continue
		}
		cluster2Quotas[clusterName] = quotaList
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for range s.pushVolumeQuotaMetrics(ctx, s.gatherVolumeQuotaMetrics(ctx, cluster2Quotas, s.volumeServer(ctx, pvs))) {
			// consume the channel until it is empty and closed
		} // revive:disable-line:empty-block
		wg.Done()
	}()

	go func() {
		for range s.pushClusterQuotaMetrics(ctx, s.gatherClusterQuotaMetrics(ctx, cluster2Quotas)) {
			// consume the channel until it is empty and closed
		} // revive:disable-line:empty-block
		wg.Done()
	}()

	wg.Wait()
}

// pushClusterQuotaMetrics will push the provided channel of cluster quota metrics to a data collector
func (s *PowerScaleService) pushClusterQuotaMetrics(ctx context.Context, clusterQuotaMetrics <-chan *ClusterQuotaRecord) <-chan string {
	start := time.Now()
	defer s.timeSince(start, "pushClusterQuotaMetrics")
	var wg sync.WaitGroup

	ch := make(chan string)
	go func() {
		for metrics := range clusterQuotaMetrics {
			wg.Add(1)
			go func(metrics *ClusterQuotaRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterQuota(ctx, metrics.clusterMeta, metrics)
				if err != nil {
					s.Logger.WithError(err).WithField("cluster_name", metrics.clusterMeta.ClusterName).Error("recording quota metrics for cluster")
				} else {
					ch <- metrics.clusterMeta.ClusterName
				}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

// gatherClusterQuotaMetrics will return a channel of volume metrics based on the input of volumes
func (s *PowerScaleService) gatherClusterQuotaMetrics(_ context.Context, cluster2Quotas map[string]goisilon.QuotaList) <-chan *ClusterQuotaRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherClusterQuotaMetrics")

	ch := make(chan *ClusterQuotaRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	go func() {
		for clusterName := range s.PowerScaleClients {
			sem <- struct{}{}
			wg.Add(1)
			meta := ClusterMeta{ClusterName: clusterName}
			go func(meta ClusterMeta) {
				defer func() {
					wg.Done()
					<-sem
				}()
				quotaList := cluster2Quotas[meta.ClusterName]
				if len(quotaList) == 0 {
					return
				}
				highestLevelQuotas := getHighestQuotas(quotaList)
				totalHardQuota := int64(0)
				totalHardQuotaUsage := int64(0)
				for _, quota := range highestLevelQuotas {
					totalHardQuota = totalHardQuota + quota.Thresholds.Hard
					totalHardQuotaUsage = totalHardQuotaUsage + quota.Usage.Logical
				}

				totalHardQuotaPct := float64(0)
				if totalHardQuota != 0 {
					totalHardQuotaPct = float64(totalHardQuotaUsage) * 100.0 / float64(totalHardQuota)
				}

				metric := &ClusterQuotaRecord{
					clusterMeta:       &meta,
					totalHardQuota:    totalHardQuota,
					totalHardQuotaPct: totalHardQuotaPct,
				}
				s.Logger.Debugf("cluster quota metrics %+v", *metric)

				ch <- metric
			}(meta)
		}
		wg.Wait()
		close(ch)
		close(sem)
	}()
	return ch
}

func getHighestQuotas(list goisilon.QuotaList) goisilon.QuotaList {
	highestQuotas := make(goisilon.QuotaList, 0)
	for _, quota := range list {
		if quota.Type != DirectoryQuotaType {
			continue
		}
		isSubLevel := false

		for hIndex := 0; hIndex < len(highestQuotas); hIndex++ {
			// current Quota's level is higher,remove current highest quota
			if strings.Contains(highestQuotas[hIndex].Path, quota.Path+"/") {
				if hIndex == len(highestQuotas)-1 {
					highestQuotas = highestQuotas[:hIndex]
				} else {
					highestQuotas = append(highestQuotas[:hIndex], highestQuotas[hIndex+1:]...)
				}
				hIndex--
			} else if strings.Contains(quota.Path, highestQuotas[hIndex].Path+"/") {
				// current Quota is a children of known Quota
				isSubLevel = true
				break
			}
		}
		if !isSubLevel {
			highestQuotas = append(highestQuotas, quota)
		}
	}
	return highestQuotas
}

// volumeServer will return a channel of volumes that can provide statistics about each volume
func (s *PowerScaleService) volumeServer(_ context.Context, volumes []k8s.VolumeInfo) <-chan k8s.VolumeInfo {
	volumeChannel := make(chan k8s.VolumeInfo, len(volumes))
	go func() {
		for _, volume := range volumes {
			volumeChannel <- volume
		}
		close(volumeChannel)
	}()
	return volumeChannel
}

// gatherVolumeQuotaMetrics will return a channel of volume metrics based on the input of volumes
func (s *PowerScaleService) gatherVolumeQuotaMetrics(ctx context.Context, cluster2Quotas map[string]goisilon.QuotaList,
	volumes <-chan k8s.VolumeInfo,
) <-chan *VolumeQuotaMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherVolumeQuotaMetrics")

	ch := make(chan *VolumeQuotaMetricsRecord)
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

				// volumeName=_=_=exportID=_=_=accessZone=_=_=clusterName
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
					ID:                        volume.VolumeHandle,
					PersistentVolumeName:      volume.PersistentVolume,
					ClusterName:               clusterName,
					AccessZone:                accessZone,
					ExportID:                  exportID,
					StorageClass:              volume.StorageClass,
					Driver:                    volume.Driver,
					IsiPath:                   volume.IsiPath,
					PersistentVolumeClaimName: volume.VolumeClaimName,
					Namespace:                 volume.Namespace,
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

				path := volumeMeta.IsiPath + "/" + volumeID
				var volQuota goisilon.Quota
				for _, q := range cluster2Quotas[clusterName] {
					if q.Path == path && q.Type == DirectoryQuotaType {
						volQuota = q
						break
					}
				}
				if volQuota == nil {
					s.Logger.WithError(err).WithField("volume_id", volumeMeta.ID).Error("getting quota metrics")
					return
				}

				subscribedQuota := volQuota.Usage.Logical
				hardQuotaRemaining := volQuota.Thresholds.Hard - volQuota.Usage.Logical

				subscribedQuotaPct := float64(0)
				hardQuotaRemainingPct := float64(0)
				if volQuota.Thresholds.Hard != 0 {
					subscribedQuotaPct = float64(subscribedQuota) * 100.0 / float64(volQuota.Thresholds.Hard)
					hardQuotaRemainingPct = float64(hardQuotaRemaining) * 100.0 / float64(volQuota.Thresholds.Hard)
				}

				metric := &VolumeQuotaMetricsRecord{
					volumeMeta:            volumeMeta,
					quotaSubscribed:       subscribedQuota,
					hardQuotaRemaining:    hardQuotaRemaining,
					quotaSubscribedPct:    subscribedQuotaPct,
					hardQuotaRemainingPct: hardQuotaRemainingPct,
				}
				s.Logger.Debugf("volume quota metrics %+v", *metric)

				ch <- metric
			}(volume)
		}

		wg.Wait()
		close(ch)
		close(sem)
	}()
	return ch
}

// pushVolumeQuotaMetrics will push the provided channel of volume metrics to a data collector
func (s *PowerScaleService) pushVolumeQuotaMetrics(ctx context.Context, volumeMetrics <-chan *VolumeQuotaMetricsRecord) <-chan string {
	start := time.Now()
	defer s.timeSince(start, "pushVolumeQuotaMetrics")
	var wg sync.WaitGroup

	ch := make(chan string)
	go func() {
		for metrics := range volumeMetrics {
			wg.Add(1)
			go func(metrics *VolumeQuotaMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordVolumeQuota(ctx, metrics.volumeMeta, metrics)
				if err != nil {
					s.Logger.WithError(err).WithField("volume_id", metrics.volumeMeta.ID).Error("recording metrics for volume")
				} else {
					ch <- metrics.volumeMeta.ID
				}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

func (s *PowerScaleService) getPowerScaleClient(_ context.Context, clusterName string) (PowerScaleClient, error) {
	if goPowerScaleClient, ok := s.PowerScaleClients[clusterName]; ok {
		return goPowerScaleClient, nil
	}
	return nil, fmt.Errorf("unable to find client")
}

func (s *PowerScaleService) getClientIsiPath(_ context.Context, clusterName string) (string, error) {
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

// ExportClusterCapacityMetrics records cluster capacity metrics
func (s *PowerScaleService) ExportClusterCapacityMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportClusterCapacityMetrics")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportClusterCapacityMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		s.Logger.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	for range s.pushClusterCapacityStatsMetrics(ctx, s.gatherClusterCapacityStatsMetrics(ctx)) {
		// consume the channel until it is empty and closed
	} // revive:disable-line:empty-block
}

// gatherClusterStatsMetrics will return a channel of array statistics metric
func (s *PowerScaleService) gatherClusterCapacityStatsMetrics(ctx context.Context) <-chan *ClusterCapacityStatsMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherClusterCapacityStatsMetrics")

	ch := make(chan *ClusterCapacityStatsMetricsRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	type StatsKeyFunc func(metric *ClusterCapacityStatsMetricsRecord, value float64)
	statsKeyFuncMap := map[string]StatsKeyFunc{
		"ifs.bytes.total": func(metric *ClusterCapacityStatsMetricsRecord, value float64) {
			metric.TotalCapacity = value
		},
		"ifs.bytes.avail": func(metric *ClusterCapacityStatsMetricsRecord, value float64) {
			metric.RemainingCapacity = value
		},
	}

	// get all stats keys that will be used as REST query string
	statsKeys := make([]string, 0, len(statsKeyFuncMap))
	for k := range statsKeyFuncMap {
		statsKeys = append(statsKeys, k)
	}

	go func() {
		for clusterName, goPowerScaleClient := range s.PowerScaleClients {
			sem <- struct{}{}
			wg.Add(1)
			go func(clusterName string, goPowerScaleClient PowerScaleClient) {
				defer func() {
					wg.Done()
					<-sem
				}()
				stats, err := goPowerScaleClient.GetFloatStatistics(ctx, statsKeys)
				if err != nil {
					s.Logger.WithError(err).WithField("cluster_name", clusterName).Error("getting capacity stats for cluster")
					return
				}

				metric := &ClusterCapacityStatsMetricsRecord{
					ClusterName: clusterName,
				}

				for _, st := range stats.StatsList {
					function, ok := statsKeyFuncMap[st.Key]
					if ok {
						function(metric, st.Value)
					}
				}

				ch <- metric
				s.Logger.Debugf("cluster capacity stats metrics %+v", *metric)
			}(clusterName, goPowerScaleClient)
		}
		wg.Wait()
		close(sem)
		close(ch)
	}()

	return ch
}

// pushClusterStatsMetrics will push the provided channel of cluster stats metrics to a data collector
func (s *PowerScaleService) pushClusterCapacityStatsMetrics(ctx context.Context, clusterStatistics <-chan *ClusterCapacityStatsMetricsRecord) <-chan *ClusterCapacityStatsMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "pushClusterCapacityStatsMetrics")
	var wg sync.WaitGroup

	ch := make(chan *ClusterCapacityStatsMetricsRecord)
	go func() {
		for m := range clusterStatistics {
			wg.Add(1)
			go func(metric *ClusterCapacityStatsMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterCapacityStatsMetrics(ctx, metric)
				if err != nil {
					s.Logger.WithError(err).Errorf("recording capcity stats for PowerScale cluster, metric=%+v", *metric)
				}

				ch <- metric
			}(m)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

// ExportClusterPerformanceMetrics records cluster performance metrics
func (s *PowerScaleService) ExportClusterPerformanceMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportClusterPerformanceMetrics")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportClusterPerformanceMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		s.Logger.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	for range s.pushClusterPerformanceStatsMetrics(ctx, s.gatherClusterPerformanceStatsMetrics(ctx)) {
		// consume the channel until it is empty and closed
	} // revive:disable-line:empty-block
}

// gatherClusterPerformanceStatsMetrics will return a channel of array statistics metric
func (s *PowerScaleService) gatherClusterPerformanceStatsMetrics(ctx context.Context) <-chan *ClusterPerformanceStatsMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherClusterPerformanceStatsMetrics")

	ch := make(chan *ClusterPerformanceStatsMetricsRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	type StatsKeyFunc func(metric *ClusterPerformanceStatsMetricsRecord, value float64)
	statsKeyFuncMap := map[string]StatsKeyFunc{
		// Cluster average of system CPU usage in tenths of a percent
		"cluster.cpu.sys.avg": func(metric *ClusterPerformanceStatsMetricsRecord, value float64) {
			metric.CPUPercentage = value
		},
		"cluster.disk.xfers.out.rate": func(metric *ClusterPerformanceStatsMetricsRecord, value float64) {
			metric.DiskReadOperationsRate = value
		},
		"cluster.disk.xfers.in.rate": func(metric *ClusterPerformanceStatsMetricsRecord, value float64) {
			metric.DiskWriteOperationsRate = value
		},
		"cluster.disk.bytes.out.rate": func(metric *ClusterPerformanceStatsMetricsRecord, value float64) {
			metric.DiskReadThroughputRate = value
		},
		"cluster.disk.bytes.in.rate": func(metric *ClusterPerformanceStatsMetricsRecord, value float64) {
			metric.DiskWriteThroughputRate = value
		},
	}

	// get all stats keys that will be used as REST query string
	statsKeys := make([]string, 0, len(statsKeyFuncMap))
	for k := range statsKeyFuncMap {
		statsKeys = append(statsKeys, k)
	}

	go func() {
		for clusterName, goPowerScaleClient := range s.PowerScaleClients {
			sem <- struct{}{}
			wg.Add(1)
			go func(clusterName string, goPowerScaleClient PowerScaleClient) {
				defer func() {
					wg.Done()
					<-sem
				}()
				stats, err := goPowerScaleClient.GetFloatStatistics(ctx, statsKeys)
				if err != nil {
					s.Logger.WithError(err).WithField("cluster_name", clusterName).Error("getting performance stats for cluster")
					return
				}

				metric := &ClusterPerformanceStatsMetricsRecord{
					ClusterName: clusterName,
				}

				for _, st := range stats.StatsList {
					function, ok := statsKeyFuncMap[st.Key]
					if ok {
						function(metric, st.Value)
					}
				}

				ch <- metric
				s.Logger.Debugf("cluster performance stats metrics %+v", *metric)
			}(clusterName, goPowerScaleClient)
		}
		wg.Wait()
		close(sem)
		close(ch)
	}()

	return ch
}

// pushClusterPerformanceStatsMetrics will push the provided channel of cluster performance stats metrics to a data collector
func (s *PowerScaleService) pushClusterPerformanceStatsMetrics(ctx context.Context, clusterStatistics <-chan *ClusterPerformanceStatsMetricsRecord) <-chan *ClusterPerformanceStatsMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "pushClusterPerformanceStatsMetrics")
	var wg sync.WaitGroup

	ch := make(chan *ClusterPerformanceStatsMetricsRecord)
	go func() {
		for m := range clusterStatistics {
			wg.Add(1)
			go func(metric *ClusterPerformanceStatsMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterPerformanceStatsMetrics(ctx, metric)
				if err != nil {
					s.Logger.WithError(err).Errorf("recording performance stats for PowerScale cluster, metric=%+v", *metric)
				}

				ch <- metric
			}(m)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

func (s *PowerScaleService) ExportTopologyMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportTopologyMetrics")
	DeleteTopologyData = ""

	fmt.Printf("ExportTopologyMetrics starts\n")

	if s.MetricsWrapper == nil {
		s.Logger.Warn("no MetricsWrapper provided for getting ExportTopologyMetrics")
		return
	}

	// if s.MaxPowerScaleConnections == 0 {
	// 	s.Logger.Debug("Using DefaultMaxPowerScaleConnections")
	// 	s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	// }

	pvs, err := s.VolumeFinder.GetPersistentVolumes(ctx)
	if err != nil {
		s.Logger.WithError(err).Error("getting persistent volumes")
		return
	}

	if CurrentTopologyData == nil {
		CurrentTopologyData = make([]TopologyDataUpdate, 0)
		for _, pv := range pvs {
			fmt.Printf("pv: %+v\n", pv)
			topologyData := TopologyDataUpdate{
				PersistentVolume:       pv.PersistentVolume,
				ProvisionedSize:        pv.ProvisionedSize,
				PersistentVolumeStatus: pv.PersistentVolumeStatus,
				PersistentVolumeClaim:  pv.PersistentVolumeClaim,
				VolumeClaimName:        pv.VolumeClaimName,
			}
			CurrentTopologyData = append(CurrentTopologyData, topologyData)
			// CurrentTopologyData[i].PersistentVolume = pv.PersistentVolume
			// CurrentTopologyData[i].ProvisionedSize = pv.ProvisionedSize
			// CurrentTopologyData[i].PersistentVolumeStatus = pv.PersistentVolumeStatus
			// CurrentTopologyData[i].PersistentVolumeClaim = pv.PersistentVolumeClaim
			// CurrentTopologyData[i].VolumeClaimName = pv.VolumeClaimName
		}
	} else {
		// for _, pv := range pvs {
		// var found bool

		// 	for j, currentData := range CurrentTopologyData {

		// 		if currentData.PersistentVolume == pv.PersistentVolume && (CurrentTopologyData[j].ProvisionedSize != pv.ProvisionedSize || CurrentTopologyData[j].PersistentVolumeStatus != pv.PersistentVolumeStatus || CurrentTopologyData[j].PersistentVolumeClaim != pv.PersistentVolumeClaim ||
		// 			CurrentTopologyData[j].VolumeClaimName != pv.VolumeClaimName) {

		// 			fmt.Printf("There has been a change to volume %v\n", pv.PersistentVolume)
		// 		}
		// 	}
		// }

		for _, pv := range pvs {
			var found bool
			for _, currentData := range CurrentTopologyData {
				if currentData.PersistentVolume == pv.PersistentVolume {
					found = true
					break
				}
			}
			if found {
				fmt.Printf("Volume %v is already present in CurrentTopologyData\n", pv.PersistentVolume)
				fmt.Printf("Checking if there has been a change to volume %v\n", pv.PersistentVolume)
				for _, currentData := range CurrentTopologyData {
					if currentData.ProvisionedSize != pv.ProvisionedSize ||
						currentData.PersistentVolumeStatus != pv.PersistentVolumeStatus ||
						currentData.PersistentVolumeClaim != pv.PersistentVolumeClaim ||
						currentData.VolumeClaimName != pv.VolumeClaimName {
						DeleteTopologyData = pv.PersistentVolume
					}
				}

			} else {
				fmt.Printf("Volume %v is not present in CurrentTopologyData\n", pv.PersistentVolume)
				DeleteTopologyData = pv.PersistentVolume
			}
		}
	}

	// Verify if data is deleted

	fmt.Printf("*************************************\npvs: %+v\n", pvs)

	// cluster2Quotas := make(map[string]goisilon.QuotaList)
	// for clusterName, client := range s.PowerScaleClients {
	// 	quotaList, err := client.GetAllQuotas(ctx)
	// 	if err != nil {
	// 		s.Logger.WithError(err).WithField("cluster_name", clusterName).Error("getting quotas")
	// 		continue
	// 	}
	// 	cluster2Quotas[clusterName] = quotaList
	// }

	// var wg sync.WaitGroup
	// wg.Add(2)
	// go func() {
	for range s.pushTopologyMetrics(ctx, s.gatherTopologyMetrics(ctx, s.volumeServer(ctx, pvs))) {
		// consume the channel until it is empty and closed
	} // revive:disable-line:empty-block
	// 	wg.Done()
	// }()

	// go func() {
	// 	for range s.pushClusterQuotaMetrics(ctx, s.gatherClusterQuotaMetrics(ctx, cluster2Quotas)) {
	// 		// consume the channel until it is empty and closed
	// 	} // revive:disable-line:empty-block
	// 	wg.Done()
	// }()

	// wg.Wait()
}

func (s *PowerScaleService) gatherTopologyMetrics(ctx context.Context,
	volumes <-chan k8s.VolumeInfo,
) <-chan *TopologyMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherTopologyMetrics")

	ch := make(chan *TopologyMetricsRecord)
	var wg sync.WaitGroup
	// sem := make(chan struct{}, s.MaxPowerScaleConnections)
	s.Logger.Info("Volume is: ", len(volumes))

	go func() {
		// storageClasses := make(map[string]v1.StorageClass)
		// scs, err := s.StorageClassFinder.GetStorageClasses(ctx)
		// if err != nil {
		// 	s.Logger.WithError(err).Error("failed to get storage classes, skip")
		// }
		// for _, sc := range scs {
		// 	storageClasses[sc.Name] = sc
		// }

		for volume := range volumes {
			wg.Add(1)
			// sem <- struct{}{}
			go func(volume k8s.VolumeInfo) {
				defer wg.Done()

				// volumeName=_=_=exportID=_=_=accessZone=_=_=clusterName
				// VolumeHandle is of the format "volumeHandle: k8s-2217be0fe2=_=_=5=_=_=System=_=_=PIE-Isilon-X"
				volumeProperties := strings.Split(volume.VolumeHandle, "=_=_=")
				if len(volumeProperties) != ExpectedVolumeHandleProperties {
					s.Logger.WithField("volume_handle", volume.VolumeHandle).Warn("unable to get VolumeID and ClusterID from volume handle")
					return
				}

				// volumeMeta := &VolumeMeta{
				// 	ID:                        volume.VolumeHandle,
				// 	PersistentVolumeName:      volume.PersistentVolume,
				// 	ClusterName:               clusterName,
				// 	AccessZone:                accessZone,
				// 	ExportID:                  exportID,
				// 	StorageClass:              volume.StorageClass,
				// 	Driver:                    volume.Driver,
				// 	IsiPath:                   volume.IsiPath,
				// 	PersistentVolumeClaimName: volume.VolumeClaimName,
				// 	Namespace:                 volume.Namespace,
				// }

				// volumeID := volumeProperties[0]
				// exportID := volumeProperties[1]
				// accessZone := volumeProperties[2]
				// clusterName := volumeProperties[3]

				topologyMeta := &TopologyMeta{
					Namespace:               volume.Namespace,
					PersistentVolumeClaim:   volume.VolumeClaimName,
					VolumeClaimName:         volume.PersistentVolume,
					PersistentVolumeStatus:  volume.PersistentVolumeStatus,
					PersistentVolume:        volume.PersistentVolume,
					StorageClass:            volume.StorageClass,
					Driver:                  volume.Driver,
					ProvisionedSize:         volume.ProvisionedSize,
					StorageSystemVolumeName: volume.StorageSystemVolumeName,
					StoragePoolName:         volume.StoragePoolName,
					StorageSystem:           volume.StorageSystem,
					Protocol:                volume.Protocol,
					CreatedTime:             volume.CreatedTime,
				}

				// if volumeMeta.IsiPath == "" {
				// 	if sc, ok := storageClasses[volumeMeta.StorageClass]; ok {
				// 		path := sc.Parameters["IsiPath"]
				// 		s.Logger.WithFields(logrus.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass, "isiPath": path}).Info("setting storage_class_isiPath to volume_isiPath")
				// 		volumeMeta.IsiPath = path
				// 	}
				// 	if volumeMeta.IsiPath == "" {
				// 		s.Logger.WithFields(logrus.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass}).Warn("could not find a StorageClass for Volume, setting client_isiPath to volume_isiPath")
				// 		volumeMeta.IsiPath, _ = s.getClientIsiPath(ctx, clusterName)
				// 	}
				// }

				// path := volumeMeta.IsiPath + "/" + volumeID
				// var volQuota goisilon.Quota
				// for _, q := range cluster2Quotas[clusterName] {
				// 	if q.Path == path && q.Type == DirectoryQuotaType {
				// 		volQuota = q
				// 		break
				// 	}
				// }
				// if volQuota == nil {
				// 	s.Logger.WithError(err).WithField("volume_id", volumeMeta.ID).Error("getting quota metrics")
				// 	return
				// }

				pvcSize, err := convertToBytes(volume.ProvisionedSize)
				if err != nil {
					s.Logger.Debugf("err is: %v", err)
					return
				}
				fmt.Printf("pvcSize *********** %v", pvcSize)
				// hardQuotaRemaining := volQuota.Thresholds.Hard - volQuota.Usage.Logical

				// subscribedQuotaPct := float64(0)
				// hardQuotaRemainingPct := float64(0)
				// if volQuota.Thresholds.Hard != 0 {
				// 	subscribedQuotaPct = float64(subscribedQuota) * 100.0 / float64(volQuota.Thresholds.Hard)
				// 	hardQuotaRemainingPct = float64(hardQuotaRemaining) * 100.0 / float64(volQuota.Thresholds.Hard)
				// }

				// test test ---------------

				pvcSize = 1

				metric := &TopologyMetricsRecord{
					topologyMeta: topologyMeta,
					pvcSize:      pvcSize,
				}

				s.Logger.Debugf("topology metrics -pv name %+v", metric.topologyMeta.PersistentVolume)
				s.Logger.Debugf("topology metrics - pvcsize %+v", metric.pvcSize)
				s.Logger.Debugf("topology metrics - provisionedsize %+v", topologyMeta.ProvisionedSize)

				ch <- metric
			}(volume)
		}

		wg.Wait()
		close(ch)
	}()
	return ch
}

// pushVolumeQuotaMetrics will push the provided channel of volume metrics to a data collector
func (s *PowerScaleService) pushTopologyMetrics(ctx context.Context, topologyMetrics <-chan *TopologyMetricsRecord) <-chan *TopologyMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "pushTopologyMetrics")
	var wg sync.WaitGroup

	ch := make(chan *TopologyMetricsRecord)
	go func() {
		for metrics := range topologyMetrics {
			wg.Add(1)
			go func(metrics *TopologyMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordTopologyMetrics(ctx, metrics.topologyMeta, metrics, DeleteTopologyData)
				if err != nil {
					s.Logger.WithError(err).WithField("volume_id", metrics.topologyMeta.PersistentVolume).Error("recording topology metrics for volume")
				} else {
					fmt.Printf("recorded topology metrics for volume %v and size %v\n", metrics.topologyMeta.PersistentVolume, metrics.topologyMeta.ProvisionedSize)
					ch <- metrics
				}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	// select {
	// case <-ctx.Done():
	// 	fmt.Printf("Context timed out, closing channel\n")
	// 	// Context timed out, close channel
	// 	close(ch)
	// }

	return ch
}

func convertToBytes(s string) (int64, error) {

	fmt.Printf("The pvc size I got is %s", s)
	// Remove the unit from the string
	num := strings.TrimSuffix(s, "Gi")
	// Convert the number to a float64
	numFloat, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, err
	}
	// Convert to bytes (1 GiB = 1024 * 1024 * 1024 bytes)
	bytes := int64(numFloat * 1024 * 1024 * 1024)
	fmt.Printf("The pvc size in bytes is %d", bytes)
	return bytes, nil
}
