package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Yacobolo/leapview/internal/app"
	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/Yacobolo/leapview/internal/platform/locking"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
	"github.com/spf13/cobra"
)

const defaultHTTPServerShutdownTimeout = 15 * time.Second

func serveCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the LeapView HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.environment = serveEnvironmentFlagValue(cmd.Flags().Changed("environment"), opts.environment)
			return runServe(ctx, opts)
		},
	}
	cmd.Flags().StringVar(&opts.addr, "addr", "", "listen address; defaults to the configured address")
	cmd.Flags().StringVar(&opts.environment, "environment", "", "instance environment; overrides LEAPVIEW_ENVIRONMENT, then defaults to prod in production and dev otherwise")
	cmd.Flags().BoolVar(&opts.production, "production", false, "serve active serving state from the platform DB")
	return cmd
}

func runServe(ctx context.Context, opts *rootOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	production := serveProductionMode(cfg, *opts)
	cfg.Production = production
	addr := opts.addr
	if addr == "" {
		addr = cfg.ListenAddr()
	}
	cfg.Addr = addr
	if err := cfg.Validate(config.ProfileServe); err != nil {
		return err
	}
	environment := serveEnvironment(production, opts.environment, cfg.Environment)
	cfg.Environment = string(environment)
	instanceLock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer instanceLock.Release()
	application, err := app.Build(ctx, cfg)
	if err != nil {
		return err
	}
	serveCtx, stopServe := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopServe()
	if err := application.Start(serveCtx); err != nil {
		_ = application.Shutdown(context.Background())
		return err
	}
	fatalErr := make(chan error, 1)
	go func() {
		select {
		case <-serveCtx.Done():
		case err := <-application.Fatal():
			fatalErr <- err
			stopServe()
		}
	}()
	slog.Info("LeapView listening", "url", listenURL(addr), "environment", environment)
	err = runHTTPServer(serveCtx, productionHTTPServer(addr, application.Handler()))
	stopServe()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPServerShutdownTimeout)
	defer cancel()
	if stopErr := application.Shutdown(shutdownCtx); err == nil && stopErr != nil {
		err = stopErr
	}
	select {
	case fatal := <-fatalErr:
		return fatal
	default:
	}
	return err
}

func serveProductionMode(cfg config.Config, opts rootOptions) bool {
	return opts.production || cfg.Production
}

func serveEnvironment(production bool, flagValue, configuredValue string) servingstate.Environment {
	if value := strings.TrimSpace(flagValue); value != "" {
		return servingstate.NormalizeEnvironment(servingstate.Environment(value))
	}
	if value := strings.TrimSpace(configuredValue); value != "" {
		return servingstate.NormalizeEnvironment(servingstate.Environment(value))
	}
	if production {
		return servingstate.Environment("prod")
	}
	return servingstate.DefaultEnvironment
}

func serveEnvironmentFlagValue(changed bool, value string) string {
	if !changed {
		return ""
	}
	return value
}

func listenURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

func productionHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
}

func runHTTPServer(ctx context.Context, server *http.Server) error {
	if server == nil {
		return errors.New("http server is required")
	}
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return err
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPServerShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}
