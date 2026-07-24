package module

import "github.com/Yacobolo/leapview/internal/dashboard/queryruntime"

// Metrics is the dashboard capability's runtime query surface. Composition
// refers to the capability-owned name while the implementation contract
// remains shared by dashboard transports and runtime factories.
type Metrics = queryruntime.Metrics

type WorkspaceMetrics = queryruntime.WorkspaceMetrics
