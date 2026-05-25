package main

import "testing"

func TestListenURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "loopback", addr: "127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "bare port", addr: ":8080", want: "http://localhost:8080"},
		{name: "all interfaces", addr: "0.0.0.0:8080", want: "http://localhost:8080"},
		{name: "ipv6 all interfaces", addr: "[::]:8080", want: "http://localhost:8080"},
		{name: "already has scheme", addr: "http://localhost:8080", want: "http://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenURL(tt.addr); got != tt.want {
				t.Fatalf("listenURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
