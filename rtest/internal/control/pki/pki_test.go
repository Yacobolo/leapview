package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/url"
	"path/filepath"
	"sync/atomic"
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

func TestServerTLSConfigRejectsWrongKindUnknownAndMissingCredentials(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	job, _ := authority.Issue(pki.OperationJob, "job-123", time.Minute)
	build, _ := authority.Issue(pki.OperationBuild, "build-123", time.Minute)
	unknown, _ := authority.Issue(pki.OperationJob, "job-unknown", time.Minute)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	tlsListener := tls.NewListener(listener, authority.ServerTLSConfig(pki.OperationJob, func(kind pki.Operation, id string) bool {
		return kind == pki.OperationJob && id == "job-123"
	}))
	done := make(chan error, 4)
	go func() {
		for i := 0; i < 4; i++ {
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
	if got := dial(t, listener.Addr().String(), unknown); got != "" {
		t.Fatalf("unknown job certificate unexpectedly accepted: %q", got)
	}
	if got := dialWithoutCredential(t, listener.Addr().String(), job); got != "" {
		t.Fatalf("connection without an operation certificate was accepted: %q", got)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestServerTLSConfigRejectsExpiredOperationCredential(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []pki.Operation{pki.OperationJob, pki.OperationBuild} {
		t.Run(string(kind), func(t *testing.T) {
			credential, err := authority.Issue(kind, string(kind)+"-expired", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			configuration := authority.ServerTLSConfig(kind, func(pki.Operation, string) bool { return true })
			configuration.Time = func() time.Time { return credential.ExpiresAt.Add(time.Second) }
			tlsListener := tls.NewListener(listener, configuration)
			done := make(chan struct{})
			go func() {
				connection, err := tlsListener.Accept()
				if err == nil {
					_, _ = connection.Write([]byte("ok"))
					_ = connection.Close()
				}
				close(done)
			}()
			if got := dial(t, listener.Addr().String(), credential); got != "" {
				t.Fatalf("expired %s certificate unexpectedly accepted: %q", kind, got)
			}
			<-done
		})
	}
}

func TestServerTLSConfigImmediatelyRejectsAnInactiveOperation(t *testing.T) {
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	job, _ := authority.Issue(pki.OperationJob, "job-1", time.Minute)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var active atomic.Bool
	active.Store(true)
	tlsListener := tls.NewListener(listener, authority.ServerTLSConfig(pki.OperationJob, func(_ pki.Operation, id string) bool {
		return id == "job-1" && active.Load()
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
		t.Fatalf("active operation response = %q", got)
	}
	active.Store(false)
	if got := dial(t, listener.Addr().String(), job); got != "" {
		t.Fatalf("inactive operation credential was accepted: %q", got)
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

func dialWithoutCredential(t *testing.T, address string, authority pki.Credential) string {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(authority.CAPEM)
	connection, err := tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, ServerName: authority.ServerName})
	if err != nil {
		return ""
	}
	defer connection.Close()
	buffer := make([]byte, 2)
	n, _ := connection.Read(buffer)
	return string(buffer[:n])
}
