package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

func TestTargetRuntimePoolFactoryPreparesOnlyConnectorOwnedReadOnlyProbe(t *testing.T) {
	session := &recordingTargetSession{}
	factory, err := NewTargetRuntimePoolFactory(TargetRuntimePoolFactoryConfig{
		Open: func(context.Context) (TargetRuntimeSession, error) { return session, nil },
		Limits: TargetRuntimeLimits{
			MemoryMaxBytes: 64 << 20, TempMaxBytes: 16 << 20, MaxThreads: 1,
		},
		RequireTLS: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := testDuckDBTargetBinding(t)
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"password": "source-secret"},
		"secret-1:v4", time.Now(), time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := factory.Prepare(context.Background(), binding, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(session.statements, "\n")
	for _, required := range []string{
		"SET allow_persistent_secrets = false",
		"SET autoinstall_known_extensions = false",
		"SET autoload_known_extensions = false",
		"SET memory_limit = '67108864B'",
		"SET max_temp_directory_size = '16777216B'",
		"SET threads = 1",
		"INSTALL postgres FROM core",
		"LOAD postgres",
		"CREATE OR REPLACE TEMPORARY SECRET leapview_warehouse",
		"HOST 'warehouse.internal'",
		"PASSWORD 'source-secret'",
		"ATTACH '' AS conn_warehouse (TYPE postgres, READ_ONLY, SECRET leapview_warehouse)",
		"SET lock_configuration = true",
		"SELECT 1",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("runtime statements missing %q:\n%s", required, joined)
		}
	}
	for _, forbidden := range []string{"PERSISTENT SECRET", "READ_WRITE", "postgres_execute", "mysql_execute"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("runtime statements contain forbidden capability %q:\n%s", forbidden, joined)
		}
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if !session.closed {
		t.Fatal("runtime session was not closed")
	}
}

func TestTargetRuntimePoolFactoryRejectsUnboundedOrUnsupportedEndpointsBeforeOpeningRuntime(t *testing.T) {
	opened := 0
	factory, err := NewTargetRuntimePoolFactory(TargetRuntimePoolFactoryConfig{
		Open: func(context.Context) (TargetRuntimeSession, error) {
			opened++
			return &recordingTargetSession{}, nil
		},
		Limits:     TargetRuntimeLimits{MemoryMaxBytes: 1, TempMaxBytes: 1, MaxThreads: 1},
		RequireTLS: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"password": "source-secret"},
		"secret-1:v4", time.Now(), time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*connectionbinding.TargetBinding){
		"unsupported_connector": func(binding *connectionbinding.TargetBinding) {
			binding.ConnectorKind = "s3"
			binding.Endpoint.ObjectScope = "s3://warehouse/"
		},
		"missing_host": func(binding *connectionbinding.TargetBinding) {
			binding.Endpoint.Host = ""
		},
		"missing_database": func(binding *connectionbinding.TargetBinding) {
			binding.Endpoint.Database = ""
		},
		"missing_identity": func(binding *connectionbinding.TargetBinding) {
			binding.Endpoint.SourceIdentity = ""
		},
		"insecure_transport": func(binding *connectionbinding.TargetBinding) {
			binding.Endpoint.TLSMode = "disable"
		},
	} {
		t.Run(name, func(t *testing.T) {
			binding := testDuckDBTargetBinding(t)
			mutate(&binding)
			if _, err := factory.Prepare(context.Background(), binding, snapshot); err == nil {
				t.Fatal("Prepare() accepted an unbounded target")
			}
		})
	}
	if opened != 0 {
		t.Fatalf("opened runtimes for rejected targets = %d", opened)
	}
}

func TestTargetRuntimePoolHealthAndCloseAreIdempotentAndPropagateOnlyInternally(t *testing.T) {
	sourceErr := errors.New("driver included source-secret")
	session := &recordingTargetSession{failOn: "SELECT 1", err: sourceErr}
	factory, err := NewTargetRuntimePoolFactory(TargetRuntimePoolFactoryConfig{
		Open:       func(context.Context) (TargetRuntimeSession, error) { return session, nil },
		Limits:     TargetRuntimeLimits{MemoryMaxBytes: 1, TempMaxBytes: 1, MaxThreads: 1},
		RequireTLS: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"password": "source-secret"},
		"secret-1:v4", time.Now(), time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := factory.Prepare(context.Background(), testDuckDBTargetBinding(t), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.HealthCheck(context.Background()); !errors.Is(err, sourceErr) {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if session.closeCalls != 1 {
		t.Fatalf("session close calls = %d", session.closeCalls)
	}
}

func TestIsolatedTargetRuntimeOpenerCreatesPrivateSingleConnectionSession(t *testing.T) {
	open := NewIsolatedTargetRuntimeOpener()
	session, err := open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.ExecContext(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

type recordingTargetSession struct {
	statements []string
	failOn     string
	err        error
	closed     bool
	closeCalls int
}

func (session *recordingTargetSession) ExecContext(
	_ context.Context,
	statement string,
	_ ...any,
) (sql.Result, error) {
	session.statements = append(session.statements, statement)
	if session.failOn != "" && strings.Contains(statement, session.failOn) {
		return nil, session.err
	}
	return nil, nil
}

func (session *recordingTargetSession) Close() error {
	session.closeCalls++
	session.closed = true
	return nil
}
