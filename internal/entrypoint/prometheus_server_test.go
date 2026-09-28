package entrypoint_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dell/csm-metrics-powerscale/internal/entrypoint"
	"github.com/dell/csm-metrics-powerscale/internal/service/mocks"
	exportermocks "github.com/dell/csm-metrics-powerscale/opentelemetry/exporters/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/mock/gomock"
)

func freeListenAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func TestRunPrometheusMetricsServerServesHealthAndMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderElector := mocks.NewMockLeaderElector(ctrl)
	leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").AnyTimes().Return(nil)
	leaderElector.EXPECT().IsLeader().AnyTimes().Return(false)

	registry := prometheus.NewRegistry()
	metric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_run_metric", Help: "test metric"})
	registry.MustRegister(metric)
	metric.Set(1)

	exporter := exportermocks.NewMockOtlexporter(ctrl)
	exporter.EXPECT().InitExporter(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	exporter.EXPECT().StopExporter().AnyTimes().Return(nil)

	service := mocks.NewMockService(ctrl)
	config := &entrypoint.Config{
		LeaderElector:                  leaderElector,
		ClusterCapacityTickInterval:    10 * time.Second,
		ClusterPerformanceTickInterval: 10 * time.Second,
		QuotaCapacityTickInterval:      10 * time.Second,
		TopologyMetricsTickInterval:    10 * time.Second,
		CollectorAddress:               "localhost:4317",
		PrometheusMetricsEnabled:       true,
		PrometheusRegistry:             registry,
		PrometheusListenAddress:        freeListenAddress(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- entrypoint.Run(ctx, config, exporter, service)
	}()

	baseURL := "http://" + config.PrometheusListenAddress
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("prometheus metrics server did not become healthy")
		}
		time.Sleep(25 * time.Millisecond)
	}

	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from metrics endpoint, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "test_run_metric") {
		t.Fatalf("expected metrics body to contain seeded metric, got %s", string(body))
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("run returned error after cancellation: %v", err)
	}
}

func TestRunInvalidPrometheusTLSConfigReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderElector := mocks.NewMockLeaderElector(ctrl)
	leaderElector.EXPECT().InitLeaderElection("karavi-metrics-powerscale", "karavi").AnyTimes().Return(nil)
	leaderElector.EXPECT().IsLeader().AnyTimes().Return(false)

	exporter := exportermocks.NewMockOtlexporter(ctrl)
	exporter.EXPECT().InitExporter(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	exporter.EXPECT().StopExporter().AnyTimes().Return(nil)

	service := mocks.NewMockService(ctrl)
	config := &entrypoint.Config{
		LeaderElector:                  leaderElector,
		ClusterCapacityTickInterval:    10 * time.Second,
		ClusterPerformanceTickInterval: 10 * time.Second,
		QuotaCapacityTickInterval:      10 * time.Second,
		TopologyMetricsTickInterval:    10 * time.Second,
		CollectorAddress:               "localhost:4317",
		PrometheusMetricsEnabled:       true,
		PrometheusRegistry:             prometheus.NewRegistry(),
		PrometheusListenAddress:        freeListenAddress(t),
		PrometheusCertFile:             "/nonexistent/cert.pem",
		PrometheusKeyFile:              "/nonexistent/key.pem",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := entrypoint.Run(ctx, config, exporter, service)
	if err == nil {
		t.Fatal("expected invalid prometheus tls configuration error")
	}
	if !strings.Contains(err.Error(), "invalid prometheus metrics TLS configuration") {
		t.Fatalf("unexpected error: %v", err)
	}
}
