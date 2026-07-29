package secret

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const metricName = "cert_exporter_secret_not_after"

func newTestExporter(t *testing.T) *Exporter {
	t.Helper()

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return &Exporter{
		// The production descriptor, so a change to the exported labels is
		// caught here instead of silently passing against a copy.
		cert:   newCertDesc(),
		ctx:    context.Background(),
		logger: logger,
	}
}

func generateSelfSignedCertPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		// Derive a unique serial from the expiry so concatenated certs in a
		// test get distinct serial numbers, mirroring real-world certificates.
		SerialNumber: big.NewInt(notAfter.Unix()),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func TestCalculateExpiry_SingleCert(t *testing.T) {
	e := newTestExporter(t)

	notAfter := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	certPEM := generateSelfSignedCertPEM(t, notAfter)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"ca.crt":  certPEM,
		},
	}

	ch := make(chan prometheus.Metric, 10)
	err := e.calculateExpiry(ch, secret)
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// One metric per key (ca.crt + tls.crt)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
}

func TestCalculateExpiry_MultipleCertsInSameKey(t *testing.T) {
	e := newTestExporter(t)

	notAfter1 := time.Now().Add(1 * time.Hour).Truncate(time.Second)
	notAfter2 := time.Now().Add(48 * time.Hour).Truncate(time.Second)

	// Concatenate two certs into a single PEM, simulating Kyverno's behavior.
	certPEM1 := generateSelfSignedCertPEM(t, notAfter1)
	certPEM2 := generateSelfSignedCertPEM(t, notAfter2)
	combined := append(certPEM1, certPEM2...)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-webhook",
			Namespace: "kyverno",
		},
		Data: map[string][]byte{
			"tls.crt": combined,
			"ca.crt":  generateSelfSignedCertPEM(t, notAfter2),
		},
	}

	ch := make(chan prometheus.Metric, 10)
	err := e.calculateExpiry(ch, secret)
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// tls.crt has 2 certs, ca.crt has 1 = 3 total
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics for concatenated certs, got %d", len(metrics))
	}
}

// secretCollector adapts calculateExpiry to the prometheus.Collector interface
// so the metrics can be exercised through a real registry Gather(), which is
// where duplicate-series collisions surface (a plain channel read does not).
type secretCollector struct {
	e      *Exporter
	secret v1.Secret
}

func (c *secretCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.e.cert }
func (c *secretCollector) Collect(ch chan<- prometheus.Metric) { _ = c.e.calculateExpiry(ch, c.secret) }

// multiSecretCollector collects several secrets in one scrape, so a collision
// caused by one secret can be observed against the metrics of the others.
type multiSecretCollector struct {
	e       *Exporter
	secrets []v1.Secret
}

func (c *multiSecretCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.e.cert }
func (c *multiSecretCollector) Collect(ch chan<- prometheus.Metric) {
	for _, s := range c.secrets {
		_ = c.e.calculateExpiry(ch, s)
	}
}

// TestGather_MultipleCertsInSameKey guards against the regression where two
// certs concatenated in a single secret key produced metrics with identical
// label sets, causing Gather() to fail and blanking out the whole scrape.
func TestGather_MultipleCertsInSameKey(t *testing.T) {
	e := newTestExporter(t)

	certPEM1 := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	certPEM2 := generateSelfSignedCertPEM(t, time.Now().Add(48*time.Hour))
	combined := append(certPEM1, certPEM2...)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno-webhook", Namespace: "kyverno"},
		Data:       map[string][]byte{"tls.crt": combined},
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(&secretCollector{e: e, secret: secret}); err != nil {
		t.Fatal(err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed (duplicate series regression): %v", err)
	}

	var series int
	for _, mf := range mfs {
		series += len(mf.GetMetric())
	}
	if series != 2 {
		t.Fatalf("expected 2 distinct series for concatenated certs, got %d", series)
	}
}

// serveMetrics renders a registry through the same handler configuration
// main.go uses, so tests can assert what a Prometheus scrape actually receives.
func serveMetrics(t *testing.T, reg *prometheus.Registry) (int, string) {
	t.Helper()

	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	return rec.Code, rec.Body.String()
}

func samplesFor(body, secretName string) []string {
	var samples []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, metricName) && strings.Contains(line, `name="`+secretName+`"`) {
			samples = append(samples, line)
		}
	}

	return samples
}

// TestScrape_DuplicateIdenticalCertInSameKey covers a secret key holding the
// *same* certificate twice, which the serialnumber label cannot tell apart. The
// scrape must still succeed and report the certificate once.
func TestScrape_DuplicateIdenticalCertInSameKey(t *testing.T) {
	e := newTestExporter(t)

	certPEM := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	duplicated := append(append([]byte{}, certPEM...), certPEM...)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "duplicated-chain", Namespace: "default"},
		Data:       map[string][]byte{"tls.crt": duplicated},
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(&secretCollector{e: e, secret: secret}); err != nil {
		t.Fatal(err)
	}

	code, body := serveMetrics(t, reg)
	if code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200, got %d", code)
	}

	if got := len(samplesFor(body, "duplicated-chain")); got != 1 {
		t.Fatalf("expected the duplicated certificate to be served once, got %d samples", got)
	}
}

// TestGather_SameCertInBothKeys makes sure a certificate stored under two keys
// is still reported twice: cert-manager routinely stores the CA in both ca.crt
// and tls.crt, and the secretkey label keeps those samples distinct.
func TestGather_SameCertInBothKeys(t *testing.T) {
	e := newTestExporter(t)

	certPEM := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-ca", Namespace: "default"},
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"ca.crt":  certPEM,
		},
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(&secretCollector{e: e, secret: secret}); err != nil {
		t.Fatal(err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}

	var series int
	for _, mf := range mfs {
		series += len(mf.GetMetric())
	}
	if series != 2 {
		t.Fatalf("expected 1 series per secret key, got %d", series)
	}
}

// TestGather_LeafPlusCA models a real chain: a leaf plus its issuing CA
// concatenated in tls.crt. Both must be reported, which is the v2.11.1 fix for
// the original regression.
func TestGather_LeafPlusCA(t *testing.T) {
	e := newTestExporter(t)

	leaf := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	ca := generateSelfSignedCertPEM(t, time.Now().Add(48*time.Hour))

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "leaf-and-ca", Namespace: "default"},
		Data:       map[string][]byte{"tls.crt": append(append([]byte{}, leaf...), ca...)},
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(&secretCollector{e: e, secret: secret}); err != nil {
		t.Fatal(err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed (duplicate series regression): %v", err)
	}

	var series int
	for _, mf := range mfs {
		series += len(mf.GetMetric())
	}
	if series != 2 {
		t.Fatalf("expected 2 distinct series (leaf + CA), got %d", series)
	}
}

// TestScrape_DuplicateDoesNotHideOtherSecrets is the core regression guard: a
// secret carrying duplicated certificates must not take the metrics of
// unrelated secrets down with it. Losing those was the customer visible
// symptom, and it is what ContinueOnError prevents.
func TestScrape_DuplicateDoesNotHideOtherSecrets(t *testing.T) {
	e := newTestExporter(t)

	healthy := generateSelfSignedCertPEM(t, time.Now().Add(72*time.Hour))
	broken := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))

	secrets := []v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "healthy", Namespace: "default"},
			Data:       map[string][]byte{"tls.crt": healthy},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "duplicated", Namespace: "default"},
			Data:       map[string][]byte{"tls.crt": append(append([]byte{}, broken...), broken...)},
		},
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(&multiSecretCollector{e: e, secrets: secrets}); err != nil {
		t.Fatal(err)
	}

	code, body := serveMetrics(t, reg)
	if code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200 despite a duplicate series, got %d", code)
	}

	if got := len(samplesFor(body, "healthy")); got != 1 {
		t.Fatalf("expected the unrelated secret to still be served, got %d samples", got)
	}
}

func TestCalculateExpiry_MissingKey(t *testing.T) {
	e := newTestExporter(t)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "incomplete-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tls.crt": generateSelfSignedCertPEM(t, time.Now().Add(24*time.Hour)),
			// ca.crt is missing
		},
	}

	ch := make(chan prometheus.Metric, 10)
	err := e.calculateExpiry(ch, secret)
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric (only tls.crt), got %d", len(metrics))
	}
}
