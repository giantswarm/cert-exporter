package secret

import (
	"context"
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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestExporter(t *testing.T) *Exporter {
	t.Helper()

	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return &Exporter{
		cert: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "secret", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{"name", "namespace", "secretkey", "certificatename"},
			nil,
		),
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
