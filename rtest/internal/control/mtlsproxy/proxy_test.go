package mtlsproxy_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/control/mtlsproxy"
	"github.com/flidai/leapview/rtest/internal/control/pki"
)

func TestProxyPreservesTCPProtocolAndChecksOperationIdentity(t *testing.T) {
	target := echoServer(t)
	authority, err := pki.Ensure(filepath.Join(t.TempDir(), "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = mtlsproxy.Serve(ctx, listener, target, authority.ServerTLSConfig(pki.OperationJob, func(kind pki.Operation, id string) bool {
			return kind == pki.OperationJob && id == "job-1"
		}))
	}()
	job, _ := authority.Issue(pki.OperationJob, "job-1", time.Minute)
	connection, err := dial(listener.Addr().String(), job)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("standard grpc bytes")); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len("standard grpc bytes"))
	if _, err := io.ReadFull(connection, buffer); err != nil {
		t.Fatal(err)
	}
	if string(buffer) != "standard grpc bytes" {
		t.Fatalf("echo = %q", buffer)
	}

	build, _ := authority.Issue(pki.OperationBuild, "build-1", time.Minute)
	if rejected, err := dial(listener.Addr().String(), build); err == nil {
		defer rejected.Close()
		if _, err := rejected.Write([]byte("should fail")); err == nil {
			buffer := make([]byte, 1)
			if _, err := rejected.Read(buffer); err == nil {
				t.Fatal("build certificate was accepted by job proxy")
			}
		}
	}
}

func echoServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer connection.Close()
				_, _ = io.Copy(connection, connection)
			}()
		}
	}()
	return listener.Addr().String()
}

func dial(address string, credential pki.Credential) (*tls.Conn, error) {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(credential.CAPEM)
	certificate, err := tls.X509KeyPair(credential.CertificatePEM, credential.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	return tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{certificate}, ServerName: credential.ServerName})
}
