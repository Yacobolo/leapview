package module

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/leapview/internal/config"
	"github.com/Yacobolo/leapview/internal/manageddata/control"
	"github.com/Yacobolo/leapview/internal/manageddata/maintenance"
	"github.com/Yacobolo/leapview/internal/manageddata/storage"
	"github.com/Yacobolo/leapview/internal/platform"
	servingstatemodule "github.com/Yacobolo/leapview/internal/servingstate/module"
)

func TestBuildKeepsPersistencePrivateAndExposesNamedServices(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	states, err := servingstatemodule.Build(t.Context(), servingstatemodule.Config{Database: store.SQLDB()})
	if err != nil {
		t.Fatal(err)
	}
	module, err := Build(t.Context(), Config{
		Database: store.SQLDB(), ServingStates: states,
		Product: config.Config{
			ManagedDataBackend:          "local",
			ManagedDataDir:              filepath.Join(t.TempDir(), "managed"),
			ManagedDataUploadSessionTTL: time.Hour,
			ManagedDataGCGracePeriod:    time.Hour,
			ManagedDataMaxFiles:         10,
			ManagedDataMaxFileBytes:     1024,
			ManagedDataMaxRevisionBytes: 4096,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if module.BindingValidation() == nil || module.RuntimeResolution() == nil || module.DeploymentMetadata() == nil {
		t.Fatal("managed-data module did not expose its named cross-capability services")
	}
}

func TestNewManagedDataStorageLocal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	services, err := newManagedDataStorage(context.Background(), config.Config{
		ManagedDataBackend:      "local",
		ManagedDataDir:          root,
		ManagedDataMaxFileBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if services.blobs == nil || services.transport == nil || services.materializer == nil || services.tus == nil || services.s3 != nil {
		t.Fatalf("services = %#v", services)
	}
	if services.runtimeCache != nil {
		t.Fatal("local backend unexpectedly allocated a copying runtime cache")
	}
	collector, err := newManagedDataRuntimeCollector(services, config.Config{ManagedDataGCGracePeriod: time.Hour})
	if err != nil || collector != nil {
		t.Fatalf("local runtime collector = %#v, %v; want nil", collector, err)
	}
	if services.transport.Backend() != "local" {
		t.Fatalf("backend = %q", services.transport.Backend())
	}
	for _, relative := range []string{"objects", "uploads"} {
		info, statErr := os.Stat(filepath.Join(root, relative))
		if statErr != nil {
			t.Fatalf("stat %s: %v", relative, statErr)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s permissions = %o, want private", relative, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(root, "runtime")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local runtime cache stat error = %v, want not exist", err)
	}
}

func TestCapacityProtectedTusRejectsChunkWithoutReserve(t *testing.T) {
	checker, err := maintenance.NewCapacityChecker(t.TempDir(), math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := capacityProtectedTus(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), checker)
	request := httptest.NewRequest(http.MethodPatch, "/tus/upload", strings.NewReader("x"))
	request.ContentLength = 1
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInsufficientStorage || called {
		t.Fatalf("status = %d, called = %v", recorder.Code, called)
	}
}

func TestTusMethodsAreClosedByDefault(t *testing.T) {
	handler := TusProtocolHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodConnect, http.MethodTrace} {
		t.Run(method, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(method, "/upload-protocols/tus/tus_abc", nil))
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestTusMethodsForwardsResumableOperations(t *testing.T) {
	var methods []string
	handler := TusProtocolHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, method := range []string{http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodDelete} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/upload-protocols/tus/tus_abc", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want %d", method, recorder.Code, http.StatusNoContent)
		}
	}
	if want := []string{http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodDelete}; !reflect.DeepEqual(methods, want) {
		t.Fatalf("forwarded methods = %v, want %v", methods, want)
	}
}

func TestNewManagedDataStorageRejectsUnknownBackend(t *testing.T) {
	_, err := newManagedDataStorage(context.Background(), config.Config{
		ManagedDataBackend: "shared-filesystem",
		ManagedDataDir:     t.TempDir(),
	})
	if err == nil || !errors.Is(err, storage.ErrInvalid) {
		t.Fatalf("error = %v, want storage.ErrInvalid", err)
	}
}

func TestNewManagedDataControlRequiresStorage(t *testing.T) {
	_, err := newManagedDataControl(nil, managedDataStorage{}, config.Config{})
	if err == nil || !errors.Is(err, control.ErrInvalid) {
		t.Fatalf("error = %v, want control.ErrInvalid", err)
	}
}
