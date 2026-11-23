package diagnostics

import "testing"

func TestIsLoopbackAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{name: "ipv4 loopback", addr: "127.0.0.1:6060", want: true},
		{name: "localhost", addr: "localhost:6060", want: true},
		{name: "ipv6 loopback", addr: "[::1]:6060", want: true},
		{name: "all interfaces", addr: "0.0.0.0:6060", want: false},
		{name: "private network", addr: "10.0.0.5:6060", want: false},
		{name: "missing port", addr: "127.0.0.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLoopbackAddress(tt.addr); got != tt.want {
				t.Fatalf("isLoopbackAddress(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}
