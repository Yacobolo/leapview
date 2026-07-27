package module

import (
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
	refreshschedule "github.com/Yacobolo/leapview/internal/refresh/schedule"
	"github.com/Yacobolo/leapview/internal/workload"
)

type Clock = refreshschedule.Clock
type RunRecord = refreshrun.RunRecord
type CandidateValidationHook = refreshrun.CandidateValidationHook
type Service = refreshrun.Service
type QueuePipelineInput = refreshrun.QueuePipelineInput
type QueueAssetResult = refreshrun.QueueAssetResult
type ServingStateRepository = refreshrun.ServingStateRepository
type WorkloadStats = workload.Stats

const RunStatusSucceeded = refreshrun.RunStatusSucceeded

func NewRealClock() Clock {
	return refreshschedule.RealClock{}
}
