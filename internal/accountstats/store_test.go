package accountstats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordSummaryAndPersistence(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	store.now = func() time.Time { return time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC) }

	if err := store.Record("user@example.com", "deepseek-v4-flash"); err != nil {
		t.Fatalf("record flash: %v", err)
	}
	if err := store.Record("user@example.com", "deepseek-v4-pro"); err != nil {
		t.Fatalf("record pro: %v", err)
	}
	if err := store.Record("user@example.com", "deepseek-v4-vision"); err != nil {
		t.Fatalf("record vision: %v", err)
	}

	got := store.Summary("user@example.com")
	if got.DailyFlashRequests != 1 || got.DailyProRequests != 1 || got.DailyRequests != 3 {
		t.Fatalf("unexpected daily summary: %#v", got)
	}
	if got.TotalFlashRequests != 1 || got.TotalProRequests != 1 || got.TotalRequests != 3 {
		t.Fatalf("unexpected total summary: %#v", got)
	}

	reloaded := New(dir)
	reloaded.now = store.now
	if got := reloaded.Summary("user@example.com"); got.TotalRequests != 3 {
		t.Fatalf("expected persisted total requests, got %#v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, accountFileName("user@example.com"))); err != nil {
		t.Fatalf("expected account stats file: %v", err)
	}
}

func TestStoreSummaryUsesCurrentDay(t *testing.T) {
	store := New(t.TempDir())
	store.now = func() time.Time { return time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC) }
	if err := store.Record("user@example.com", "deepseek-v4-flash"); err != nil {
		t.Fatalf("record: %v", err)
	}

	store.now = func() time.Time { return time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC) }
	got := store.Summary("user@example.com")
	if got.DailyRequests != 0 || got.TotalRequests != 1 {
		t.Fatalf("expected daily reset with persisted total, got %#v", got)
	}
}
