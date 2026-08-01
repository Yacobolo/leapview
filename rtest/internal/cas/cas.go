package cas

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
)

type UploadResult struct {
	RootDigest       string
	InputFiles       int
	TotalInputBytes  int64
	LogicalUploaded  int64
	TransferredBytes int64
}

type Connection struct {
	Service        string
	Instance       string
	CACertFile     string
	ClientCertFile string
	ClientKeyFile  string
	ServerName     string
	Insecure       bool
}

func Check(ctx context.Context, service, instance string) error {
	return CheckConnection(ctx, Connection{Service: service, Instance: instance, Insecure: true})
}

func CheckConnection(ctx context.Context, connection Connection) error {
	grpcClient, err := connect(ctx, connection)
	if err != nil {
		return err
	}
	return grpcClient.Close()
}

func Upload(ctx context.Context, service, instance, root string, files []string) (UploadResult, error) {
	return UploadConnection(ctx, Connection{Service: service, Instance: instance, Insecure: true}, root, files)
}

func UploadConnection(ctx context.Context, connection Connection, root string, files []string) (UploadResult, error) {
	grpcClient, err := connect(ctx, connection)
	if err != nil {
		return UploadResult{}, err
	}
	defer grpcClient.Close()
	rootDigest, entries, stats, err := grpcClient.ComputeMerkleTree(ctx, root, "", "", &command.InputSpec{
		Inputs: files, SymlinkBehavior: command.PreserveSymlink,
	}, filemetadata.NewNoopCache())
	if err != nil {
		return UploadResult{}, fmt.Errorf("compute REAPI input tree: %w", err)
	}
	missing, transferred, err := grpcClient.UploadIfMissing(ctx, entries...)
	if err != nil {
		return UploadResult{}, fmt.Errorf("upload REAPI input tree: %w", err)
	}
	var logical int64
	for _, item := range missing {
		logical += item.Size
	}
	return UploadResult{
		RootDigest: rootDigest.String(), InputFiles: stats.InputFiles, TotalInputBytes: stats.TotalInputBytes,
		LogicalUploaded: logical, TransferredBytes: transferred,
	}, nil
}

func Materialize(ctx context.Context, service, instance, rootDigest, destination string) error {
	return MaterializeConnection(ctx, Connection{Service: service, Instance: instance, Insecure: true}, rootDigest, destination)
}

func MaterializeConnection(ctx context.Context, connection Connection, rootDigest, destination string) error {
	digestValue, err := digest.NewFromString(rootDigest)
	if err != nil {
		return fmt.Errorf("parse REAPI root digest: %w", err)
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	grpcClient, err := connect(ctx, connection)
	if err != nil {
		return err
	}
	defer grpcClient.Close()
	if _, _, err := grpcClient.DownloadDirectory(ctx, digestValue, destination, filemetadata.NewNoopCache()); err != nil {
		return fmt.Errorf("materialize REAPI input tree: %w", err)
	}
	return nil
}

func connect(ctx context.Context, connection Connection) (*client.Client, error) {
	if connection.Service == "" || connection.Instance == "" {
		return nil, errors.New("CAS service and instance are required")
	}
	params := client.DialParams{Service: connection.Service, NoSecurity: connection.Insecure}
	if !connection.Insecure {
		if connection.CACertFile == "" || connection.ClientCertFile == "" || connection.ClientKeyFile == "" || connection.ServerName == "" {
			return nil, errors.New("CAS mTLS requires CA, client certificate, client key, and server name")
		}
		params.NoAuth = true
		params.TLSCACertFile = connection.CACertFile
		params.TLSClientAuthCert = connection.ClientCertFile
		params.TLSClientAuthKey = connection.ClientKeyFile
		params.TLSServerName = connection.ServerName
	}
	grpcClient, err := client.NewClient(ctx, connection.Instance, params)
	if err != nil {
		return nil, fmt.Errorf("connect to REAPI CAS: %w", err)
	}
	return grpcClient, nil
}
