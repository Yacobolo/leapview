package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Operation string

const (
	OperationJob   Operation = "job"
	OperationBuild Operation = "build"
)

type Credential struct {
	CAPEM          []byte
	CertificatePEM []byte
	PrivateKeyPEM  []byte
	ServerName     string
	ExpiresAt      time.Time
}

type Authority struct {
	ca            *x509.Certificate
	caKey         *ecdsa.PrivateKey
	caPEM         []byte
	serverName    string
	serverKeyPair tls.Certificate
}

type ActiveOperation func(Operation, string) bool

func Ensure(root string, names []string) (*Authority, error) {
	if len(names) == 0 || strings.TrimSpace(names[0]) == "" {
		return nil, errors.New("at least one PKI server name is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create PKI directory: %w", err)
	}
	caCertPath, caKeyPath := filepath.Join(root, "ca.pem"), filepath.Join(root, "ca-key.pem")
	serverCertPath, serverKeyPath := filepath.Join(root, "server.pem"), filepath.Join(root, "server-key.pem")
	if _, err := os.Stat(caCertPath); errors.Is(err, os.ErrNotExist) {
		if err := createAuthority(caCertPath, caKeyPath); err != nil {
			return nil, err
		}
	}
	ca, caKey, caPEM, err := loadAuthority(caCertPath, caKeyPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(serverCertPath); errors.Is(err, os.ErrNotExist) {
		if err := createServerCertificate(ca, caKey, serverCertPath, serverKeyPath, names); err != nil {
			return nil, err
		}
	}
	serverPair, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load PKI server certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(serverPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse PKI server certificate: %w", err)
	}
	for _, name := range names {
		if err := leaf.VerifyHostname(name); err != nil {
			return nil, fmt.Errorf("server certificate does not cover %q: %w", name, err)
		}
	}
	serverPair.Leaf = leaf
	return &Authority{ca: ca, caKey: caKey, caPEM: caPEM, serverName: names[0], serverKeyPair: serverPair}, nil
}

func (a *Authority) Issue(kind Operation, id string, ttl time.Duration) (Credential, error) {
	if kind != OperationJob && kind != OperationBuild {
		return Credential{}, fmt.Errorf("unsupported operation kind %q", kind)
	}
	if strings.TrimSpace(id) == "" {
		return Credential{}, errors.New("operation ID is required")
	}
	if ttl < time.Minute || ttl > 2*time.Hour {
		return Credential{}, errors.New("credential lifetime must be between 1 minute and 2 hours")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Credential{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	serial, err := randomSerial()
	if err != nil {
		return Credential{}, err
	}
	identity, err := url.Parse("spiffe://rtest/" + string(kind) + "/" + url.PathEscape(id))
	if err != nil {
		return Credential{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: string(kind) + ":" + id, Organization: []string{"rtest"}},
		NotBefore:    now.Add(-time.Minute), NotAfter: expires,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:        []*url.URL{identity},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.ca, &key.PublicKey, a.caKey)
	if err != nil {
		return Credential{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Credential{}, err
	}
	return Credential{
		CAPEM:          append([]byte(nil), a.caPEM...),
		CertificatePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		PrivateKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		ServerName:     a.serverName,
		ExpiresAt:      expires,
	}, nil
}

func (a *Authority) ServerTLSConfig(expected Operation, active ActiveOperation) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(a.ca)
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
		Certificates: []tls.Certificate{a.serverKeyPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 || len(state.PeerCertificates[0].URIs) != 1 {
				return errors.New("exactly one rtest operation identity is required")
			}
			identity := state.PeerCertificates[0].URIs[0]
			parts := strings.Split(strings.TrimPrefix(identity.EscapedPath(), "/"), "/")
			if identity.Scheme != "spiffe" || identity.Host != "rtest" || len(parts) != 2 {
				return errors.New("invalid rtest operation identity")
			}
			id, err := url.PathUnescape(parts[1])
			if err != nil || Operation(parts[0]) != expected {
				return fmt.Errorf("operation certificate is not valid for %s", expected)
			}
			if active != nil && !active(expected, id) {
				return errors.New("operation credential is no longer active")
			}
			return nil
		},
	}
}

func createAuthority(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "rtest private CA", Organization: []string{"rtest"}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true, IsCA: true, MaxPathLen: 0,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writePEM(certPath, 0o644, "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(keyPath, 0o600, "PRIVATE KEY", keyDER)
}

func loadAuthority(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, nil, err
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, nil, errors.New("decode PKI CA certificate")
	}
	certificate, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, nil, err
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, nil, errors.New("decode PKI CA private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, nil, errors.New("PKI CA key is not ECDSA")
	}
	return certificate, key, certPEM, nil
}

func createServerCertificate(ca *x509.Certificate, caKey *ecdsa.PrivateKey, certPath, keyPath string, names []string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: names[0], Organization: []string{"rtest"}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(2, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, name := range names {
		if address := net.ParseIP(name); address != nil {
			template.IPAddresses = append(template.IPAddresses, address)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writePEM(certPath, 0o644, "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(keyPath, 0o600, "PRIVATE KEY", keyDER)
}

func writePEM(path string, mode os.FileMode, blockType string, data []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data}), mode)
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	return serial, nil
}
