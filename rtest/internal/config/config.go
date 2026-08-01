package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SSH struct {
	Host          string `json:"host"`
	User          string `json:"user,omitempty"`
	IdentityFile  string `json:"identity_file"`
	RemoteAddress string `json:"remote_address,omitempty"`
}

type Backend string

const (
	BackendLegacy  Backend = "legacy"
	BackendREAPI   Backend = "reapi"
	BackendSwarm   Backend = "swarm"
	BackendService Backend = "service"
)

type REAPI struct {
	Service       string `json:"service,omitempty"`
	Instance      string `json:"instance,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"`
}

type BuildKit struct {
	Address       string `json:"address,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"`
}

type CAS struct {
	Service       string `json:"service,omitempty"`
	Instance      string `json:"instance,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"`
	JobAddress    string `json:"job_address,omitempty"`
}

type Swarm struct {
	DockerHost string `json:"docker_host,omitempty"`
	JobsRoot   string `json:"jobs_root,omitempty"`
	Image      string `json:"image,omitempty"`
	CPUs       string `json:"cpus,omitempty"`
	Memory     string `json:"memory,omitempty"`
}

type Service struct {
	Project      string `json:"project,omitempty"`
	Image        string `json:"image,omitempty"`
	CPUs         string `json:"cpus,omitempty"`
	Memory       string `json:"memory,omitempty"`
	CACertFile   string `json:"ca_cert_file,omitempty"`
	OIDCAudience string `json:"oidc_audience,omitempty"`
}

type Config struct {
	Backend  Backend   `json:"backend,omitempty"`
	URL      string    `json:"url,omitempty"`
	Token    string    `json:"token,omitempty"`
	SSH      *SSH      `json:"ssh,omitempty"`
	REAPI    *REAPI    `json:"reapi,omitempty"`
	CAS      *CAS      `json:"cas,omitempty"`
	Swarm    *Swarm    `json:"swarm,omitempty"`
	BuildKit *BuildKit `json:"buildkit,omitempty"`
	Service  *Service  `json:"service,omitempty"`
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	config, fileErr := read(path)
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return Config{}, fileErr
	}
	if value := os.Getenv("RTEST_URL"); value != "" {
		config.URL = value
	}
	if value := os.Getenv("RTEST_TOKEN"); value != "" {
		config.Token = value
	}
	if config.Backend == "" {
		if config.REAPI != nil {
			config.Backend = BackendREAPI
		} else {
			config.Backend = BackendLegacy
		}
	}
	if config.Backend != BackendLegacy && config.Backend != BackendREAPI && config.Backend != BackendSwarm && config.Backend != BackendService {
		return Config{}, fmt.Errorf("unsupported backend %q in %s", config.Backend, path)
	}
	if config.SSH != nil {
		if config.SSH.User == "" {
			config.SSH.User = "root"
		}
		if config.SSH.RemoteAddress == "" && config.Backend == BackendLegacy {
			config.SSH.RemoteAddress = "127.0.0.1:8080"
		}
	}
	if config.Backend == BackendLegacy {
		if config.Token == "" {
			return Config{}, fmt.Errorf("RTEST_TOKEN is required; configure %s or set the environment variable", path)
		}
		if config.URL == "" && (config.SSH == nil || config.SSH.Host == "") {
			return Config{}, fmt.Errorf("either url or ssh.host is required in %s", path)
		}
		return config, nil
	}
	if config.Backend == BackendService {
		if config.URL == "" {
			return Config{}, fmt.Errorf("url is required for service backend in %s", path)
		}
		if config.Service == nil {
			config.Service = &Service{}
		}
		if config.Service.Project != "" {
			return Config{}, fmt.Errorf("service.project is no longer a global default; remove it and run rtest init to create rtest.json")
		}
		if config.Service.CPUs == "" {
			config.Service.CPUs = "2"
		}
		if config.Service.Memory == "" {
			config.Service.Memory = "4g"
		}
		if config.Service.OIDCAudience == "" {
			config.Service.OIDCAudience = config.URL
		}
		return config, nil
	}
	if config.Backend == BackendSwarm {
		if config.CAS == nil {
			return Config{}, fmt.Errorf("cas configuration is required in %s", path)
		}
		if config.Swarm == nil {
			return Config{}, fmt.Errorf("swarm configuration is required in %s", path)
		}
		if config.CAS.Instance == "" {
			config.CAS.Instance = "rtest"
		}
		if config.CAS.RemoteAddress == "" {
			config.CAS.RemoteAddress = "127.0.0.1:50051"
		}
		if config.CAS.JobAddress == "" {
			config.CAS.JobAddress = config.CAS.RemoteAddress
		}
		if config.CAS.Service == "" && (config.SSH == nil || config.SSH.Host == "") {
			return Config{}, fmt.Errorf("either cas.service or ssh.host is required in %s", path)
		}
		if config.Swarm.DockerHost == "" && config.SSH == nil {
			config.Swarm.DockerHost = "unix:///var/run/docker.sock"
		}
		if config.Swarm.JobsRoot == "" {
			config.Swarm.JobsRoot = "/var/lib/rtest/jobs"
		}
		if config.Swarm.Image == "" {
			config.Swarm.Image = "rtest-runner-standard:local"
		}
		if config.Swarm.CPUs == "" {
			config.Swarm.CPUs = "1.5"
		}
		if config.Swarm.Memory == "" {
			config.Swarm.Memory = "2500m"
		}
		if config.BuildKit != nil && config.BuildKit.RemoteAddress == "" {
			config.BuildKit.RemoteAddress = "127.0.0.1:1234"
		}
		return config, nil
	}
	if config.REAPI == nil {
		return Config{}, fmt.Errorf("reapi configuration is required in %s", path)
	}
	if config.REAPI.Instance == "" {
		config.REAPI.Instance = "rtest"
	}
	if config.REAPI.RemoteAddress == "" {
		config.REAPI.RemoteAddress = "127.0.0.1:50051"
	}
	if config.REAPI.Service == "" && (config.SSH == nil || config.SSH.Host == "") {
		return Config{}, fmt.Errorf("either reapi.service or ssh.host is required in %s", path)
	}
	if config.BuildKit != nil && config.BuildKit.RemoteAddress == "" {
		config.BuildKit.RemoteAddress = "127.0.0.1:1234"
	}
	return config, nil
}

func Path() (string, error) {
	if value := os.Getenv("RTEST_CONFIG"); value != "" {
		return value, nil
	}
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "rtest", "config.json"), nil
}

func read(path string) (Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Config{}, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Config{}, fmt.Errorf("%s must not be accessible by group or other users", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	return config, nil
}
