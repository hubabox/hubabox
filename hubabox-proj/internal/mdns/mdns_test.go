package mdns

import "testing"

func TestListenPort(t *testing.T) {
	tests := []struct {
		addr string
		want int
	}{
		{":8787", 8787},
		{"0.0.0.0:9090", 9090},
		{"127.0.0.1:80", 80},
		{"", 8787},
	}
	for _, tc := range tests {
		if got := ListenPort(tc.addr); got != tc.want {
			t.Errorf("ListenPort(%q) = %d, want %d", tc.addr, got, tc.want)
		}
	}
}
