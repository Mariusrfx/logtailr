package api

import (
	"testing"
	"time"

	"logtailr/pkg/logline"
)

func TestHub_RegisterUnregisterAfterStopDoNotBlock(t *testing.T) {
	h := NewHub()
	go h.Run()
	h.Stop()
	time.Sleep(50 * time.Millisecond) // let the Run loop observe done

	c := &Client{Send: make(chan *logline.LogLine, 1)}
	done := make(chan struct{})
	go func() {
		h.Register(c)
		h.Unregister(c)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Register/Unregister blocked after Stop")
	}
}

func TestHub_BroadcastReachesMatchingClient(t *testing.T) {
	h := NewHub()
	go h.Run()
	defer h.Stop()

	c := &Client{Send: make(chan *logline.LogLine, 4)}
	h.Register(c)

	deadline := time.Now().Add(2 * time.Second)
	for h.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.ClientCount() != 1 {
		t.Fatal("client was not registered")
	}

	h.Broadcast(&logline.LogLine{Source: "s", Level: "info", Message: "hi"})

	select {
	case got := <-c.Send:
		if got.Message != "hi" {
			t.Fatalf("expected 'hi', got %q", got.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast was not received")
	}
}

func TestHub_FilteredBySource(t *testing.T) {
	h := NewHub()
	go h.Run()
	defer h.Stop()

	c := &Client{Send: make(chan *logline.LogLine, 4), SourceName: "other"}
	h.Register(c)

	deadline := time.Now().Add(2 * time.Second)
	for h.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	h.Broadcast(&logline.LogLine{Source: "s", Level: "info", Message: "hi"})

	select {
	case got := <-c.Send:
		t.Fatalf("client for source 'other' should not receive lines from 's', got %q", got.Message)
	case <-time.After(300 * time.Millisecond):
	}
}
