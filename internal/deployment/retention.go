package deployment

import "time"

type RetentionPolicy struct {
	ProtectActive              bool
	ProtectDraining            bool
	QueryDrainGrace            time.Duration
	RequireApplyForDestructive bool
}

const DefaultQueryDrainGrace = 15 * time.Minute

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		ProtectActive:              true,
		ProtectDraining:            true,
		QueryDrainGrace:            DefaultQueryDrainGrace,
		RequireApplyForDestructive: true,
	}
}
