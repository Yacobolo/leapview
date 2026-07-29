package module

import "github.com/flidai/leapview/internal/runtimehost"

type Provider = runtimehost.Provider
type ManagedDataResolver = runtimehost.ManagedDataResolver
type CandidateRegistration = runtimehost.CandidateRegistration
type CandidateLeaseRequest = runtimehost.CandidateLeaseRequest
type CandidateCompatibility = runtimehost.CandidateCompatibility
type CandidateBindingVersion = runtimehost.CandidateBindingVersion

var (
	ErrCandidateRuntimeInvalid      = runtimehost.ErrCandidateRuntimeInvalid
	ErrCandidateRuntimeNotFound     = runtimehost.ErrCandidateRuntimeNotFound
	ErrCandidateRuntimeIncompatible = runtimehost.ErrCandidateRuntimeIncompatible
	ErrCandidateRuntimeConflict     = runtimehost.ErrCandidateRuntimeConflict
	ErrCandidateRuntimeExpired      = runtimehost.ErrCandidateRuntimeExpired
	ErrCandidateRuntimeClosed       = runtimehost.ErrCandidateRuntimeClosed
)
