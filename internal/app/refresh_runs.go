package app

import (
	"context"

	workloadmodule "github.com/Yacobolo/leapview/internal/workload/module"
)

func (s *applicationAssembly) workloadController() workloadControl {
	if s.runtime.workloads == nil {
		s.runtime.workloads, _ = workloadmodule.Build(context.Background(), workloadmodule.Config{Policy: workloadmodule.DefaultConfig()})
	}
	return s.runtime.workloads
}
