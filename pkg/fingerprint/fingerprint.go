// Package fingerprint derives a short, stable identifier for an X.509
// certificate from its DER encoding.
//
// The value is the first Length lowercase hex characters of the SHA-256
// digest over the raw certificate, i.e. the prefix of
// `openssl x509 -fingerprint -sha256` with the colons removed and lowercased.
// Unlike the serial number, the Subject Key Identifier or the public key, it
// differs for any two certificates that differ in a single byte, which is what
// tells a renewed Kyverno CA (same key, serial 0) apart from the expired one
// it replaces.
package fingerprint

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
)

// Length is the number of hex characters kept from the SHA-256 digest.
const Length = 16

// SHA256 returns the first Length lowercase hex characters of the SHA-256
// digest of the certificate's DER encoding.
func SHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])[:Length]
}
