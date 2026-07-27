package runtimebinding

import (
	"strings"
	"testing"
)

func TestBindRootsUsesTrustedRuntimeResolution(t *testing.T) {
	target := &recordingTarget{
		connections: []Connection{{ModelID: "sales", Name: "olist"}},
		bound:       map[Connection]string{},
	}
	if err := BindRoots(target, map[string]string{"olist": "/managed/olist/revision"}); err != nil {
		t.Fatal(err)
	}
	if got := target.bound[Connection{ModelID: "sales", Name: "olist"}]; got != "/managed/olist/revision" {
		t.Fatalf("bound root = %q", got)
	}
}

func TestBindRootsRequiresEveryManagedConnection(t *testing.T) {
	target := &recordingTarget{connections: []Connection{{ModelID: "sales", Name: "olist"}}}
	err := BindRoots(target, nil)
	if err == nil || !strings.Contains(err.Error(), "olist") {
		t.Fatalf("bind error = %v, want missing olist revision", err)
	}
}

type recordingTarget struct {
	connections []Connection
	bound       map[Connection]string
}

func (t *recordingTarget) ManagedConnections() []Connection {
	return append([]Connection(nil), t.connections...)
}

func (t *recordingTarget) BindManagedRoot(connection Connection, root string) error {
	if t.bound == nil {
		t.bound = map[Connection]string{}
	}
	t.bound[connection] = root
	return nil
}
