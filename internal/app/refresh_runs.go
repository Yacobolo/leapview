package app

import (
	"context"

	workloadmodule "github.com/flidai/leapview/internal/workload/module"
)

func workloadController(current *workloadControl) workloadControl {
	if *current == nil {
		*current, _ = workloadmodule.Build(context.Background(), workloadmodule.Config{Policy: workloadmodule.DefaultConfig()})
	}
	return *current
}
