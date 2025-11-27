package tftp

import "testing"

func TestSelectHTTPNetwork(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "ipv4 unspecified", addr: ":8069", want: "tcp4"},
		{name: "ipv4 all interfaces", addr: "0.0.0.0:8069", want: "tcp4"},
		{name: "ipv4 loopback", addr: "127.0.0.1:8080", want: "tcp4"},
		{name: "ipv6 unspecified", addr: "[::]:9090", want: "tcp6"},
		{name: "hostname", addr: "localhost:9090", want: "tcp"},
		{name: "missing port", addr: "9090", want: "tcp"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := selectHTTPNetwork(tt.addr); got != tt.want {
				t.Fatalf("selectHTTPNetwork(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
