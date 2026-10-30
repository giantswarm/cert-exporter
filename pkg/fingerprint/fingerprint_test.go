package fingerprint

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"math/big"
	"testing"
	"time"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

// newCert issues a self-signed certificate with the given key, serial and
// expiry, mirroring how Kyverno issues its CA: same key, serial 0.
func newCert(t *testing.T, key *ecdsa.PrivateKey, serial int64, notAfter time.Time) *x509.Certificate {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return cert
}

func TestSHA256_IsPrefixOfDERDigest(t *testing.T) {
	cert := newCert(t, newKey(t), 1, time.Now().Add(time.Hour))
	sum := sha256.Sum256(cert.Raw)
	want := hex.EncodeToString(sum[:])[:Length]
	if got := SHA256(cert); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(want) != 16 {
		t.Fatalf("Length must be 16, got %d", len(want))
	}
}

// Kyverno regenerates its CA with the same private key and SerialNumber 0; the
// fingerprint is the only label that can tell the old and new CA apart.
func TestSHA256_DistinguishesSameKeySameSerial(t *testing.T) {
	key := newKey(t)
	old := newCert(t, key, 0, time.Now().Add(time.Hour))
	renewed := newCert(t, key, 0, time.Now().Add(48*time.Hour))
	if SHA256(old) == SHA256(renewed) {
		t.Fatal("two different certificates with the same key and serial must not share a fingerprint")
	}
}

func TestSHA256_StableForIdenticalBytes(t *testing.T) {
	cert := newCert(t, newKey(t), 1, time.Now().Add(time.Hour))
	again, err := x509.ParseCertificate(cert.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if SHA256(cert) != SHA256(again) {
		t.Fatal("identical DER must produce the same fingerprint")
	}
}
