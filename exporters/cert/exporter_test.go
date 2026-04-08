package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/afero"
)

func newTestExporter(t *testing.T, fs afero.Fs, paths []string) *Exporter {
	t.Helper()

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return &Exporter{
		cert: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{"path"},
			nil,
		),
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

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
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
