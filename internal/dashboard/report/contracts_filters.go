package report

func (targets InteractionTargets) IsEmpty() bool {
	return len(targets.Visuals) == 0 && len(targets.Tables) == 0
}

func (targets InteractionTargets) Contains(kind, id string) bool {
	switch kind {
	case "visual":
		return containsString(targets.Visuals, id)
	case "table":
		return containsString(targets.Tables, id)
	default:
		return false
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
