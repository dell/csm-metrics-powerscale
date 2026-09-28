/*
 Copyright (c) 2026 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package entrypoint

import (
	"context"
	"fmt"
	"maps"
	"runtime"
	"sync"
	"testing"

	pscaleService "github.com/dell/csm-metrics-powerscale/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type powerScaleFailureSvc struct {
	mu      sync.RWMutex
	obs     interface{}
	clients map[string]pscaleService.PowerScaleClient
}

func (s *powerScaleFailureSvc) ExportQuotaMetrics(context.Context)              {}
func (s *powerScaleFailureSvc) ExportClusterCapacityMetrics(context.Context)    {}
func (s *powerScaleFailureSvc) ExportClusterPerformanceMetrics(context.Context) {}
func (s *powerScaleFailureSvc) ExportTopologyMetrics(context.Context)           {}
func (s *powerScaleFailureSvc) GetObsInstrumenter() interface{}                 { return s.obs }
func (s *powerScaleFailureSvc) GetPowerScaleClients() map[string]pscaleService.PowerScaleClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.clients)
}

func (s *powerScaleFailureSvc) setClients(clients map[string]pscaleService.PowerScaleClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = maps.Clone(clients)
}

func gatherPowerScaleMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func counterPowerScaleObs(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := map[string]string{}
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

func TestRecordPowerScaleExportFailure_UsesCurrentClientMap(t *testing.T) {
	reg := prometheus.NewRegistry()
	inst := pscaleService.NewPSCObsInstrumenter(reg)
	svc := &powerScaleFailureSvc{
		obs: inst,
		clients: map[string]pscaleService.PowerScaleClient{
			"cluster-1": nil,
		},
	}

	recordPowerScaleExportFailure(svc)
	svc.setClients(map[string]pscaleService.PowerScaleClient{
		"cluster-2": nil,
	})
	recordPowerScaleExportFailure(svc)

	mf := gatherPowerScaleMetric(t, reg, "dell_csm_obs_export_success_total")
	require.NotNil(t, mf)

	count, ok := counterPowerScaleObs(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster-1", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 1.0, count)

	count, ok = counterPowerScaleObs(mf, map[string]string{"module": "metrics-powerscale", "cluster_name": "cluster-2", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 1.0, count)
}

func TestRecordPowerScaleExportFailure_IsSafeDuringClientUpdates(_ *testing.T) {
	reg := prometheus.NewRegistry()
	inst := pscaleService.NewPSCObsInstrumenter(reg)
	svc := &powerScaleFailureSvc{
		obs: inst,
		clients: map[string]pscaleService.PowerScaleClient{
			"cluster-1": nil,
		},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			recordPowerScaleExportFailure(svc)
			runtime.Gosched()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			svc.setClients(map[string]pscaleService.PowerScaleClient{
				fmt.Sprintf("cluster-%d", i%10): nil,
			})
			runtime.Gosched()
		}
	}()
	wg.Wait()
}
