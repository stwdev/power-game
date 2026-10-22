// Command genkey creates the signing key for the Android release APK using
// only the standard library:
//
//	go run ./cmd/genkey <output-dir>
//
// It writes two files, readable only by you:
//
//	signing-key.b64   the private key (PKCS#8, base64): the ANDROID_SIGNING_KEY secret
//	signing-cert.pem  the public certificate:           the ANDROID_SIGNING_CERT secret
//
// Android only installs updates signed with the same key, so keep the
// private key safe and never commit it.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./cmd/genkey <output-dir>")
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		log.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Power Game", Organization: []string{"Power Game"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(30, 0, 0), // app signing keys must outlive every update
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	pk8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		log.Fatal(err)
	}
	write(filepath.Join(dir, "signing-key.b64"), []byte(base64.StdEncoding.EncodeToString(pk8)+"\n"))
	write(filepath.Join(dir, "signing-cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	sum := sha256.Sum256(der)
	hexes := make([]string, len(sum))
	for i, b := range sum {
		hexes[i] = fmt.Sprintf("%02X", b)
	}
	fmt.Printf("Wrote %s and %s\n", filepath.Join(dir, "signing-key.b64"), filepath.Join(dir, "signing-cert.pem"))
	fmt.Printf("Certificate SHA-256: %s\n", strings.Join(hexes, ":"))
}

func write(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Fatal(err)
	}
}
