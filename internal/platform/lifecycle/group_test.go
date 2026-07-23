package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestGroupStartsInOrderAndStopsInReverse(t *testing.T) {
	events := []string{}
	component := func(name string) Component {
		return Component{
			Start: func(context.Context) error { events = append(events, "start:"+name); return nil },
			Stop:  func(context.Context) error { events = append(events, "stop:"+name); return nil },
		}
	}
	group := New(component("dependency"), component("worker"))
	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := group.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:dependency", "start:worker", "stop:worker", "stop:dependency"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestGroupRollsBackStartedComponents(t *testing.T) {
	events := []string{}
	wantErr := errors.New("boom")
	group := New(
		Component{
			Start: func(context.Context) error { events = append(events, "start:one"); return nil },
			Stop:  func(context.Context) error { events = append(events, "stop:one"); return nil },
		},
		Component{Start: func(context.Context) error { events = append(events, "start:two"); return wantErr }},
	)
	if err := group.Start(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Start() error = %v, want %v", err, wantErr)
	}
	want := []string{"start:one", "start:two", "stop:one"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}
