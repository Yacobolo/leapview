package app

import (
	"context"

	workloadmodule "github.com/Yacobolo/leapview/internal/workload/module"
)

func (s *applicationAssembly) workloadController() workloadControl {
	if s.workloads == nil {
		s.workloads, _ = workloadmodule.Build(context.Background(), workloadmodule.Config{Policy: workloadmodule.DefaultConfig()})
	}
	return s.workloads
}
