package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// collidingCollector emits two samples with an identical label set, reproducing
// what a secret with duplicated certificates used to do.
type collidingCollector struct {
	desc *prometheus.Desc
}

func (c *collidingCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c *collidingCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1, "colliding")
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1, "colliding")
}

// healthyCollector emits an unrelated, well formed metric.
type healthyCollector struct {
	desc *prometheus.Desc
}

func (c *healthyCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c *healthyCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 42, "healthy")
}

// TestMetricsHandler_DuplicateSeriesDoesNotFailScrape is the endpoint level
// regression guard: even if some collector still manages to emit a duplicate
// series, /metrics must answer 200 and keep serving the remaining metrics
// instead of blanking out completely with HTTP 500.
func TestMetricsHandler_DuplicateSeriesDoesNotFailScrape(t *testing.T) {
	reg := prometheus.NewRegistry()

	desc := prometheus.NewDesc("cert_exporter_test_not_after", "test", []string{"name"}, nil)
	if err := reg.Register(&collidingCollector{desc: desc}); err != nil {
		t.Fatal(err)
	}

	healthyDesc := prometheus.NewDesc("cert_exporter_healthy_not_after", "test", []string{"name"}, nil)
	if err := reg.Register(&healthyCollector{desc: healthyDesc}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	metricsHandler(reg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200 despite a duplicate series, got %d", rec.Code)
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "cert_exporter_healthy_not_after") {
		t.Fatal("expected unrelated metrics to still be served when another collector produces a duplicate series")
	}
}
