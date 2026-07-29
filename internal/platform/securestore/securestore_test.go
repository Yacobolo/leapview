package securestore

import (
	"context"
	"errors"
	"testing"
)

type memoryBackend struct {
	values map[string]string
	err    error
}

func (backend *memoryBackend) Set(service, account, secret string) error {
	if backend.err != nil {
		return backend.err
	}
	if backend.values == nil {
		backend.values = map[string]string{}
	}
	backend.values[service+"\x00"+account] = secret
	return nil
}

func (backend *memoryBackend) Get(service, account string) (string, error) {
	if backend.err != nil {
		return "", backend.err
	}
	secret, ok := backend.values[service+"\x00"+account]
	if !ok {
		return "", errBackendNotFound
	}
	return secret, nil
}

func (backend *memoryBackend) Delete(service, account string) error {
	if backend.err != nil {
		return backend.err
	}
	key := service + "\x00" + account
	if _, ok := backend.values[key]; !ok {
		return errBackendNotFound
	}
	delete(backend.values, key)
	return nil
}

func TestNativeStoreIsolatesCredentialNamespaces(t *testing.T) {
	backend := &memoryBackend{}
	cli, err := newNative("com.leapview.cli", backend)
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := newNative("com.leapview.desktop", backend)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := cli.Set(ctx, "instance", "cli-secret"); err != nil {
		t.Fatal(err)
	}
	if err := desktop.Set(ctx, "instance", "desktop-secret"); err != nil {
		t.Fatal(err)
	}
	cliSecret, err := cli.Get(ctx, "instance")
	if err != nil {
		t.Fatal(err)
	}
	desktopSecret, err := desktop.Get(ctx, "instance")
	if err != nil {
		t.Fatal(err)
	}
	if cliSecret != "cli-secret" || desktopSecret != "desktop-secret" {
		t.Fatalf("credential namespaces leaked: cli=%q desktop=%q", cliSecret, desktopSecret)
	}
}

func TestNativeStoreFailsClosed(t *testing.T) {
	backendErr := errors.New("keychain locked")
	store, err := newNative("com.leapview.cli", &memoryBackend{err: backendErr})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "instance", "secret"); !errors.Is(err, backendErr) {
		t.Fatalf("Set error = %v, want keychain error", err)
	}
	if _, err := store.Get(context.Background(), "instance"); !errors.Is(err, backendErr) {
		t.Fatalf("Get error = %v, want keychain error", err)
	}
}

func TestNativeStoreMapsMissingCredential(t *testing.T) {
	store, err := newNative("com.leapview.cli", &memoryBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestNativeStoreRejectsInvalidNamesAndCancelledContext(t *testing.T) {
	if _, err := newNative("", &memoryBackend{}); err == nil {
		t.Fatal("newNative accepted an empty service")
	}
	store, err := newNative("com.leapview.cli", &memoryBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "", "secret"); err == nil {
		t.Fatal("Set accepted an empty account")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Set(ctx, "instance", "secret"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set error = %v, want context.Canceled", err)
	}
}
