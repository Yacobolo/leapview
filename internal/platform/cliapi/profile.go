package cliapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	securefs "github.com/flidai/leapview/internal/platform/filesystem"
	instancelock "github.com/flidai/leapview/internal/platform/locking"
)

const profileDocumentVersion = 1

var ErrProfileNotFound = errors.New("target profile not found")

// TargetProfile contains target identity and a reference to a native-store
// account. Secret material is never part of this document.
type TargetProfile struct {
	Origin            string `json:"origin"`
	InstanceID        string `json:"instanceId"`
	Environment       string `json:"environment,omitempty"`
	ProjectID         string `json:"projectId"`
	CredentialAccount string `json:"credentialAccount"`
}

type NamedTargetProfile struct {
	Name    string
	Profile TargetProfile
}

type profileDocument struct {
	Version int                      `json:"version"`
	Targets map[string]TargetProfile `json:"targets"`
}

// ProfileStore persists non-secret CLI target metadata in a versioned document.
type ProfileStore struct {
	path string
	mu   sync.Mutex
}

func NewProfileStore(path string) *ProfileStore {
	return &ProfileStore{path: path}
}

func (store *ProfileStore) Get(name string) (TargetProfile, error) {
	document, err := store.load()
	if err != nil {
		return TargetProfile{}, err
	}
	profile, ok := document.Targets[strings.TrimSpace(name)]
	if !ok {
		return TargetProfile{}, ErrProfileNotFound
	}
	return profile, nil
}

func (store *ProfileStore) FindByOrigin(origin string) (string, TargetProfile, error) {
	profiles, err := store.ProfilesByOrigin(origin)
	if err != nil {
		return "", TargetProfile{}, err
	}
	if len(profiles) == 0 {
		return "", TargetProfile{}, ErrProfileNotFound
	}
	if len(profiles) != 1 {
		return "", TargetProfile{}, fmt.Errorf("multiple target profiles use origin %q", origin)
	}
	return profiles[0].Name, profiles[0].Profile, nil
}

func (store *ProfileStore) ProfilesByOrigin(origin string) ([]NamedTargetProfile, error) {
	canonical, err := canonicalTargetOrigin(origin)
	if err != nil {
		return nil, err
	}
	document, err := store.load()
	if err != nil {
		return nil, err
	}
	profiles := make([]NamedTargetProfile, 0)
	for name, profile := range document.Targets {
		if profile.Origin == canonical {
			profiles = append(profiles, NamedTargetProfile{Name: name, Profile: profile})
		}
	}
	return profiles, nil
}

func (store *ProfileStore) Put(name string, profile TargetProfile) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("target profile name is required")
	}
	canonical, err := canonicalTargetOrigin(profile.Origin)
	if err != nil {
		return err
	}
	profile.Origin = canonical
	if strings.TrimSpace(profile.InstanceID) == "" {
		return fmt.Errorf("target instance identity is required")
	}
	if strings.TrimSpace(profile.ProjectID) == "" {
		return fmt.Errorf("target project is required")
	}
	if strings.TrimSpace(profile.CredentialAccount) == "" {
		return fmt.Errorf("target credential account is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	lock, err := store.acquireMutationLock()
	if err != nil {
		return err
	}
	defer lock.Release()
	document, err := store.load()
	if err != nil {
		return err
	}
	if current, ok := document.Targets[name]; ok {
		if current.InstanceID != profile.InstanceID {
			return fmt.Errorf("target profile %q instance identity changed from %q to %q", name, current.InstanceID, profile.InstanceID)
		}
		if current.Origin != profile.Origin || current.ProjectID != profile.ProjectID {
			return fmt.Errorf("target profile %q origin or project changed; delete it before replacement", name)
		}
	}
	document.Targets[name] = profile
	return store.save(document)
}

func (store *ProfileStore) Delete(name string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	lock, err := store.acquireMutationLock()
	if err != nil {
		return err
	}
	defer lock.Release()
	document, err := store.load()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if _, ok := document.Targets[name]; !ok {
		return ErrProfileNotFound
	}
	delete(document.Targets, name)
	return store.save(document)
}

func (store *ProfileStore) load() (profileDocument, error) {
	document := profileDocument{Version: profileDocumentVersion, Targets: map[string]TargetProfile{}}
	content, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return document, nil
	}
	if err != nil {
		return profileDocument{}, fmt.Errorf("read target profiles: %w", err)
	}
	var raw any
	if err := json.Unmarshal(content, &raw); err != nil {
		return profileDocument{}, fmt.Errorf("decode target profiles: %w", err)
	}
	if field := secretBearingField(raw); field != "" {
		return profileDocument{}, fmt.Errorf("target profile contains forbidden secret-bearing field %q; remove it and log in again", field)
	}
	if err := json.Unmarshal(content, &document); err != nil {
		return profileDocument{}, fmt.Errorf("decode target profiles: %w", err)
	}
	if document.Version != profileDocumentVersion {
		return profileDocument{}, fmt.Errorf("unsupported target profile version %d", document.Version)
	}
	if document.Targets == nil {
		document.Targets = map[string]TargetProfile{}
	}
	for name, profile := range document.Targets {
		canonical, err := canonicalTargetOrigin(profile.Origin)
		if err != nil {
			return profileDocument{}, fmt.Errorf("target profile %q: %w", name, err)
		}
		profile.Origin = canonical
		document.Targets[name] = profile
	}
	return document, nil
}

func (store *ProfileStore) save(document profileDocument) error {
	if strings.TrimSpace(store.path) == "" {
		return fmt.Errorf("target profile path is required")
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode target profiles: %w", err)
	}
	if err := securefs.WritePrivateFileAtomic(store.path, content); err != nil {
		return fmt.Errorf("write target profiles: %w", err)
	}
	return nil
}

func (store *ProfileStore) acquireMutationLock() (*instancelock.Lock, error) {
	if strings.TrimSpace(store.path) == "" {
		return nil, fmt.Errorf("target profile path is required")
	}
	return instancelock.AcquireNamed(
		filepath.Dir(store.path),
		"."+filepath.Base(store.path)+".lock",
	)
}

func canonicalTargetOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("target origin must be an absolute URL")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("target origin must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("target origin must not contain a path, query, or fragment")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", fmt.Errorf("target origin must use HTTPS (HTTP is allowed only for loopback development)")
	}
	parsed.Path = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	return net.ParseIP(host).IsLoopback()
}

func secretBearingField(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			switch normalized {
			case "token", "apitoken", "accesstoken", "refreshtoken", "password", "secret", "clientsecret":
				return key
			}
			if field := secretBearingField(nested); field != "" {
				return field
			}
		}
	case []any:
		for _, nested := range typed {
			if field := secretBearingField(nested); field != "" {
				return field
			}
		}
	}
	return ""
}
