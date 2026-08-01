package authclient

import (
	"errors"
	"net/url"
	"strings"

	"github.com/zalando/go-keyring"
)

const serviceName = "rtest"

type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type SystemKeyring struct{}

func (SystemKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (SystemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (SystemKeyring) Delete(service, user string) error { return keyring.Delete(service, user) }

func StoreToken(store Keyring, serviceURL, token string) error {
	if token == "" {
		return errors.New("token is required")
	}
	account, err := account(serviceURL)
	if err != nil {
		return err
	}
	return store.Set(serviceName, account, token)
}

func StoredToken(store Keyring, serviceURL string) (string, error) {
	account, err := account(serviceURL)
	if err != nil {
		return "", err
	}
	return store.Get(serviceName, account)
}

func DeleteToken(store Keyring, serviceURL string) error {
	account, err := account(serviceURL)
	if err != nil {
		return err
	}
	return store.Delete(serviceName, account)
}

func account(serviceURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(serviceURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("rtest service URL must be absolute")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
