package cmd

import "testing"

func TestIsNonLoopbackAddr(t *testing.T) {
	loopback := []string{
		"127.0.0.1",
		"127.0.0.1:8080",
		"localhost",
		"::1",
	}
	for _, a := range loopback {
		if isNonLoopbackAddr(a) {
			t.Errorf("%s: expected loopback", a)
		}
	}

	nonLoopback := []string{
		"0.0.0.0",
		"10.0.0.5",
		"192.168.1.10",
		"172.17.0.1:8080",
		"example.com",
	}
	for _, a := range nonLoopback {
		if !isNonLoopbackAddr(a) {
			t.Errorf("%s: expected non-loopback", a)
		}
	}
}
