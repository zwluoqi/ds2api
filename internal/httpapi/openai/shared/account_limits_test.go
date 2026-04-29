package shared

import (
	"testing"

	"ds2api/internal/accountstats"
	"ds2api/internal/config"
)

func TestAccountWithinTotalLimits(t *testing.T) {
	stats := accountstats.New(t.TempDir())
	acc := config.Account{
		Email:           "limited@example.com",
		TotalFlashLimit: 1,
		TotalProLimit:   2,
	}
	if !AccountWithinTotalLimits(stats, acc, "deepseek-v4-flash") {
		t.Fatal("expected flash account to be available before reaching limit")
	}
	if err := stats.Record(acc.Identifier(), "deepseek-v4-flash"); err != nil {
		t.Fatalf("record flash: %v", err)
	}
	if AccountWithinTotalLimits(stats, acc, "deepseek-v4-flash") {
		t.Fatal("expected flash account to be unavailable after reaching limit")
	}
	if !AccountWithinTotalLimits(stats, acc, "deepseek-v4-pro") {
		t.Fatal("expected pro account to remain available below pro limit")
	}
	if err := stats.Record(acc.Identifier(), "deepseek-v4-pro"); err != nil {
		t.Fatalf("record pro 1: %v", err)
	}
	if err := stats.Record(acc.Identifier(), "deepseek-v4-pro"); err != nil {
		t.Fatalf("record pro 2: %v", err)
	}
	if AccountWithinTotalLimits(stats, acc, "deepseek-v4-pro") {
		t.Fatal("expected pro account to be unavailable after reaching limit")
	}
}
