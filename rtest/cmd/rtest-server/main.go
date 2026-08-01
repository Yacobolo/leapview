package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/control/controlapi"
	"github.com/flidai/leapview/rtest/internal/control/githuboidc"
	"github.com/flidai/leapview/rtest/internal/control/mtlsproxy"
	"github.com/flidai/leapview/rtest/internal/control/pki"
	"github.com/flidai/leapview/rtest/internal/control/reconciler"
	"github.com/flidai/leapview/rtest/internal/control/recovery"
	"github.com/flidai/leapview/rtest/internal/control/secret"
	controlsqlite "github.com/flidai/leapview/rtest/internal/control/sqlite"
	"github.com/flidai/leapview/rtest/internal/control/swarmscheduler"
	"github.com/flidai/leapview/rtest/internal/swarm"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap":
			bootstrap(os.Args[2:])
		case "backup":
			backupState(os.Args[2:])
		case "restore":
			restoreState(os.Args[2:])
		default:
			log.Fatalf("unknown command %q", os.Args[1])
		}
		return
	}
	serve()
}

func serve() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dataDir := env("RTEST_DATA_DIR", "/var/lib/rtest")
	store := openStore(dataDir)
	defer store.Close()
	initialized, err := store.Initialized(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !initialized {
		log.Fatal("rtest is not bootstrapped; run rtest-server bootstrap before starting the service")
	}
	names := splitNames(env("RTEST_SERVER_NAMES", "localhost,127.0.0.1"))
	pkiDir := env("RTEST_PKI_DIR", filepath.Join(dataDir, "pki"))
	authority, err := pki.Ensure(pkiDir, names)
	if err != nil {
		log.Fatal(err)
	}
	docker := swarm.New(swarm.Config{Binary: os.Getenv("RTEST_DOCKER"), Host: env("RTEST_DOCKER_HOST", "unix:///var/run/docker.sock")})
	if err := docker.Check(ctx); err != nil {
		log.Fatal(err)
	}
	casInternal := env("RTEST_CAS_INTERNAL", "127.0.0.1:50051")
	casListen := env("RTEST_CAS_LISTEN", ":50052")
	buildKitInternal := env("RTEST_BUILDKIT_INTERNAL", "127.0.0.1:1234")
	buildKitListen := env("RTEST_BUILDKIT_LISTEN", ":1235")
	casInstance := env("RTEST_CAS_INSTANCE", "rtest")
	serverName := names[0]
	scheduler := swarmscheduler.New(swarmscheduler.Config{
		Client: docker, CASAddress: casInternal, CASInstance: casInstance, JobsRoot: env("RTEST_JOBS_ROOT", "/var/lib/rtest/jobs"),
		EntrypointHostPath: env("RTEST_JOB_ENTRYPOINT", "/usr/local/lib/rtest/rtest-job-entrypoint"),
		CacheRoot:          env("RTEST_CACHE_ROOT", "/var/lib/rtest/cache"),
	})
	reconcile := reconciler.New(reconciler.Config{
		Store: store, Scheduler: scheduler,
		ServiceRetention: durationEnv("RTEST_SERVICE_RETENTION", time.Hour),
	})
	if err := reconcile.RunOnce(ctx); err != nil {
		log.Printf("initial reconciliation: %v", err)
	}
	go runReconciler(ctx, reconcile, durationEnv("RTEST_RECONCILE_INTERVAL", 30*time.Second))
	var verifier controlapi.OIDCVerifier
	if audience := os.Getenv("RTEST_GITHUB_OIDC_AUDIENCE"); audience != "" {
		verifier, err = githuboidc.New(ctx, env("RTEST_GITHUB_OIDC_ISSUER", githuboidc.Issuer), audience)
		if err != nil {
			log.Fatal(err)
		}
	}
	handler, err := controlapi.New(controlapi.Config{
		Store: store, Scheduler: scheduler, Authority: authority, OIDCVerifier: verifier,
		CASEndpoint: env("RTEST_CAS_ENDPOINT", endpoint(serverName, casListen)), CASInstance: casInstance,
		BuildKitEndpoint:    env("RTEST_BUILDKIT_ENDPOINT", endpoint(serverName, buildKitListen)),
		CredentialTTL:       durationEnv("RTEST_CREDENTIAL_TTL", 15*time.Minute),
		AllowUnpinnedImages: os.Getenv("RTEST_ALLOW_UNPINNED_IMAGES") == "1",
	})
	if err != nil {
		log.Fatal(err)
	}
	errorsChannel := make(chan error, 3)
	active := func(kind pki.Operation, id string) bool {
		return store.OperationActive(context.Background(), string(kind), id)
	}
	go func() {
		errorsChannel <- mtlsproxy.ListenAndServe(ctx, casListen, casInternal, authority.ServerTLSConfig(pki.OperationJob, active))
	}()
	go func() {
		errorsChannel <- mtlsproxy.ListenAndServe(ctx, buildKitListen, buildKitInternal, authority.ServerTLSConfig(pki.OperationBuild, active))
	}()
	server := &http.Server{
		Addr: env("RTEST_LISTEN", ":8443"), Handler: handler, ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout: 2 * time.Minute, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13},
	}
	go func() {
		errorsChannel <- server.ListenAndServeTLS(filepath.Join(pkiDir, "server.pem"), filepath.Join(pkiDir, "server-key.pem"))
	}()
	log.Printf("rtest control plane listening on %s; CAS mTLS on %s; BuildKit mTLS on %s", server.Addr, casListen, buildKitListen)
	select {
	case err := <-errorsChannel:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

type reconciliationRunner interface {
	RunOnce(context.Context) error
}

func runReconciler(ctx context.Context, runner reconciliationRunner, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runner.RunOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("reconciliation: %v", err)
			}
		}
	}
}

func bootstrap(args []string) {
	flags := flag.NewFlagSet("rtest-server bootstrap", flag.ExitOnError)
	dataDir := flags.String("data-dir", env("RTEST_DATA_DIR", "/var/lib/rtest"), "persistent rtest data directory")
	user := flags.String("user", "owner", "initial administrator name")
	project := flags.String("project", "default", "initial project slug")
	projectName := flags.String("project-name", "", "initial project display name")
	tokenName := flags.String("token-name", "bootstrap-device", "initial device token name")
	_ = flags.Parse(args)
	store := openStore(*dataDir)
	defer store.Close()
	result, err := store.Bootstrap(context.Background(), control.Bootstrap{
		UserName: *user, ProjectSlug: *project, ProjectName: *projectName, TokenName: *tokenName,
	})
	if err != nil {
		if errors.Is(err, control.ErrAlreadyExists) {
			log.Fatal("rtest is already bootstrapped; create another device token through the authenticated API")
		}
		log.Fatal(err)
	}
	fmt.Printf("Project: %s (%s)\nUser: %s (%s)\nToken: %s\n", result.Project.Slug, result.Project.ID, result.User.Name, result.User.ID, result.Token)
}

func backupState(args []string) {
	flags := flag.NewFlagSet("rtest-server backup", flag.ExitOnError)
	dataDir := flags.String("data-dir", env("RTEST_DATA_DIR", "/var/lib/rtest"), "persistent rtest data directory")
	output := flags.String("output", "", "new private backup directory")
	_ = flags.Parse(args)
	if *output == "" {
		log.Fatal("--output is required")
	}
	store := openStore(*dataDir)
	defer store.Close()
	if err := recovery.Create(context.Background(), store, *dataDir, *output); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Backup: %s\n", *output)
}

func restoreState(args []string) {
	flags := flag.NewFlagSet("rtest-server restore", flag.ExitOnError)
	input := flags.String("input", "", "validated rtest backup directory")
	dataDir := flags.String("data-dir", env("RTEST_DATA_DIR", "/var/lib/rtest"), "new persistent rtest data directory")
	_ = flags.Parse(args)
	if *input == "" {
		log.Fatal("--input is required")
	}
	if err := recovery.Restore(*input, *dataDir); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Restored: %s\n", *dataDir)
}

func openStore(dataDir string) *controlsqlite.Store {
	pepper, err := secret.Ensure(filepath.Join(dataDir, "control", "token-pepper"), 32)
	if err != nil {
		log.Fatal(err)
	}
	store, err := controlsqlite.Open(filepath.Join(dataDir, "control"), pepper)
	if err != nil {
		log.Fatal(err)
	}
	return store
}

func splitNames(value string) []string {
	var names []string
	for _, item := range strings.Split(value, ",") {
		if name := strings.TrimSpace(item); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		log.Fatal("RTEST_SERVER_NAMES must contain at least one DNS name or IP address")
	}
	return names
}

func endpoint(serverName, listen string) string {
	_, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return listen
	}
	return net.JoinHostPort(serverName, port)
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	return parsed
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
