package mtlsproxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

func ListenAndServe(ctx context.Context, address, target string, config *tls.Config) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return Serve(ctx, listener, target, config)
}

func Serve(ctx context.Context, listener net.Listener, target string, config *tls.Config) error {
	if config == nil {
		_ = listener.Close()
		return errors.New("mTLS proxy configuration is required")
	}
	tlsListener := tls.NewListener(listener, config)
	var connections sync.WaitGroup
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = tlsListener.Close()
		case <-done:
		}
	}()
	defer close(done)
	defer connections.Wait()
	for {
		connection, err := tlsListener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		connections.Add(1)
		go func() {
			defer connections.Done()
			proxyConnection(ctx, connection, target)
		}()
	}
}

func proxyConnection(ctx context.Context, client net.Conn, target string) {
	defer client.Close()
	tlsClient, ok := client.(*tls.Conn)
	if !ok {
		return
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := tlsClient.HandshakeContext(handshakeCtx)
	cancel()
	if err != nil {
		return
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	upstream, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return
	}
	defer upstream.Close()
	copied := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, tlsClient); copied <- struct{}{} }()
	go func() { _, _ = io.Copy(tlsClient, upstream); copied <- struct{}{} }()
	select {
	case <-ctx.Done():
	case <-copied:
	}
}
