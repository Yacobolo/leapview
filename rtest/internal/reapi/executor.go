package reapi

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/outerr"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/rexec"
	longrunning "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Outcome struct {
	ExitCode             int
	Status               command.ResultStatus
	ActionDigest         string
	InputFiles           int
	TotalInputBytes      int64
	LogicalBytesUploaded int64
	RealBytesUploaded    int64
}

func Check(ctx context.Context, service, instance string) error {
	grpcClient, err := client.NewClient(ctx, instance, client.DialParams{Service: service, NoSecurity: true})
	if err != nil {
		return fmt.Errorf("connect to REAPI service: %w", err)
	}
	return grpcClient.Close()
}

func Execute(ctx context.Context, service, instance string, request Request, stdout, stderr io.Writer) (Outcome, error) {
	tracker := &operationTracker{}
	grpcClient, err := client.NewClient(ctx, instance, client.DialParams{
		Service:    service,
		NoSecurity: true,
		DialOpts:   []grpc.DialOption{grpc.WithChainStreamInterceptor(trackOperations(tracker))},
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("connect to REAPI service: %w", err)
	}
	defer grpcClient.Close()

	cmd, options := Action(request)
	executor := &rexec.Client{
		FileMetadataCache: filemetadata.NewNoopCache(),
		GrpcClient:        grpcClient,
	}
	result, metadata := executor.Run(ctx, cmd, options, outerr.NewStreamOutErr(stdout, stderr))
	if result == nil {
		return Outcome{}, fmt.Errorf("REAPI execution returned no result")
	}
	outcome := Outcome{
		ExitCode:             result.ExitCode,
		Status:               result.Status,
		ActionDigest:         metadata.ActionDigest.String(),
		InputFiles:           metadata.InputFiles,
		TotalInputBytes:      metadata.TotalInputBytes,
		LogicalBytesUploaded: metadata.LogicalBytesUploaded,
		RealBytesUploaded:    metadata.RealBytesUploaded,
	}
	if ctx.Err() != nil {
		name := tracker.name()
		if name != "" {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, cancelErr := grpcClient.CancelOperation(cancelCtx, &longrunning.CancelOperationRequest{Name: name})
			cancel()
			if cancelErr != nil {
				if status.Code(cancelErr) == codes.Unimplemented {
					fmt.Fprintln(stderr, "warning: REAPI backend does not implement CancelOperation; the action will stop at its configured timeout")
					return outcome, ctx.Err()
				}
				return outcome, fmt.Errorf("%w; cancel remote operation %s: %v", ctx.Err(), name, cancelErr)
			}
		}
		return outcome, ctx.Err()
	}
	if result.Err != nil {
		return outcome, result.Err
	}
	return outcome, nil
}

type operationTracker struct {
	mu            sync.Mutex
	operationName string
}

func (t *operationTracker) observe(operation *longrunning.Operation) {
	if operation == nil || operation.Name == "" {
		return
	}
	t.mu.Lock()
	t.operationName = operation.Name
	t.mu.Unlock()
}

func (t *operationTracker) name() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.operationName
}

func trackOperations(tracker *operationTracker) grpc.StreamClientInterceptor {
	return func(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, description, connection, method, options...)
		if err != nil {
			return nil, err
		}
		return &operationTrackingStream{ClientStream: stream, tracker: tracker}, nil
	}
}

type operationTrackingStream struct {
	grpc.ClientStream
	tracker *operationTracker
}

func (s *operationTrackingStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	if operation, ok := message.(*longrunning.Operation); ok {
		s.tracker.observe(operation)
	}
	return err
}
