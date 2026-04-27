package config

import (
	"fmt"
	"strings"
)

const (
	AccountSelectionTokenFirst = "token_first"
	AccountSelectionRoundRobin = "round_robin"
)

func NormalizeAccountSelectionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "token_first", "token-first", "current", "fill", "filled":
		return AccountSelectionTokenFirst
	case "round_robin", "round-robin", "roundrobin", "rotation", "polling":
		return AccountSelectionRoundRobin
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func ValidateAccountSelectionMode(mode string) error {
	normalized := NormalizeAccountSelectionMode(mode)
	if normalized == AccountSelectionTokenFirst || normalized == AccountSelectionRoundRobin {
		return nil
	}
	return fmt.Errorf("runtime.account_selection_mode must be token_first or round_robin")
}
