package shared

import (
	"strings"

	"ds2api/internal/accountstats"
	"ds2api/internal/config"
)

func AccountWithinTotalLimits(stats *accountstats.Store, acc config.Account, model string) bool {
	if stats == nil {
		return true
	}
	id := strings.TrimSpace(acc.Identifier())
	if id == "" {
		return true
	}
	summary := stats.Summary(id)
	switch accountstats.ModelFamily(model) {
	case "flash":
		return acc.TotalFlashLimit <= 0 || summary.TotalFlashRequests < acc.TotalFlashLimit
	case "pro":
		return acc.TotalProLimit <= 0 || summary.TotalProRequests < acc.TotalProLimit
	default:
		return true
	}
}
