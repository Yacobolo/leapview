package main

import "testing"

func TestEndpointUsesThePublicServerNameAndListenerPort(t *testing.T) {
	for _, test := range []struct {
		name, listen, want string
	}{
		{"rtest.example.com", ":50052", "rtest.example.com:50052"},
		{"127.0.0.1", "127.0.0.1:1235", "127.0.0.1:1235"},
		{"2001:db8::1", "[::]:50052", "[2001:db8::1]:50052"},
	} {
		if got := endpoint(test.name, test.listen); got != test.want {
			t.Errorf("endpoint(%q, %q) = %q, want %q", test.name, test.listen, got, test.want)
		}
	}
}
