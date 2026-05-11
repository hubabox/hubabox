package netutil

import (
	"net"
	"testing"
)

func TestIsLikelyLANIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.5", true},
		{"172.20.3.4", true},
		{"100.64.1.2", true},
		{"8.8.8.8", false},
		{"127.0.0.1", false},
	}
	for _, tc := range tests {
		if got := isLikelyLANIPv4(net.ParseIP(tc.ip)); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.ip, got, tc.want)
		}
	}
}

func TestSkipIface(t *testing.T) {
	if !skipIface("docker0") || !skipIface("br-1234") || !skipIface("veth0") {
		t.Fatal("expected virtual ifaces skipped")
	}
	if skipIface("wlan0") || skipIface("enp0s3") {
		t.Fatal("expected real ifaces not skipped")
	}
}
