package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/control/pki"
)

func TestAuthorityIssuesShortLivedOperationCertificates(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := authority.Issue(pki.OperationJob, "job-123", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if credential.ExpiresAt.Sub(time.Now()) > 5*time.Minute || credential.ServerName != "localhost" {
		t.Fatalf("credential = %#v", credential)
	}
	block, _ := pem.Decode(credential.CertificatePEM)
	if block == nil {
		t.Fatal("missing certificate PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	wantURI, _ := url.Parse("spiffe://rtest/job/job-123")
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != wantURI.String() {
		t.Fatalf("uris = %#v", certificate.URIs)
	}
	if _, err := tls.X509KeyPair(credential.CertificatePEM, credential.PrivateKeyPEM); err != nil {
		t.Fatalf("client key pair: %v", err)
	}
}

func TestServerTLSConfigNegotiatesHTTP2ForGRPCDataPlanes(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	configuration := authority.ServerTLSConfig(pki.OperationJob, nil)
	if len(configuration.NextProtos) != 1 || configuration.NextProtos[0] != "h2" {
		t.Fatalf("NextProtos = %#v, want h2", configuration.NextProtos)
	}
}

func TestServerTLSConfigRejectsWrongOperationKind(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	job, _ := authority.Issue(pki.OperationJob, "job-123", time.Minute)
	build, _ := authority.Issue(pki.OperationBuild, "build-123", time.Minute)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	tlsListener := tls.NewListener(listener, authority.ServerTLSConfig(pki.OperationJob, func(kind pki.Operation, id string) bool {
		return kind == pki.OperationJob && id == "job-123"
	}))
	done := make(chan error, 2)
	go func() {
		for i := 0; i < 2; i++ {
			connection, err := tlsListener.Accept()
			if err == nil {
				_, err = connection.Write([]byte("ok"))
				_ = connection.Close()
			}
			done <- err
		}
	}()
	if got := dial(t, listener.Addr().String(), job); got != "ok" {
		t.Fatalf("job response = %q", got)
	}
	if got := dial(t, listener.Addr().String(), build); got != "" {
		t.Fatalf("build certificate unexpectedly accepted: %q", got)
	}
	<-done
	<-done
}

func dial(t *testing.T, address string, credential pki.Credential) string {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(credential.CAPEM)
	certificate, err := tls.X509KeyPair(credential.CertificatePEM, credential.PrivateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := tls.Dial("tcp", address, &tls.Config{
		MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{certificate}, ServerName: credential.ServerName,
	})
	if err != nil {
		return ""
	}
	defer connection.Close()
	buffer := make([]byte, 2)
	n, _ := connection.Read(buffer)
	return string(buffer[:n])
}
