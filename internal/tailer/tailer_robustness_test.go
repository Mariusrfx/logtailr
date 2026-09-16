package tailer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"logtailr/internal/health"
	"logtailr/pkg/logline"
)

func TestSendErr_DeliversWhenAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan error, 1)

	sendErr(ctx, ch, errors.New("boom"))

	select {
	case err := <-ch:
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected 'boom', got %v", err)
		}
	default:
		t.Fatal("error was not delivered")
	}
}

func TestSendErr_DoesNotBlockWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := make(chan error) // unbuffered, no receiver

	start := time.Now()
	sendErr(ctx, ch, errors.New("boom"))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("sendErr blocked for %v after ctx cancel", elapsed)
	}
}

// installFakeBinary puts a shell script named `name` on PATH for the test,
// so exec.CommandContext(name, ...) runs the script instead of the real tool.
func installFakeBinary(t *testing.T, name, script string) {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDockerTailer_FollowFalseReadsOnce(t *testing.T) {
	installFakeBinary(t, "docker", "#!/bin/sh\necho \"hello one\"\necho \"hello two\"\n")

	dt, err := NewDockerTailer("fake", false, health.NewMonitor())
	if err != nil {
		t.Fatal(err)
	}

	out := make(chan *logline.LogLine, 10)
	errChan := make(chan error, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dt.Start(ctx, out, errChan)

	var got []string
	for i := 0; i < 2; i++ {
		select {
		case ll := <-out:
			got = append(got, ll.Message)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for line %d", i+1)
		}
	}

	// Wait past the reconnect backoff (~1s): the buggy code re-emitted the
	// whole log here.
	select {
	case ll := <-out:
		t.Fatalf("unexpected re-emitted line after one-shot read: %q", ll.Message)
	case <-time.After(1500 * time.Millisecond):
	}

	if got[0] != "hello one" || got[1] != "hello two" {
		t.Fatalf("unexpected lines: %v", got)
	}
}

func TestKubernetesTailer_FollowFalseReadsOnce(t *testing.T) {
	installFakeBinary(t, "kubectl", "#!/bin/sh\necho \"pod line one\"\necho \"pod line two\"\n")

	kt, err := NewKubernetesTailer("default", "fake-pod", "", "", "", false, health.NewMonitor())
	if err != nil {
		t.Fatal(err)
	}

	out := make(chan *logline.LogLine, 10)
	errChan := make(chan error, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kt.Start(ctx, out, errChan)

	var got []string
	for i := 0; i < 2; i++ {
		select {
		case ll := <-out:
			got = append(got, ll.Message)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for line %d", i+1)
		}
	}

	select {
	case ll := <-out:
		t.Fatalf("unexpected re-emitted line after one-shot read: %q", ll.Message)
	case <-time.After(1500 * time.Millisecond):
	}

	if got[0] != "pod line one" || got[1] != "pod line two" {
		t.Fatalf("unexpected lines: %v", got)
	}
}

func TestFileTailer_CopyTruncateResyncs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("line one\nline two\nline three\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ft := NewFileTailer(path, true, health.NewMonitor())
	out := make(chan *logline.LogLine, 20)
	errChan := make(chan error, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ft.Start(ctx, out, errChan)

	for i := 0; i < 3; i++ {
		select {
		case <-out:
		case err := <-errChan:
			t.Fatalf("unexpected error: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for initial line %d", i+1)
		}
	}

	// copytruncate: truncate the file to zero, then append new data.
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line four\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case ll := <-out:
		if ll.Message != "line four" {
			t.Fatalf("expected 'line four' after truncation, got %q", ll.Message)
		}
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("line written after copytruncate was lost")
	}
}
