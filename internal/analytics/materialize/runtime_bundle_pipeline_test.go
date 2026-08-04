package materialize

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/arrowresult"
	"github.com/flidai/leapview/internal/analytics/dataquery"
	"github.com/stretchr/testify/require"
)

func TestBundlePipelineCancellationBoundariesReleaseArrowOwnership(t *testing.T) {
	stages := []bundleStage{
		bundleStageGovern,
		bundleStageCache,
		bundleStagePlan,
		bundleStageExecute,
		bundleStageSplitStoreDecode,
		bundleStageTransformObserve,
	}
	for _, stage := range stages {
		t.Run(string(stage), func(t *testing.T) {
			database := &bundleCountingDatabase{}
			runtime := bundleCacheRuntime(database)
			before := arrowresult.Stats()
			ctx, cancel := context.WithCancel(context.Background())
			ctx = withBundleStageObserver(ctx, func(current bundleStage) {
				if current == stage {
					cancel()
				}
			})
			_, err := runtime.ExecuteDataQueryBundle(ctx, bundleCacheRequests())
			require.ErrorIs(t, err, context.Canceled)
			require.NoError(t, runtime.CloseView())
			require.Eventually(t, func() bool { return before == arrowresult.Stats() }, time.Second, time.Millisecond)
		})
	}
}

func TestBundlePipelineTransformFailureIsBranchAttributedAndReleasesArrowOwnership(t *testing.T) {
	database := &bundleCountingDatabase{}
	runtime := bundleCacheRuntime(database)
	before := arrowresult.Stats()
	want := errors.New("transform failed")
	governor := failingTransformGovernor{err: want}
	_, err := runtime.ExecuteDataQueryBundle(dataquery.WithGovernor(context.Background(), governor), bundleCacheRequests())
	var branchErr *dataquery.BundleBranchError
	require.ErrorAs(t, err, &branchErr)
	require.Equal(t, "orders", branchErr.ID)
	require.ErrorIs(t, err, want)
	require.Equal(t, int32(1), database.queries.Load())
	require.NoError(t, runtime.CloseView())
	require.Eventually(t, func() bool { return before == arrowresult.Stats() }, time.Second, time.Millisecond)
}

func TestBundlePipelineAdmitsAndObservesOnePhysicalQuery(t *testing.T) {
	database := &bundleCountingDatabase{}
	runtime := bundleCacheRuntime(database)
	defer runtime.CloseView()
	observations := []dataquery.PhysicalQueryObservation{}
	ctx := dataquery.WithPhysicalQueryObserver(context.Background(), func(observation dataquery.PhysicalQueryObservation) {
		observations = append(observations, observation)
	})
	result, err := runtime.ExecuteDataQueryBundle(ctx, bundleCacheRequests())
	require.NoError(t, err)
	require.Len(t, observations, 1)
	require.Equal(t, 1, observations[0].Count)
	require.Equal(t, int32(1), database.queries.Load())
	require.Equal(t, dataquery.CacheMiss, result.Results["orders"].CacheOutcome)
	require.Equal(t, dataquery.CacheMiss, result.Results["events"].CacheOutcome)
}

type failingTransformGovernor struct{ err error }

func (governor failingTransformGovernor) GovernDataQuery(_ context.Context, request dataquery.Query) (dataquery.Query, dataquery.ResultTransformer, error) {
	return request, func(*dataquery.Result, error) error { return governor.err }, nil
}
