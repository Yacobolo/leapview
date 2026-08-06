package connectors

import "testing"

func TestQuackURIRequiresCanonicalBoundedEndpoint(t *testing.T) {
	for name, test := range map[string]struct {
		host string
		port int
		want string
	}{
		"dns":  {host: "quack.example.com", port: 443, want: "quack:quack.example.com:443"},
		"ipv4": {host: "192.0.2.4", port: 444, want: "quack:192.0.2.4:444"},
		"ipv6": {host: "2001:db8::4", port: 443, want: "quack:[2001:db8::4]:443"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := QuackURI(test.host, test.port)
			if err != nil || got != test.want {
				t.Fatalf("QuackURI(%q, %d) = %q, %v; want %q", test.host, test.port, got, err, test.want)
			}
		})
	}

	for name, test := range map[string]struct {
		host string
		port int
	}{
		"missing_host":  {port: 443},
		"missing_port":  {host: "quack.example.com"},
		"invalid_port":  {host: "quack.example.com", port: 65536},
		"whitespace":    {host: " quack.example.com", port: 443},
		"scheme":        {host: "https://quack.example.com", port: 443},
		"path":          {host: "quack.example.com/api", port: 443},
		"userinfo":      {host: "token@quack.example.com", port: 443},
		"invalid_colon": {host: "quack.example.com:443", port: 443},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := QuackURI(test.host, test.port); err == nil {
				t.Fatalf("QuackURI(%q, %d) = %q, want rejection", test.host, test.port, got)
			}
		})
	}
}
