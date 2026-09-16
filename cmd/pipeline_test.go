package cmd

import (
	"context"
	"testing"
	"time"

	"logtailr/internal/filter"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
)

func mkPipelineLine(level, msg string) *logline.LogLine {
	return &logline.LogLine{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Source:    "test",
		Fields:    map[string]interface{}{},
	}
}

func TestPipeline_DrainsInFlightLinesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logChan := make(chan *logline.LogLine, 10)
	errChan := make(chan error, 10)
	stopped := make(chan struct{})

	rec := &recordingWriter{}
	mon := health.NewMonitor()
	regexFilter, err := filter.NewRegexFilter("")
	if err != nil {
		t.Fatal(err)
	}

	prevLevel := level
	level = "debug"
	t.Cleanup(func() { level = prevLevel })

	logChan <- mkPipelineLine("info", "in flight")
	cancel()
	close(stopped)

	done := make(chan error, 1)
	go func() {
		done <- runPipeline(ctx, logChan, errChan, regexFilter, rec, mon, nil, nil, nil, stopped)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pipeline returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not return after drain")
	}

	if got := rec.count(); got != 1 {
		t.Fatalf("expected the in-flight line to be drained, got %d lines", got)
	}
}

func TestPipeline_FiltersAndWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logChan := make(chan *logline.LogLine, 10)
	errChan := make(chan error, 10)
	stopped := make(chan struct{})

	rec := &recordingWriter{}
	mon := health.NewMonitor()
	regexFilter, err := filter.NewRegexFilter("boom")
	if err != nil {
		t.Fatal(err)
	}

	prevLevel, prevParser := level, parserFlag
	level, parserFlag = "info", ""
	t.Cleanup(func() { level, parserFlag = prevLevel, prevParser })

	done := make(chan error, 1)
	go func() {
		done <- runPipeline(ctx, logChan, errChan, regexFilter, rec, mon, nil, nil, nil, stopped)
	}()

	// The text parser consumes the first word as the level, so the level
	// word is included in the messages below.
	logChan <- mkPipelineLine("error", "error boom happened")
	logChan <- mkPipelineLine("error", "error quiet event") // filtered out by regex
	logChan <- mkPipelineLine("debug", "debug boom too")    // filtered out by level

	waitFor(t, func() bool { return rec.count() == 1 }, 5*time.Second)

	cancel()
	close(stopped)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not return after drain")
	}

	if got := rec.count(); got != 1 {
		t.Fatalf("expected exactly 1 written line, got %d", got)
	}
}
