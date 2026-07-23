package module

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	appconfig "github.com/Yacobolo/leapview/internal/config"
	"github.com/Yacobolo/leapview/internal/manageddata"
	"github.com/Yacobolo/leapview/internal/manageddata/apiadapter"
	"github.com/Yacobolo/leapview/internal/manageddata/binding"
	"github.com/Yacobolo/leapview/internal/manageddata/control"
	manageddatahttp "github.com/Yacobolo/leapview/internal/manageddata/http"
	"github.com/Yacobolo/leapview/internal/manageddata/maintenance"
	maintenancesqlite "github.com/Yacobolo/leapview/internal/manageddata/maintenance/sqlite"
	manageddataresolver "github.com/Yacobolo/leapview/internal/manageddata/resolver"
	"github.com/Yacobolo/leapview/internal/manageddata/runtimeview"
	"github.com/Yacobolo/leapview/internal/manageddata/s3multipart"
	manageddatasqlite "github.com/Yacobolo/leapview/internal/manageddata/sqlite"
	"github.com/Yacobolo/leapview/internal/manageddata/storage"
	managedfilesystem "github.com/Yacobolo/leapview/internal/manageddata/storage/filesystem"
	manageds3 "github.com/Yacobolo/leapview/internal/manageddata/storage/s3"
	managedtus "github.com/Yacobolo/leapview/internal/manageddata/storage/tus"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	"github.com/Yacobolo/leapview/internal/securefs"
	"github.com/Yacobolo/leapview/internal/servingstate"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	managedDataTusPath             = "/upload-protocols/tus"
	managedDataS3MultipartTemplate = "/api/v1/projects/{project}/connections/{connection}/upload-sessions/{uploadSession}/s3-multipart-uploads"
)

type managedDataStorage struct {
	blobs        storage.BlobStore
	inventory    storage.BlobInventory
	transport    control.Transport
	materializer manageddata.RevisionMaterializer
	runtimeCache *runtimeview.Cache
	tus          http.Handler
	s3           *manageds3.Store
}

// Module owns managed-data adapter construction for one application process.
// Its exported methods are deliberately named ports instead of exposing the
// internal storage bundle.
type Module struct {
	handler           *manageddatahttp.Handler
	uploads           *control.Service
	finalizer         manageddatahttp.UploadCoordinator
	multipart         s3multipart.Coordinator
	multipartService  *s3multipart.Service
	materializer      manageddata.RevisionMaterializer
	tus               http.Handler
	maintenance       Maintenance
	maintenanceWorker *maintenanceWorker
	jobs              JobStore
	bindings          *binding.Binder
	runtimeResolver   *manageddataresolver.Resolver
	metadata          DeploymentMetadata
}

type repository interface {
	control.Repository
	s3multipart.Repository
	apiadapter.Repository
	binding.Repository
	manageddataresolver.Repository
	DeploymentMetadata
}

type JobStore interface {
	Enqueue(context.Context, jobs.EnqueueInput) (jobs.Job, error)
	AppendEvent(context.Context, string, string, string, []byte) (jobs.Event, error)
	ListEvents(context.Context, string, string, int64, int) ([]jobs.Event, error)
}

type Principal struct {
	ID string
}

type Config struct {
	Database         *sql.DB
	Disabled         bool
	Product          appconfig.Config
	Worker           MaintenanceWorkerConfig
	MaxJSONBodyBytes int64
	Environment      string
	CurrentPrincipal func(*http.Request) (Principal, bool)
	Jobs             JobStore
	ServingStates    ServingStateReader
}

type ServingStateReader interface {
	ByID(context.Context, servingstate.ID) (servingstate.State, error)
}

func Build(ctx context.Context, cfg Config) (*Module, error) {
	currentPrincipal := func(r *http.Request) (manageddatahttp.Principal, bool) {
		if cfg.CurrentPrincipal == nil {
			return manageddatahttp.Principal{}, false
		}
		principal, ok := cfg.CurrentPrincipal(r)
		return manageddatahttp.Principal{ID: principal.ID}, ok
	}
	if cfg.Disabled {
		module := &Module{jobs: cfg.Jobs}
		module.handler = manageddatahttp.NewHandler(manageddatahttp.Options{
			CurrentPrincipal: currentPrincipal, MaxJSONBodyBytes: cfg.MaxJSONBodyBytes,
			Environment: cfg.Environment,
		})
		return module, nil
	}
	if cfg.Database == nil {
		return nil, errors.New("managed-data database is required")
	}
	repository := manageddatasqlite.NewRepository(cfg.Database)
	services, err := newManagedDataStorage(ctx, cfg.Product)
	if err != nil {
		return nil, err
	}
	uploads, err := newManagedDataControl(repository, services, cfg.Product)
	if err != nil {
		return nil, err
	}
	collector, err := newManagedDataCollector(cfg.Database, services, cfg.Product)
	if err != nil {
		return nil, err
	}
	runtimeCollector, err := newManagedDataRuntimeCollector(services, cfg.Product)
	if err != nil {
		return nil, err
	}
	var multipart s3multipart.Coordinator
	var multipartService *s3multipart.Service
	if services.s3 != nil {
		multipartService, err = s3multipart.New(repository, services.s3, s3multipart.Config{Backend: "s3"})
		if err != nil {
			return nil, err
		}
		multipart = multipartService
	}
	apiRepository, err := apiadapter.New(repository)
	if err != nil {
		return nil, err
	}
	bindings, err := binding.New(repository)
	if err != nil {
		return nil, err
	}
	var runtimeResolver *manageddataresolver.Resolver
	if cfg.ServingStates != nil {
		runtimeResolver, err = manageddataresolver.New(repository, cfg.ServingStates, services.materializer)
		if err != nil {
			return nil, err
		}
	}
	module := &Module{
		uploads:          uploads,
		finalizer:        uploads,
		multipart:        multipart,
		multipartService: multipartService,
		materializer:     services.materializer,
		tus:              services.tus,
		maintenance: Maintenance{
			uploads: uploads, multipart: multipartService, uploadTTL: cfg.Product.ManagedDataUploadSessionTTL,
			collector: collector, runtime: runtimeCollector,
		},
		jobs: cfg.Jobs, bindings: bindings, runtimeResolver: runtimeResolver,
		metadata: metadataReader{repository: repository},
	}
	module.handler = manageddatahttp.NewHandler(manageddatahttp.Options{
		Repository: apiRepository, Uploads: uploads, Multipart: multipart,
		CurrentPrincipal: currentPrincipal, Environment: cfg.Environment,
		EnqueueFinalize: module.enqueueFinalize, RecordUploadCreated: module.recordUploadCreated,
	})
	module.maintenanceWorker = newMaintenanceWorker(module.maintenance, cfg.Worker)
	return module, nil
}

func (m *Module) HasFinalizeJobs() bool { return m.finalizer != nil }

func (m *Module) SupportsS3Multipart() bool { return m != nil && m.multipart != nil }

func (m *Module) Materializer() manageddata.RevisionMaterializer { return m.materializer }

type BindingValidation interface {
	AfterArtifactValidation(context.Context, servingstate.State, servingstate.Validation) error
	ValidateServingStatePins(context.Context, string, string, map[string]string) error
}

func (m *Module) BindingValidation() BindingValidation {
	if m == nil {
		return nil
	}
	return m.bindings
}

type RuntimeResolver interface {
	ResolveManagedData(context.Context, servingstate.ID) (manageddataresolver.Resolution, error)
}

func (m *Module) RuntimeResolution() RuntimeResolver {
	if m == nil {
		return nil
	}
	return m.runtimeResolver
}

type DeploymentMetadata interface {
	CollectionByID(context.Context, string) (manageddata.Collection, error)
	RevisionByID(context.Context, string) (manageddata.Revision, error)
}

type metadataReader struct {
	repository repository
}

func (r metadataReader) CollectionByID(ctx context.Context, id string) (manageddata.Collection, error) {
	return r.repository.CollectionByID(ctx, id)
}

func (r metadataReader) RevisionByID(ctx context.Context, id string) (manageddata.Revision, error) {
	return r.repository.RevisionByID(ctx, id)
}

func (m *Module) DeploymentMetadata() DeploymentMetadata {
	if m == nil {
		return nil
	}
	return m.metadata
}

func (m *Module) TusHandler() http.Handler { return m.tus }

func (m *Module) Start(ctx context.Context) {
	if m != nil {
		m.maintenanceWorker.Start(ctx)
	}
}

func (m *Module) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	return m.maintenanceWorker.Stop(ctx)
}

func (m *Module) HTTP() *manageddatahttp.Handler { return m.handler }

func newManagedDataStorage(ctx context.Context, cfg appconfig.Config) (managedDataStorage, error) {
	root, err := filepath.Abs(strings.TrimSpace(cfg.ManagedDataDir))
	if err != nil || strings.TrimSpace(cfg.ManagedDataDir) == "" {
		return managedDataStorage{}, fmt.Errorf("%w: managed-data directory is required", storage.ErrInvalid)
	}
	if err := securefs.EnsurePrivateDir(root); err != nil {
		return managedDataStorage{}, err
	}

	var result managedDataStorage
	switch strings.TrimSpace(cfg.ManagedDataBackend) {
	case "local":
		blobs, err := managedfilesystem.New(filepath.Join(root, "objects"))
		if err != nil {
			return managedDataStorage{}, err
		}
		engine, err := managedtus.New(filepath.Join(root, "uploads"), blobs)
		if err != nil {
			return managedDataStorage{}, err
		}
		transport, err := control.NewTusTransport("local", managedDataTusPath, engine)
		if err != nil {
			return managedDataStorage{}, err
		}
		handler, err := engine.HTTPHandler(managedtus.HTTPConfig{BasePath: managedDataTusPath, MaxSize: cfg.ManagedDataMaxFileBytes})
		if err != nil {
			return managedDataStorage{}, err
		}
		capacity, err := maintenance.NewCapacityChecker(root, cfg.ManagedDataMinFreeBytes)
		if err != nil {
			return managedDataStorage{}, err
		}
		result.blobs, result.transport, result.materializer, result.tus = blobs, transport, blobs, capacityProtectedTus(handler, capacity)
	case "s3":
		store, err := newManagedDataS3Store(ctx, cfg)
		if err != nil {
			return managedDataStorage{}, err
		}
		transport, err := control.NewS3MultipartTransport("s3", control.S3MultipartDescription{
			CreateEndpoint:  managedDataS3MultipartTemplate,
			MinimumPartSize: s3multipart.MinimumPartSize,
			MaximumPartSize: s3multipart.MaximumPartSize,
			MaximumParts:    s3multipart.MaximumParts,
		})
		if err != nil {
			return managedDataStorage{}, err
		}
		cache, err := runtimeview.New(filepath.Join(root, "runtime"), store)
		if err != nil {
			return managedDataStorage{}, err
		}
		result.blobs, result.transport, result.materializer, result.runtimeCache, result.s3 = store, transport, cache, cache, store
	default:
		return managedDataStorage{}, fmt.Errorf("%w: managed-data backend must be local or s3", storage.ErrInvalid)
	}
	inventory, ok := result.blobs.(storage.BlobInventory)
	if !ok {
		return managedDataStorage{}, fmt.Errorf("%w: managed-data backend has no blob inventory", storage.ErrInvalid)
	}
	result.inventory = inventory
	return result, nil
}

func newManagedDataCollector(db *sql.DB, services managedDataStorage, cfg appconfig.Config) (*maintenance.BlobCollector, error) {
	reachability, err := maintenancesqlite.New(db)
	if err != nil {
		return nil, err
	}
	return maintenance.NewBlobCollector(services.inventory, reachability, maintenance.BlobGCConfig{
		GraceAge: cfg.ManagedDataGCGracePeriod,
	})
}

func newManagedDataRuntimeCollector(services managedDataStorage, cfg appconfig.Config) (*maintenance.RuntimeViewCollector, error) {
	if services.runtimeCache == nil {
		return nil, nil
	}
	return maintenance.NewRuntimeViewCollector(services.runtimeCache, maintenance.RuntimeViewGCConfig{
		GraceAge: cfg.ManagedDataGCGracePeriod,
		Limit:    100,
	})
}

func capacityProtectedTus(next http.Handler, capacity *maintenance.CapacityChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength < 0 {
			http.Error(w, "Content-Length is required", http.StatusLengthRequired)
			return
		}
		reservation, err := capacity.Reserve(r.Context(), r.ContentLength)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, maintenance.ErrInsufficientCapacity) {
				status = http.StatusInsufficientStorage
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		defer reservation.Release()
		next.ServeHTTP(w, r)
	})
}

func newManagedDataS3Store(ctx context.Context, cfg appconfig.Config) (*manageds3.Store, error) {
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(strings.TrimSpace(cfg.ManagedDataS3Region))}
	if cfg.ManagedDataS3AccessKeyID != "" {
		provider := credentials.NewStaticCredentialsProvider(
			cfg.ManagedDataS3AccessKeyID,
			cfg.ManagedDataS3SecretAccessKey,
			cfg.ManagedDataS3SessionToken,
		)
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(provider))
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("initialize managed-data S3 client: %w", err)
	}
	client := awss3.NewFromConfig(awsConfig, func(options *awss3.Options) {
		options.UsePathStyle = cfg.ManagedDataS3PathStyle
		if endpoint := strings.TrimSpace(cfg.ManagedDataS3Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})
	return manageds3.New(client, awss3.NewPresignClient(client), manageds3.Config{
		Bucket: cfg.ManagedDataS3Bucket,
		Prefix: cfg.ManagedDataS3Prefix,
	})
}

func newManagedDataControl(repo control.Repository, services managedDataStorage, cfg appconfig.Config) (*control.Service, error) {
	return control.New(repo, services.blobs, control.Config{
		Limits: manageddata.Limits{
			MaxFiles:         cfg.ManagedDataMaxFiles,
			MaxFileBytes:     cfg.ManagedDataMaxFileBytes,
			MaxRevisionBytes: cfg.ManagedDataMaxRevisionBytes,
		},
		UploadTTL: cfg.ManagedDataUploadSessionTTL,
		Transport: services.transport,
	})
}
