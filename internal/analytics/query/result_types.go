package query

import (
	"fmt"
	"strings"
)

// Row and Rows are planner-boundary values used by pure planning and
// projection helpers. Governed execution does not use them as its physical
// result representation; that boundary is Arrow-native.
type Row map[string]any

type Rows []Row

type FloatBounds struct {
	Min   float64
	Max   float64
	Valid bool
}

type HistogramBin struct {
	Bucket int
	Count  int
	Start  float64
	End    float64
}

type DistributionSpec struct {
	GroupColumn string
	ValueColumn string
	Sort        []Sort
	Limit       int
}

type HistogramSpec struct {
	ValueColumn string
	BinCount    int
}

func validateDistributionSort(sort []Sort) error {
	for _, sortSpec := range sort {
		field := sortSpec.Field
		if field == "" {
			field = "label"
		}
		switch field {
		case "label", "min", "q1", "median", "q3", "max":
		default:
			return fmt.Errorf("unsupported distribution sort field %q", sortSpec.Field)
		}
		if sortSpec.Direction != "" && !strings.EqualFold(sortSpec.Direction, "asc") && !strings.EqualFold(sortSpec.Direction, "desc") {
			return fmt.Errorf("unsupported sort direction %q", sortSpec.Direction)
		}
	}
	return nil
}
