package deployment

import "time"

type RetentionPolicy struct {
	ProtectActive              bool
	ProtectInactive            bool
	InactiveRetention          time.Duration
	RequireApplyForDestructive bool
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		ProtectActive:              true,
		ProtectInactive:            true,
		InactiveRetention:          30 * 24 * time.Hour,
		RequireApplyForDestructive: true,
	}
}
