package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/afero"
)

const metricName = "cert_exporter_not_after"

func newTestExporter(t *testing.T, fs afero.Fs, paths []string) *Exporter {
	t.Helper()

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return &Exporter{
		// The production descriptor, so a change to the exported labels is
		// caught here instead of silently passing against a copy.
		cert:   newCertDesc(),
		fs:     fs,
		logger: logger,
		paths:  paths,
	}
}

func generateSelfSignedCertPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Derive a unique serial from the expiry so concatenated certs in a
	// test get distinct serial numbers, mirroring real-world certificates.
	return generateCertPEM(t, key, notAfter.Unix(), notAfter)
}

// generateCertPEM issues a self-signed certificate with an explicit key and
// serial, so a test can reproduce Kyverno's CA rotation: the renewed CA reuses
// the private key and SerialNumber 0.
func generateCertPEM(t *testing.T, key *ecdsa.PrivateKey, serial int64, notAfter time.Time) []byte {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// valuesFor returns the set of sample values (NotAfter timestamps) carried by
// the given exposition lines.
func valuesFor(t *testing.T, samples []string) map[int64]bool {
	t.Helper()

	values := map[int64]bool{}
	for _, s := range samples {
		fields := strings.Fields(s)
		v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			t.Fatalf("cannot parse sample value in %q: %v", s, err)
		}
		values[int64(v)] = true
	}

	return values
}

func TestCollectPath_SingleCert(t *testing.T) {
	fs := afero.NewMemMapFs()
	certPEM := generateSelfSignedCertPEM(t, time.Now().Add(24*time.Hour))

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/tls.crt", certPEM, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	ch := make(chan prometheus.Metric, 10)
	err := e.collectPath(ch, "/certs")
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
}

func TestCollectPath_MultipleCertsInSameFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	cert1 := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	cert2 := generateSelfSignedCertPEM(t, time.Now().Add(48*time.Hour))
	combined := append(cert1, cert2...)

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/tls.crt", combined, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	ch := make(chan prometheus.Metric, 10)
	err := e.collectPath(ch, "/certs")
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics for concatenated certs, got %d", len(metrics))
	}
}

// TestGather_MultipleCertsInSameFile guards against the regression where two
// certs concatenated in a single file produced metrics with identical label
// sets, causing Gather() to fail and blanking out the whole scrape.
func TestGather_MultipleCertsInSameFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	cert1 := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	cert2 := generateSelfSignedCertPEM(t, time.Now().Add(48*time.Hour))
	combined := append(cert1, cert2...)

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/tls.crt", combined, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	reg := prometheus.NewRegistry()
	if err := reg.Register(e); err != nil {
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

func samplesFor(body, path string) []string {
	var samples []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, metricName) && strings.Contains(line, `path="`+path+`"`) {
			samples = append(samples, line)
		}
	}

	return samples
}

// TestScrape_DuplicateIdenticalCertInSameFile covers a file holding the *same*
// certificate twice, which neither the serialnumber nor the fingerprint label
// can tell apart. The scrape must still succeed and report the certificate once.
func TestScrape_DuplicateIdenticalCertInSameFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	certPEM := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))
	duplicated := append(append([]byte{}, certPEM...), certPEM...)

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/tls.crt", duplicated, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	reg := prometheus.NewRegistry()
	if err := reg.Register(e); err != nil {
		t.Fatal(err)
	}

	// A repeated identical certificate is expected and must not be reported
	// as a gather error, otherwise ErrorLog fires on every scrape for a
	// benign file and buries real collisions.
	if _, err := reg.Gather(); err != nil {
		t.Fatalf("a repeated identical certificate must be deduplicated, not collide: %v", err)
	}

	code, body := serveMetrics(t, reg)
	if code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200, got %d", code)
	}

	if got := len(samplesFor(body, "/certs/tls.crt")); got != 1 {
		t.Fatalf("expected the duplicated certificate to be served once, got %d samples", got)
	}
}

// TestGather_SameCertInTwoFiles makes sure the same certificate on two paths
// still yields two series, because the path label keeps them distinct.
func TestGather_SameCertInTwoFiles(t *testing.T) {
	fs := afero.NewMemMapFs()

	certPEM := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/tls.crt", certPEM, 0644)
	_ = afero.WriteFile(fs, "/certs/ca.crt", certPEM, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	reg := prometheus.NewRegistry()
	if err := reg.Register(e); err != nil {
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
		t.Fatalf("expected 1 series per file, got %d", series)
	}
}

// TestScrape_DuplicateDoesNotHideOtherCerts is the core regression guard: a file
// with duplicated certificates must not take the metrics of the other files down
// with it. Losing those was the customer visible symptom, and it is what
// ContinueOnError prevents.
func TestScrape_DuplicateDoesNotHideOtherCerts(t *testing.T) {
	fs := afero.NewMemMapFs()

	healthy := generateSelfSignedCertPEM(t, time.Now().Add(72*time.Hour))
	broken := generateSelfSignedCertPEM(t, time.Now().Add(1*time.Hour))

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/healthy.crt", healthy, 0644)
	_ = afero.WriteFile(fs, "/certs/duplicated.crt", append(append([]byte{}, broken...), broken...), 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	reg := prometheus.NewRegistry()
	if err := reg.Register(e); err != nil {
		t.Fatal(err)
	}

	code, body := serveMetrics(t, reg)
	if code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200 despite a duplicate series, got %d", code)
	}

	if got := len(samplesFor(body, "/certs/healthy.crt")); got != 1 {
		t.Fatalf("expected the unrelated certificate to still be served, got %d samples", got)
	}
}

func TestCollectPath_PrivateKeySkipped(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/certs", 0755)
	// Contains the substring "RSA PRIVATE KEY" which is what fileIsPrivateKey checks.
	// Avoiding the full PEM header to not trigger gitleaks false positives.
	_ = afero.WriteFile(fs, "/certs/tls.key", []byte("not a real cert, just contains RSA PRIVATE KEY marker"), 0644)
	_ = afero.WriteFile(fs, "/certs/tls.crt", generateSelfSignedCertPEM(t, time.Now().Add(24*time.Hour)), 0644)

	e := newTestExporter(t, fs, []string{"/certs"})

	ch := make(chan prometheus.Metric, 10)
	err := e.collectPath(ch, "/certs")
	if err != nil {
		t.Fatal(err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// Only tls.crt should produce a metric, not the private key.
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric (private key skipped), got %d", len(metrics))
	}
}

// permDeniedFs makes a single path unreadable. MemMapFs does not enforce
// permission bits, so a root-only file on a node has to be simulated.
type permDeniedFs struct {
	afero.Fs
	denied string
}

func (fs permDeniedFs) Open(name string) (afero.File, error) {
	if name == fs.denied {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrPermission}
	}
	return fs.Fs.Open(name)
}

func TestCollectPath_UnreadableFileDoesNotAbortWalk(t *testing.T) {
	base := afero.NewMemMapFs()

	// The walk is lexical, so an unreadable ca.crt is reached before
	// server.crt and used to abort collection before server.crt was read.
	_ = base.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(base, "/certs/ca.crt", []byte("root only"), 0600)
	_ = afero.WriteFile(base, "/certs/server.crt", generateSelfSignedCertPEM(t, time.Now().Add(24*time.Hour)), 0644)

	e := newTestExporter(t, permDeniedFs{Fs: base, denied: "/certs/ca.crt"}, []string{"/certs"})

	ch := make(chan prometheus.Metric, 10)
	err := e.collectPath(ch, "/certs")
	if err != nil {
		t.Fatalf("an unreadable file must not fail collection: %v", err)
	}
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected server.crt to still be collected past the unreadable ca.crt, got %d metrics", len(metrics))
	}
}

// TestScrape_SameSerialDifferentCertsInSameFile tests that two certs with the same serial value
// are still exposed as distinct series.
func TestScrape_SameSerialDifferentCertsInSameFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	renewed := time.Now().Add(300 * 24 * time.Hour).Truncate(time.Second)
	combined := append(generateCertPEM(t, key, 0, expired), generateCertPEM(t, key, 0, renewed)...)

	_ = fs.MkdirAll("/certs", 0755)
	_ = afero.WriteFile(fs, "/certs/kyverno-tls-ca.crt", combined, 0644)

	e := newTestExporter(t, fs, []string{"/certs"})
	reg := prometheus.NewRegistry()
	if err := reg.Register(e); err != nil {
		t.Fatal(err)
	}

	code, body := serveMetrics(t, reg)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	samples := samplesFor(body, "/certs/kyverno-tls-ca.crt")
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples for two certs sharing serial 0, got %d: %v", len(samples), samples)
	}
	got := valuesFor(t, samples)
	if !got[expired.Unix()] || !got[renewed.Unix()] {
		t.Fatalf("expected both NotAfter values %d and %d, got %v", expired.Unix(), renewed.Unix(), got)
	}
	for _, s := range samples {
		fp := strings.Split(strings.Split(s, `fingerprint="`)[1], `"`)[0]
		if len(fp) != 16 || strings.ToLower(fp) != fp {
			t.Fatalf("fingerprint must be 16 lowercase hex chars, got %q", fp)
		}
	}
}
