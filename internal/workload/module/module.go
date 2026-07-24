// Package module owns process composition for node-local workload admission.
package module

import (
	"context"
	"errors"
	"sync"

	"github.com/Yacobolo/leapview/internal/workload"
)

type Config struct {
	Policy   workload.Config
	Observer workload.Observer
}

type Admitter = workload.Admitter
type Stats = workload.Stats
type Observer = workload.Observer
type Request = workload.Request

const (
	BackgroundClass  = workload.Background
	MaintenanceClass = workload.Maintenance
	GlobalWorkspace  = workload.GlobalWorkspace
)

func DefaultConfig() workload.Config {
	return workload.DefaultConfig()
}

func MaintenanceRequest(operation string) Request {
	return Request{Class: MaintenanceClass, Operation: operation}
}

type Module struct {
	controller *workload.Controller
	stop       sync.Once
}

func Build(_ context.Context, config Config) (*Module, error) {
	options := []workload.Option{}
	if config.Observer != nil {
		options = append(options, workload.WithObserver(config.Observer))
	}
	controller, err := workload.New(config.Policy, options...)
	if err != nil {
		return nil, err
	}
	return &Module{controller: controller}, nil
}

func (m *Module) Acquire(ctx context.Context, request workload.Request) (workload.Lease, error) {
	if m == nil || m.controller == nil {
		return nil, errors.New("workload admission is unavailable")
	}
	return m.controller.Acquire(ctx, request)
}

func (m *Module) Stats() workload.Stats {
	if m == nil || m.controller == nil {
		return workload.Stats{}
	}
	return m.controller.Stats()
}

func (m *Module) SetObserver(observer workload.Observer) {
	if m != nil && m.controller != nil {
		m.controller.SetObserver(observer)
	}
}

func (m *Module) Close() {
	if m != nil && m.controller != nil {
		m.stop.Do(m.controller.Close)
	}
}
