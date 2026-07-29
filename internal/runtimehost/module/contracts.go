package module

import "github.com/flidai/leapview/internal/runtimehost"

type Provider = runtimehost.Provider
type ManagedDataResolver = runtimehost.ManagedDataResolver
type CandidateRegistration = runtimehost.CandidateRegistration
type CandidateLeaseRequest = runtimehost.CandidateLeaseRequest
type CandidateCompatibility = runtimehost.CandidateCompatibility
type CandidateBindingVersion = runtimehost.CandidateBindingVersion
type CandidateRestriction = runtimehost.CandidateRestriction
type CandidatePreparation = runtimehost.CandidatePreparation
type OwnedCandidateView = runtimehost.OwnedCandidateView
type OwnedCandidateWorkspace = runtimehost.OwnedCandidateWorkspace
type RuntimeLifetime = runtimehost.RuntimeLifetime

var (
	ErrCandidateRuntimeInvalid      = runtimehost.ErrCandidateRuntimeInvalid
	ErrCandidateRuntimeNotFound     = runtimehost.ErrCandidateRuntimeNotFound
	ErrCandidateRuntimeIncompatible = runtimehost.ErrCandidateRuntimeIncompatible
	ErrCandidateRuntimeConflict     = runtimehost.ErrCandidateRuntimeConflict
	ErrCandidateRuntimeExpired      = runtimehost.ErrCandidateRuntimeExpired
	ErrCandidateRuntimeClosed       = runtimehost.ErrCandidateRuntimeClosed
)
