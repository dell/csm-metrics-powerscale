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
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/k8s"
	"github.com/dell/csmlog"
	"github.com/dell/gopowerscale"
	v1 "k8s.io/api/storage/v1"
)

var _ Service = (*PowerScaleService)(nil)

const (
	// DefaultMaxPowerScaleConnections is the number of workers that can query powerscale at a time
	DefaultMaxPowerScaleConnections = 10
	// ExpectedVolumeHandleProperties is the number of properties that the VolumeHandle contains
	ExpectedVolumeHandleProperties = 4
	// DirectoryQuotaType is the type of Quota corresponding to a volume
	DirectoryQuotaType               = "directory"
	obsClusterQuotaMetricCount       = 2
	obsVolumeQuotaMetricCount        = 4
	obsClusterCapacityMetricCount    = 3
	obsClusterPerformanceMetricCount = 5
)

// Service contains operations that would be used to interact with a PowerScale system
//
//go:generate mockgen -destination=mocks/service_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service Service
type Service interface {
	ExportQuotaMetrics(context.Context)
	ExportClusterCapacityMetrics(context.Context)
	ExportClusterPerformanceMetrics(context.Context)
	ExportTopologyMetrics(context.Context)
	GetObsInstrumenter() interface{}
	GetPowerScaleClients() map[string]PowerScaleClient
}

// PowerScaleClient contains operations for accessing the PowerScale API
//
//go:generate mockgen -destination=mocks/powerscale_client_mocks.go -package=mocks github.com/dell/csm-metrics-powerscale/internal/service PowerScaleClient
type PowerScaleClient interface {
	GetFloatStatistics(ctx context.Context, keys []string) (gopowerscale.FloatStats, error)
	GetAllQuotas(ctx context.Context) (gopowerscale.QuotaList, error)
}

// PowerScaleService represents the service for getting metrics data for a PowerScale system
type PowerScaleService struct {
	MetricsWrapper           MetricsRecorder
	ObsInstrumenter          *PSCObsInstrumenter
	MaxPowerScaleConnections int
	PowerScaleClients        map[string]PowerScaleClient
	ClientIsiPaths           map[string]string
	DefaultPowerScaleCluster *PowerScaleCluster
	VolumeFinder             VolumeFinder
	StorageClassFinder       StorageClassFinder
	mu                       sync.RWMutex
}

// GetObsInstrumenter returns the observability instrumenter for this service
func (s *PowerScaleService) GetObsInstrumenter() interface{} {
	return s.ObsInstrumenter
}

// GetPowerScaleClients returns a snapshot of the PowerScale clients map.
func (s *PowerScaleService) GetPowerScaleClients() map[string]PowerScaleClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.PowerScaleClients)
}

// SetPowerScaleClients replaces the PowerScale clients map with a defensive copy.
func (s *PowerScaleService) SetPowerScaleClients(clients map[string]PowerScaleClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PowerScaleClients = maps.Clone(clients)
}

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
}

// ClusterQuotaRecord used for holding output of the Volume stat query results
type ClusterQuotaRecord struct {
	clusterMeta       *ClusterMeta
	totalHardQuota    int64
	totalHardQuotaPct float64
}

type TopologyMetricsRecord struct {
	topologyMeta *TopologyMeta
	pvAvailable  int64
}

type obsExportResult struct {
	clusterName string
	metricCount int
	exported    bool
}

// ExportQuotaMetrics records quota metrics for the given list of Volumes
func (s *PowerScaleService) ExportQuotaMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportQuotaMetrics")

	if s.MetricsWrapper == nil {
		csmlog.Warn("no MetricsWrapper provided for getting ExportQuotaMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		csmlog.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	pvs, err := s.VolumeFinder.GetPersistentVolumes(ctx)
	if err != nil {
		csmlog.Errorf("getting persistent volumes: %v", err)
		return
	}

	clients := s.GetPowerScaleClients()
	clusterConnected := make(map[string]bool)
	collectionCounts := make(map[string]int)
	exportFailed := make(map[string]bool)
	cluster2Quotas := make(map[string]gopowerscale.QuotaList)
	for clusterName, client := range clients {
		quotaList, err := client.GetAllQuotas(ctx)
		if err != nil {
			csmlog.Errorf("getting quotas for cluster %s: %v", clusterName, err)
			clusterConnected[clusterName] = false
			continue
		}
		clusterConnected[clusterName] = true
		cluster2Quotas[clusterName] = quotaList
	}

	volumeQuotaMetrics := make([]*VolumeQuotaMetricsRecord, 0)
	for metrics := range s.gatherVolumeQuotaMetrics(ctx, cluster2Quotas, s.volumeServer(ctx, pvs)) {
		volumeQuotaMetrics = append(volumeQuotaMetrics, metrics)
	}

	clusterQuotaMetrics := make([]*ClusterQuotaRecord, 0)
	for metrics := range s.gatherClusterQuotaMetrics(ctx, cluster2Quotas) {
		clusterQuotaMetrics = append(clusterQuotaMetrics, metrics)
	}

	processingStart := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)
	go func() {
		for result := range s.pushVolumeQuotaMetrics(ctx, s.volumeQuotaMetricsServer(volumeQuotaMetrics)) {
			mu.Lock()
			collectionCounts[result.clusterName] += result.metricCount
			exportFailed[result.clusterName] = exportFailed[result.clusterName] || !result.exported
			mu.Unlock()
		}
		wg.Done()
	}()

	go func() {
		for result := range s.pushClusterQuotaMetrics(ctx, s.clusterQuotaServer(clusterQuotaMetrics)) {
			mu.Lock()
			collectionCounts[result.clusterName] += result.metricCount
			exportFailed[result.clusterName] = exportFailed[result.clusterName] || !result.exported
			mu.Unlock()
		}
		wg.Done()
	}()

	wg.Wait()

	processingLatency := time.Since(processingStart)
	s.recordObservabilityMetrics(time.Since(start), processingLatency, clusterConnected, collectionCounts, exportFailed)
}

// pushClusterQuotaMetrics will push the provided channel of cluster quota metrics to a data collector
func (s *PowerScaleService) pushClusterQuotaMetrics(ctx context.Context, clusterQuotaMetrics <-chan *ClusterQuotaRecord) <-chan obsExportResult {
	var wg sync.WaitGroup

	ch := make(chan obsExportResult)
	go func() {
		start := time.Now()
		defer s.timeSince(start, "pushClusterQuotaMetrics")
		for metrics := range clusterQuotaMetrics {
			wg.Add(1)
			go func(metrics *ClusterQuotaRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterQuota(ctx, metrics.clusterMeta, metrics)
				if err != nil {
					csmlog.Errorf("recording quota metrics for cluster %s: %v", metrics.clusterMeta.ClusterName, err)
				}
				ch <- obsExportResult{clusterName: metrics.clusterMeta.ClusterName, metricCount: obsClusterQuotaMetricCount, exported: err == nil}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

// gatherClusterQuotaMetrics will return a channel of volume metrics based on the input of volumes
func (s *PowerScaleService) gatherClusterQuotaMetrics(_ context.Context, cluster2Quotas map[string]gopowerscale.QuotaList) <-chan *ClusterQuotaRecord {
	ch := make(chan *ClusterQuotaRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	go func() {
		start := time.Now()
		defer s.timeSince(start, "gatherClusterQuotaMetrics")
		clients := s.GetPowerScaleClients()
		for clusterName := range clients {
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
				csmlog.Debugf("cluster quota metrics %+v", *metric)

				ch <- metric
			}(meta)
		}
		wg.Wait()
		close(ch)
		close(sem)
	}()
	return ch
}

func getHighestQuotas(list gopowerscale.QuotaList) gopowerscale.QuotaList {
	highestQuotas := make(gopowerscale.QuotaList, 0)
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

func (s *PowerScaleService) clusterQuotaServer(metrics []*ClusterQuotaRecord) <-chan *ClusterQuotaRecord {
	ch := make(chan *ClusterQuotaRecord, len(metrics))
	go func() {
		for _, metric := range metrics {
			ch <- metric
		}
		close(ch)
	}()
	return ch
}

func (s *PowerScaleService) volumeQuotaMetricsServer(metrics []*VolumeQuotaMetricsRecord) <-chan *VolumeQuotaMetricsRecord {
	ch := make(chan *VolumeQuotaMetricsRecord, len(metrics))
	go func() {
		for _, metric := range metrics {
			ch <- metric
		}
		close(ch)
	}()
	return ch
}

func (s *PowerScaleService) clusterCapacityStatsServer(metrics []*ClusterCapacityStatsMetricsRecord) <-chan *ClusterCapacityStatsMetricsRecord {
	ch := make(chan *ClusterCapacityStatsMetricsRecord, len(metrics))
	go func() {
		for _, metric := range metrics {
			ch <- metric
		}
		close(ch)
	}()
	return ch
}

func (s *PowerScaleService) clusterPerformanceStatsServer(metrics []*ClusterPerformanceStatsMetricsRecord) <-chan *ClusterPerformanceStatsMetricsRecord {
	ch := make(chan *ClusterPerformanceStatsMetricsRecord, len(metrics))
	go func() {
		for _, metric := range metrics {
			ch <- metric
		}
		close(ch)
	}()
	return ch
}

// gatherVolumeQuotaMetrics will return a channel of volume metrics based on the input of volumes
func (s *PowerScaleService) gatherVolumeQuotaMetrics(ctx context.Context, cluster2Quotas map[string]gopowerscale.QuotaList,
	volumes <-chan k8s.VolumeInfo,
) <-chan *VolumeQuotaMetricsRecord {
	ch := make(chan *VolumeQuotaMetricsRecord)
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.MaxPowerScaleConnections)

	go func() {
		start := time.Now()
		defer s.timeSince(start, "gatherVolumeQuotaMetrics")
		storageClasses := make(map[string]v1.StorageClass)
		scs, err := s.StorageClassFinder.GetStorageClasses(ctx)
		if err != nil {
			csmlog.Errorf("failed to get storage classes, skip: %v", err)
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
					csmlog.WithFields(csmlog.Fields{"volume_handle": volume.VolumeHandle}).Warn("unable to get VolumeID and ClusterID from volume handle")
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
						csmlog.WithFields(csmlog.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass, "isiPath": path}).Info("setting storage_class_isiPath to volume_isiPath")
						volumeMeta.IsiPath = path
					}
					if volumeMeta.IsiPath == "" {
						csmlog.WithFields(csmlog.Fields{"volume_id": volumeMeta.ID, "storage_class": volumeMeta.StorageClass}).Warn("could not find a StorageClass for Volume, setting client_isiPath to volume_isiPath")
						volumeMeta.IsiPath, _ = s.getClientIsiPath(ctx, clusterName)
					}
				}

				path := volumeMeta.IsiPath + "/" + volumeID
				var volQuota gopowerscale.Quota
				for _, q := range cluster2Quotas[clusterName] {
					if q.Path == path && q.Type == DirectoryQuotaType {
						volQuota = q
						break
					}
				}
				if volQuota == nil {
					csmlog.Errorf("getting quota metrics for volume %s: %v", volumeMeta.ID, err)
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
				csmlog.Debugf("volume quota metrics %+v", *metric)

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
func (s *PowerScaleService) pushVolumeQuotaMetrics(ctx context.Context, volumeMetrics <-chan *VolumeQuotaMetricsRecord) <-chan obsExportResult {
	var wg sync.WaitGroup

	ch := make(chan obsExportResult)
	go func() {
		start := time.Now()
		defer s.timeSince(start, "pushVolumeQuotaMetrics")
		for metrics := range volumeMetrics {
			wg.Add(1)
			go func(metrics *VolumeQuotaMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordVolumeQuota(ctx, metrics.volumeMeta, metrics)
				if err != nil {
					csmlog.Errorf("recording metrics for volume %s: %v", metrics.volumeMeta.ID, err)
				}
				ch <- obsExportResult{clusterName: metrics.volumeMeta.ClusterName, metricCount: obsVolumeQuotaMetricCount, exported: err == nil}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

func (s *PowerScaleService) getPowerScaleClient(_ context.Context, clusterName string) (PowerScaleClient, error) {
	clients := s.GetPowerScaleClients()
	if goPowerScaleClient, ok := clients[clusterName]; ok {
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
	csmlog.WithFields(csmlog.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Info("function duration")
}

func collectionRatePerSecond(metricCount int, elapsed time.Duration) float64 {
	if metricCount <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(metricCount) / elapsed.Seconds()
}

func (s *PowerScaleService) recordObservabilityMetrics(totalElapsed, processingLatency time.Duration, clusterConnected map[string]bool, collectionCounts map[string]int, exportFailed map[string]bool) {
	if s.ObsInstrumenter == nil {
		return
	}

	for clusterName, connected := range clusterConnected {
		s.ObsInstrumenter.SetArrayConnectivity(clusterName, connected)
		s.ObsInstrumenter.RecordCollectionRate(clusterName, collectionRatePerSecond(collectionCounts[clusterName], totalElapsed))
		s.ObsInstrumenter.RecordProcessingLatency(clusterName, processingLatency.Seconds())
		status := "success"
		if !connected {
			status = "error"
		} else if exportFailed[clusterName] {
			status = "failure"
		}
		s.ObsInstrumenter.RecordExportSuccess(clusterName, status)
	}
}

// ExportClusterCapacityMetrics records cluster capacity metrics
func (s *PowerScaleService) ExportClusterCapacityMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportClusterCapacityMetrics")

	if s.MetricsWrapper == nil {
		csmlog.Warn("no MetricsWrapper provided for getting ExportClusterCapacityMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		csmlog.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	clients := s.GetPowerScaleClients()
	successClusters := make(map[string]bool)
	collectionCounts := make(map[string]int)
	exportFailed := make(map[string]bool)
	clusterCapacityMetrics := make([]*ClusterCapacityStatsMetricsRecord, 0, len(clients))
	for metric := range s.gatherClusterCapacityStatsMetrics(ctx) {
		clusterCapacityMetrics = append(clusterCapacityMetrics, metric)
	}

	processingStart := time.Now()
	for result := range s.pushClusterCapacityStatsMetrics(ctx, s.clusterCapacityStatsServer(clusterCapacityMetrics)) {
		successClusters[result.clusterName] = true
		collectionCounts[result.clusterName] += result.metricCount
		exportFailed[result.clusterName] = exportFailed[result.clusterName] || !result.exported
	}
	processingLatency := time.Since(processingStart)

	clusterConnected := make(map[string]bool, len(clients))
	for clusterName := range clients {
		clusterConnected[clusterName] = successClusters[clusterName]
	}

	s.recordObservabilityMetrics(time.Since(start), processingLatency, clusterConnected, collectionCounts, exportFailed)
}

// gatherClusterStatsMetrics will return a channel of array statistics metric
func (s *PowerScaleService) gatherClusterCapacityStatsMetrics(ctx context.Context) <-chan *ClusterCapacityStatsMetricsRecord {
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
		start := time.Now()
		defer s.timeSince(start, "gatherClusterCapacityStatsMetrics")
		clients := s.GetPowerScaleClients()
		for clusterName, goPowerScaleClient := range clients {
			sem <- struct{}{}
			wg.Add(1)
			go func(clusterName string, goPowerScaleClient PowerScaleClient) {
				defer func() {
					wg.Done()
					<-sem
				}()
				stats, err := goPowerScaleClient.GetFloatStatistics(ctx, statsKeys)
				if err != nil {
					csmlog.Errorf("getting capacity stats for cluster %s: %v", clusterName, err)
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
				csmlog.Debugf("cluster capacity stats metrics %+v", *metric)
			}(clusterName, goPowerScaleClient)
		}
		wg.Wait()
		close(sem)
		close(ch)
	}()

	return ch
}

// pushClusterStatsMetrics will push the provided channel of cluster stats metrics to a data collector
func (s *PowerScaleService) pushClusterCapacityStatsMetrics(ctx context.Context, clusterStatistics <-chan *ClusterCapacityStatsMetricsRecord) <-chan obsExportResult {
	var wg sync.WaitGroup

	ch := make(chan obsExportResult)
	go func() {
		start := time.Now()
		defer s.timeSince(start, "pushClusterCapacityStatsMetrics")
		for m := range clusterStatistics {
			wg.Add(1)
			go func(metric *ClusterCapacityStatsMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterCapacityStatsMetrics(ctx, metric)
				if err != nil {
					csmlog.Errorf("recording capcity stats for PowerScale cluster, metric=%+v: %v", err, *metric)
				}

				ch <- obsExportResult{clusterName: metric.ClusterName, metricCount: obsClusterCapacityMetricCount, exported: err == nil}
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
		csmlog.Warn("no MetricsWrapper provided for getting ExportClusterPerformanceMetrics")
		return
	}

	if s.MaxPowerScaleConnections == 0 {
		csmlog.Debug("Using DefaultMaxPowerScaleConnections")
		s.MaxPowerScaleConnections = DefaultMaxPowerScaleConnections
	}

	clients := s.GetPowerScaleClients()
	successClusters := make(map[string]bool)
	collectionCounts := make(map[string]int)
	exportFailed := make(map[string]bool)
	clusterPerformanceMetrics := make([]*ClusterPerformanceStatsMetricsRecord, 0, len(clients))
	for metric := range s.gatherClusterPerformanceStatsMetrics(ctx) {
		clusterPerformanceMetrics = append(clusterPerformanceMetrics, metric)
	}

	processingStart := time.Now()
	for result := range s.pushClusterPerformanceStatsMetrics(ctx, s.clusterPerformanceStatsServer(clusterPerformanceMetrics)) {
		successClusters[result.clusterName] = true
		collectionCounts[result.clusterName] += result.metricCount
		exportFailed[result.clusterName] = exportFailed[result.clusterName] || !result.exported
	}
	processingLatency := time.Since(processingStart)

	clusterConnected := make(map[string]bool, len(clients))
	for clusterName := range clients {
		clusterConnected[clusterName] = successClusters[clusterName]
	}

	s.recordObservabilityMetrics(time.Since(start), processingLatency, clusterConnected, collectionCounts, exportFailed)
}

// gatherClusterPerformanceStatsMetrics will return a channel of array statistics metric
func (s *PowerScaleService) gatherClusterPerformanceStatsMetrics(ctx context.Context) <-chan *ClusterPerformanceStatsMetricsRecord {
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
		start := time.Now()
		defer s.timeSince(start, "gatherClusterPerformanceStatsMetrics")
		clients := s.GetPowerScaleClients()
		for clusterName, goPowerScaleClient := range clients {
			sem <- struct{}{}
			wg.Add(1)
			go func(clusterName string, goPowerScaleClient PowerScaleClient) {
				defer func() {
					wg.Done()
					<-sem
				}()
				stats, err := goPowerScaleClient.GetFloatStatistics(ctx, statsKeys)
				if err != nil {
					csmlog.Errorf("getting performance stats for cluster %s: %v", clusterName, err)
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
				csmlog.Debugf("cluster performance stats metrics %+v", *metric)
			}(clusterName, goPowerScaleClient)
		}
		wg.Wait()
		close(sem)
		close(ch)
	}()

	return ch
}

// pushClusterPerformanceStatsMetrics will push the provided channel of cluster performance stats metrics to a data collector
func (s *PowerScaleService) pushClusterPerformanceStatsMetrics(ctx context.Context, clusterStatistics <-chan *ClusterPerformanceStatsMetricsRecord) <-chan obsExportResult {
	var wg sync.WaitGroup

	ch := make(chan obsExportResult)
	go func() {
		start := time.Now()
		defer s.timeSince(start, "pushClusterPerformanceStatsMetrics")
		for m := range clusterStatistics {
			wg.Add(1)
			go func(metric *ClusterPerformanceStatsMetricsRecord) {
				defer wg.Done()
				err := s.MetricsWrapper.RecordClusterPerformanceStatsMetrics(ctx, metric)
				if err != nil {
					csmlog.Errorf("recording performance stats for PowerScale cluster, metric=%+v: %v", err, *metric)
				}

				ch <- obsExportResult{clusterName: metric.ClusterName, metricCount: obsClusterPerformanceMetricCount, exported: err == nil}
			}(m)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

// ExportTopologyMetrics will export topology metrics
func (s *PowerScaleService) ExportTopologyMetrics(ctx context.Context) {
	start := time.Now()
	defer s.timeSince(start, "ExportTopologyMetrics")

	if s.MetricsWrapper == nil {
		csmlog.Warn("no MetricsWrapper provided for getting ExportTopologyMetrics")
		return
	}

	pvs, err := s.VolumeFinder.GetPersistentVolumes(ctx)
	if err != nil {
		csmlog.Errorf("getting persistent volumes: %v", err)
		return
	}

	for range s.pushTopologyMetrics(ctx, s.gatherTopologyMetrics(s.volumeServer(ctx, pvs))) {
		// consume the channel until it is empty and closed
	} // revive:disable-line:empty-block
}

// gatherTopologyMetrics will return a channel of topology metrics
func (s *PowerScaleService) gatherTopologyMetrics(volumes <-chan k8s.VolumeInfo) <-chan *TopologyMetricsRecord {
	start := time.Now()
	defer s.timeSince(start, "gatherTopologyMetrics")

	ch := make(chan *TopologyMetricsRecord)
	var wg sync.WaitGroup

	go func() {
		for volume := range volumes {
			wg.Add(1)
			go func(volume k8s.VolumeInfo) {
				defer wg.Done()

				// volumeName=_=_=exportID=_=_=accessZone=_=_=clusterName
				// VolumeHandle is of the format "volumeHandle: k8s-2217be0fe2=_=_=5=_=_=System=_=_=PIE-Isilon-X"
				volumeProperties := strings.Split(volume.VolumeHandle, "=_=_=")
				if len(volumeProperties) != ExpectedVolumeHandleProperties {
					csmlog.WithFields(csmlog.Fields{"volume_handle": volume.VolumeHandle}).Warn("unable to get VolumeID and ClusterID from volume handle")
					return
				}

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

				pvAvailable := int64(1)

				metric := &TopologyMetricsRecord{
					topologyMeta: topologyMeta,
					pvAvailable:  pvAvailable,
				}

				ch <- metric
			}(volume)
		}

		wg.Wait()
		close(ch)
	}()
	return ch
}

// pushTopologyMetrics will push the provided channel of volume metrics to a data collector
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
				err := s.MetricsWrapper.RecordTopologyMetrics(ctx, metrics.topologyMeta, metrics)
				if err != nil {
					csmlog.Errorf("recording topology metrics for volume %s: %v", metrics.topologyMeta.PersistentVolume, err)
				} else {
					ch <- metrics
				}
			}(metrics)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}
