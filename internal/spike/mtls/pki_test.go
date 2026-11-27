package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// All key material is generated at test time (ECDSA P-256, short validity) and
// lives in memory or in t.TempDir(). Nothing is committed.

const serverName = "hub.sakuraops.test"

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pool *x509.CertPool
}

func newCA(t testing.TB, name string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          randSerial(t),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &testCA{cert: cert, key: key, pool: pool}
}

type leafOpts struct {
	cn        string
	dns       []string
	eku       []x509.ExtKeyUsage
	notBefore time.Time
	notAfter  time.Time
}

type issued struct {
	tls    tls.Certificate
	leaf   *x509.Certificate
	serial string
	keyDER []byte
}

func (ca *testCA) issue(t testing.TB, o leafOpts) issued {
	t.Helper()
	if o.notBefore.IsZero() {
		o.notBefore = time.Now().Add(-time.Minute)
	}
	if o.notAfter.IsZero() {
		o.notAfter = time.Now().Add(time.Hour)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: randSerial(t),
		Subject:      pkix.Name{CommonName: o.cn},
		DNSNames:     o.dns,
		NotBefore:    o.notBefore,
		NotAfter:     o.notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  o.eku,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return issued{
		tls:    tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf},
		leaf:   leaf,
		serial: serialOf(leaf),
		keyDER: keyDER,
	}
}

func (ca *testCA) server(t testing.TB) issued {
	return ca.issue(t, leafOpts{cn: "hub", dns: []string{serverName}, eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
}

func (ca *testCA) client(t testing.TB, cn string) issued {
	return ca.issue(t, leafOpts{cn: cn, eku: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
}

func randSerial(t testing.TB) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	return n.Add(n, big.NewInt(1))
}

// writePEM stores the CA certificate and a client identity in dir (0600) for the
// helper-process test.
func writeClientFiles(t testing.TB, dir string, serverCA *testCA, c issued) (caPath, certPath, keyPath string) {
	t.Helper()
	caPath = filepath.Join(dir, "server-ca.pem")
	certPath = filepath.Join(dir, "client.pem")
	keyPath = filepath.Join(dir, "client-key.pem")
	write := func(p, typ string, der []byte) {
		b := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(caPath, "CERTIFICATE", serverCA.cert.Raw)
	write(certPath, "CERTIFICATE", c.leaf.Raw)
	write(keyPath, "PRIVATE KEY", c.keyDER)
	return
}
