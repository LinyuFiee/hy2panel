package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHY2CertificateChecksDomainAndDates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"*.example.test"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(30 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fullchain.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		domain          string
		at              time.Time
		status, message string
	}{
		{"hy2.example.test", now, "pass", "匹配"},
		{"other.test", now, "fail", "不匹配"},
		{"hy2.example.test", now.Add(-2 * time.Hour), "fail", "尚未生效"},
		{"hy2.example.test", leaf.NotAfter, "fail", "过期"},
		{"hy2.example.test", leaf.NotAfter.Add(-24 * time.Hour), "warn", "到期"},
	} {
		got := checkHY2Certificate(path, tt.domain, tt.at)
		if got.Status != tt.status || !strings.Contains(got.Message, tt.message) {
			t.Errorf("%s at %s: got %s %q", tt.domain, tt.at, got.Status, got.Message)
		}
	}
}
