/*
 Copyright (c) 2025 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"time"

	csmmodule "github.com/dell/csm-metrics-common/pkg/module"
	"github.com/prometheus/client_golang/prometheus"
)

const obsModuleLabel = "metrics-powerscale"

// PSCObsInstrumenter records observability self-metrics for csm-metrics-powerscale.
type PSCObsInstrumenter struct {
	instrumenter *csmmodule.ObsInstrumenter
}

// NewPSCObsInstrumenter creates and registers a PSCObsInstrumenter.
func NewPSCObsInstrumenter(reg prometheus.Registerer) *PSCObsInstrumenter {
	return &PSCObsInstrumenter{instrumenter: csmmodule.NewObsInstrumenter(reg, "", "cluster_name")}
}

// RecordCollectionRate sets the current collection rate.
func (i *PSCObsInstrumenter) RecordCollectionRate(clusterName string, rate float64) {
	if i == nil || i.instrumenter == nil {
		return
	}
	i.instrumenter.RecordCollectionRate(obsModuleLabel, clusterName, rate)
}

// RecordExportSuccess increments the export success counter.
func (i *PSCObsInstrumenter) RecordExportSuccess(clusterName, status string) {
	if i == nil || i.instrumenter == nil {
		return
	}
	i.instrumenter.RecordExportSuccess(obsModuleLabel, clusterName, status)
}

// SetArrayConnectivity sets the array connectivity gauge.
func (i *PSCObsInstrumenter) SetArrayConnectivity(clusterName string, connected bool) {
	if i == nil || i.instrumenter == nil {
		return
	}
	i.instrumenter.RecordArrayConnectivity(obsModuleLabel, clusterName, connected)
}

// RecordProcessingLatency observes a processing latency sample.
func (i *PSCObsInstrumenter) RecordProcessingLatency(clusterName string, seconds float64) {
	if i == nil || i.instrumenter == nil {
		return
	}
	i.instrumenter.RecordProcessingLatency(obsModuleLabel, clusterName, time.Duration(seconds*float64(time.Second)))
}
