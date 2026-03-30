package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testStore creates a Store from TEST_DATABASE_URL and runs migrations.
// Skips the test if the env var is not set.
func testStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping store integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, err := New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	if err := st.RunMigrations(dbURL); err != nil {
		st.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		// Clean up test data
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM alert_events`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM alert_rules`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM sources`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM outputs`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM settings`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM saved_searches`)
		_, _ = st.Pool.Exec(cleanCtx, `DELETE FROM bookmarks`)
		st.Close()
	})

	return st
}

// --- Sources ---

func TestSourcesCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	// Create
	src := &SourceRow{
		Name:   "test-source",
		Type:   "file",
		Path:   "/var/log/test.log",
		Follow: true,
		Parser: "json",
	}
	if err := st.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if !src.ID.Valid {
		t.Fatal("CreateSource did not return an ID")
	}

	// Get by ID
	got, err := st.GetSourceByID(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSourceByID: %v", err)
	}
	if got.Name != "test-source" {
		t.Errorf("Name = %q, want %q", got.Name, "test-source")
	}
	if got.Path != "/var/log/test.log" {
		t.Errorf("Path = %q, want %q", got.Path, "/var/log/test.log")
	}

	// Get by name
	got2, err := st.GetSourceByName(ctx, "test-source")
	if err != nil {
		t.Fatalf("GetSourceByName: %v", err)
	}
	if got2.ID != src.ID {
		t.Error("GetSourceByName returned different ID")
	}

	// List
	list, err := st.ListSources(ctx)
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	found := false
	for _, s := range list {
		if s.Name == "test-source" {
			found = true
		}
	}
	if !found {
		t.Error("ListSources did not include created source")
	}

	// Update
	src.Parser = "logfmt"
	if err := st.UpdateSource(ctx, src); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}
	updated, _ := st.GetSourceByID(ctx, src.ID)
	if updated.Parser != "logfmt" {
		t.Errorf("Parser after update = %q, want %q", updated.Parser, "logfmt")
	}

	// Delete
	if err := st.DeleteSource(ctx, src.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	_, err = st.GetSourceByID(ctx, src.ID)
	if err == nil {
		t.Error("GetSourceByID after delete should fail")
	}
}

// --- Outputs ---

func TestOutputsCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	cfg := json.RawMessage(`{"url":"https://example.com/hook"}`)
	out := &OutputRow{
		Name:    "test-output",
		Type:    "webhook",
		Config:  cfg,
		Enabled: true,
	}
	if err := st.CreateOutput(ctx, out); err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	got, err := st.GetOutputByID(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetOutputByID: %v", err)
	}
	if got.Name != "test-output" {
		t.Errorf("Name = %q, want %q", got.Name, "test-output")
	}

	list, err := st.ListOutputs(ctx)
	if err != nil {
		t.Fatalf("ListOutputs: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListOutputs returned empty")
	}

	out.Enabled = false
	if err := st.UpdateOutput(ctx, out); err != nil {
		t.Fatalf("UpdateOutput: %v", err)
	}
	updated, _ := st.GetOutputByID(ctx, out.ID)
	if updated.Enabled {
		t.Error("Enabled should be false after update")
	}

	if err := st.DeleteOutput(ctx, out.ID); err != nil {
		t.Fatalf("DeleteOutput: %v", err)
	}
}

// --- Alert Rules ---

func TestAlertRulesCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	rule := &AlertRuleRow{
		Name:     "test-rule",
		Type:     "pattern",
		Severity: "critical",
		Pattern:  "OutOfMemory",
		Enabled:  true,
	}
	if err := st.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("CreateAlertRule: %v", err)
	}

	got, err := st.GetAlertRuleByID(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetAlertRuleByID: %v", err)
	}
	if got.Pattern != "OutOfMemory" {
		t.Errorf("Pattern = %q, want %q", got.Pattern, "OutOfMemory")
	}

	list, err := st.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListAlertRules returned empty")
	}

	rule.Severity = "warning"
	if err := st.UpdateAlertRule(ctx, rule); err != nil {
		t.Fatalf("UpdateAlertRule: %v", err)
	}

	if err := st.DeleteAlertRule(ctx, rule.ID); err != nil {
		t.Fatalf("DeleteAlertRule: %v", err)
	}
}

// --- Alert Events ---

func TestAlertEventsCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	event := &AlertEventRow{
		RuleName: "test-rule",
		Severity: "critical",
		Message:  "test alert",
		Source:   "app.log",
		Count:    1,
		FiredAt:  time.Now(),
	}
	if err := st.CreateAlertEvent(ctx, event); err != nil {
		t.Fatalf("CreateAlertEvent: %v", err)
	}

	events, err := st.ListAlertEvents(ctx, AlertEventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListAlertEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("ListAlertEvents returned empty")
	}

	// Filter by severity
	events, err = st.ListAlertEvents(ctx, AlertEventFilter{Severity: "critical", Limit: 10})
	if err != nil {
		t.Fatalf("ListAlertEvents with filter: %v", err)
	}
	if len(events) == 0 {
		t.Error("ListAlertEvents with severity filter returned empty")
	}

	// Acknowledge
	if err := st.AcknowledgeAlertEvent(ctx, event.ID); err != nil {
		t.Fatalf("AcknowledgeAlertEvent: %v", err)
	}

	// Delete old
	deleted, err := st.DeleteAlertEventsOlderThan(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteAlertEventsOlderThan: %v", err)
	}
	if deleted == 0 {
		t.Error("DeleteAlertEventsOlderThan should have deleted at least 1 event")
	}
}

// --- Settings ---

func TestSettingsCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	val := json.RawMessage(`"info"`)
	if err := st.SetSetting(ctx, "global.level", val); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	got, err := st.GetSetting(ctx, "global.level")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}

	var s string
	if err := json.Unmarshal(got, &s); err != nil {
		t.Fatalf("Unmarshal setting: %v", err)
	}
	if s != "info" {
		t.Errorf("Setting = %q, want %q", s, "info")
	}

	// Overwrite
	val2 := json.RawMessage(`"error"`)
	if err := st.SetSetting(ctx, "global.level", val2); err != nil {
		t.Fatalf("SetSetting overwrite: %v", err)
	}
	got2, _ := st.GetSetting(ctx, "global.level")
	var s2 string
	_ = json.Unmarshal(got2, &s2)
	if s2 != "error" {
		t.Errorf("Setting after overwrite = %q, want %q", s2, "error")
	}
}

// --- Saved Searches ---

func TestSavedSearchesCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	ss := &SavedSearchRow{
		Name:    "test-search",
		Filters: json.RawMessage(`{"level":"error","regex":"timeout"}`),
	}
	if err := st.CreateSavedSearch(ctx, ss); err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	got, err := st.GetSavedSearchByID(ctx, ss.ID)
	if err != nil {
		t.Fatalf("GetSavedSearchByID: %v", err)
	}
	if got.Name != "test-search" {
		t.Errorf("Name = %q, want %q", got.Name, "test-search")
	}

	list, err := st.ListSavedSearches(ctx)
	if err != nil {
		t.Fatalf("ListSavedSearches: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListSavedSearches returned empty")
	}

	ss.Name = "updated-search"
	if err := st.UpdateSavedSearch(ctx, ss); err != nil {
		t.Fatalf("UpdateSavedSearch: %v", err)
	}

	if err := st.DeleteSavedSearch(ctx, ss.ID); err != nil {
		t.Fatalf("DeleteSavedSearch: %v", err)
	}
}

// --- Bookmarks ---

func TestBookmarksCRUD(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	bm := &BookmarkRow{
		Name:    "test-bookmark",
		File:    "/var/log/app.log",
		Offset:  12345,
		Inode:   67890,
		SavedAt: time.Now(),
	}
	if err := st.SaveBookmark(ctx, bm); err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	got, err := st.LoadBookmark(ctx, "test-bookmark")
	if err != nil {
		t.Fatalf("LoadBookmark: %v", err)
	}
	if got == nil {
		t.Fatal("LoadBookmark returned nil")
	}
	if got.Offset != 12345 {
		t.Errorf("Offset = %d, want %d", got.Offset, 12345)
	}

	list, err := st.ListBookmarks(ctx)
	if err != nil {
		t.Fatalf("ListBookmarks: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListBookmarks returned empty")
	}

	// Update
	bm.Offset = 99999
	if err := st.SaveBookmark(ctx, bm); err != nil {
		t.Fatalf("SaveBookmark update: %v", err)
	}
	got2, err := st.LoadBookmark(ctx, "test-bookmark")
	if err != nil {
		t.Fatalf("LoadBookmark after update: %v", err)
	}
	if got2 == nil {
		t.Fatal("LoadBookmark after update returned nil")
	}
	if got2.Offset != 99999 {
		t.Errorf("Offset after update = %d, want %d", got2.Offset, 99999)
	}

	if err := st.DeleteBookmark(ctx, "test-bookmark"); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}
}
