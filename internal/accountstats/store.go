package accountstats

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Counts struct {
	Flash int64 `json:"flash"`
	Pro   int64 `json:"pro"`
	Total int64 `json:"total"`
}

type AccountStats struct {
	AccountID string            `json:"account_id"`
	Daily     map[string]Counts `json:"daily"`
	Total     Counts            `json:"total"`
	UpdatedAt string            `json:"updated_at,omitempty"`
}

type Summary struct {
	DailyFlashRequests int64 `json:"daily_flash_requests"`
	DailyProRequests   int64 `json:"daily_pro_requests"`
	DailyRequests      int64 `json:"daily_requests"`
	TotalFlashRequests int64 `json:"total_flash_requests"`
	TotalProRequests   int64 `json:"total_pro_requests"`
	TotalRequests      int64 `json:"total_requests"`
}

type Store struct {
	dir string
	now func() time.Time

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func New(dir string) *Store {
	return &Store{
		dir:   strings.TrimSpace(dir),
		now:   time.Now,
		locks: map[string]*sync.Mutex{},
	}
}

func (s *Store) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

func (s *Store) Record(accountID, model string) error {
	accountID = strings.TrimSpace(accountID)
	if s == nil || accountID == "" || strings.TrimSpace(s.dir) == "" {
		return nil
	}
	lock := s.lockFor(accountID)
	lock.Lock()
	defer lock.Unlock()

	stats, err := s.load(accountID)
	if err != nil {
		return err
	}
	if stats.AccountID == "" {
		stats.AccountID = accountID
	}
	if stats.Daily == nil {
		stats.Daily = map[string]Counts{}
	}
	day := s.now().Format("2006-01-02")
	daily := stats.Daily[day]
	incrementCounts(&daily, model)
	incrementCounts(&stats.Total, model)
	stats.Daily[day] = daily
	stats.UpdatedAt = s.now().Format(time.RFC3339)
	return s.save(accountID, stats)
}

func (s *Store) Summary(accountID string) Summary {
	accountID = strings.TrimSpace(accountID)
	if s == nil || accountID == "" || strings.TrimSpace(s.dir) == "" {
		return Summary{}
	}
	lock := s.lockFor(accountID)
	lock.Lock()
	defer lock.Unlock()

	stats, err := s.load(accountID)
	if err != nil {
		return Summary{}
	}
	daily := stats.Daily[s.now().Format("2006-01-02")]
	return Summary{
		DailyFlashRequests: daily.Flash,
		DailyProRequests:   daily.Pro,
		DailyRequests:      daily.Total,
		TotalFlashRequests: stats.Total.Flash,
		TotalProRequests:   stats.Total.Pro,
		TotalRequests:      stats.Total.Total,
	}
}

func (s *Store) SummaryAccounts(accountIDs []string) Summary {
	var total Summary
	for _, accountID := range accountIDs {
		summary := s.Summary(accountID)
		total.DailyFlashRequests += summary.DailyFlashRequests
		total.DailyProRequests += summary.DailyProRequests
		total.DailyRequests += summary.DailyRequests
		total.TotalFlashRequests += summary.TotalFlashRequests
		total.TotalProRequests += summary.TotalProRequests
		total.TotalRequests += summary.TotalRequests
	}
	return total
}

func (s *Store) Path(accountID string) string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.dir, accountFileName(accountID))
}

func (s *Store) lockFor(accountID string) *sync.Mutex {
	key := canonicalAccountID(accountID)
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[key] = lock
	}
	return lock
}

func (s *Store) load(accountID string) (AccountStats, error) {
	var stats AccountStats
	data, err := os.ReadFile(s.Path(accountID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AccountStats{AccountID: strings.TrimSpace(accountID), Daily: map[string]Counts{}}, nil
		}
		return stats, fmt.Errorf("read account stats: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return AccountStats{AccountID: strings.TrimSpace(accountID), Daily: map[string]Counts{}}, nil
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		return stats, fmt.Errorf("decode account stats: %w", err)
	}
	if stats.Daily == nil {
		stats.Daily = map[string]Counts{}
	}
	return stats, nil
}

func (s *Store) save(accountID string, stats AccountStats) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir account stats dir: %w", err)
	}
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("encode account stats: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, ".account-stats-*.tmp")
	if err != nil {
		return fmt.Errorf("create account stats temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		closeErr := tmp.Close()
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("write account stats temp file: %w", errors.Join(err, closeErr, removeErr))
	}
	if err := tmp.Close(); err != nil {
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("close account stats temp file: %w", errors.Join(err, removeErr))
	}
	if err := os.Rename(tmpPath, s.Path(accountID)); err != nil {
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("replace account stats file: %w", errors.Join(err, removeErr))
	}
	return nil
}

func incrementCounts(counts *Counts, model string) {
	counts.Total++
	switch ModelFamily(model) {
	case "flash":
		counts.Flash++
	case "pro":
		counts.Pro++
	}
}

func ModelFamily(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(model, "flash"):
		return "flash"
	case strings.Contains(model, "pro"):
		return "pro"
	default:
		return ""
	}
}

func accountFileName(accountID string) string {
	sum := sha256.Sum256([]byte(canonicalAccountID(accountID)))
	return hex.EncodeToString(sum[:]) + ".json"
}

func canonicalAccountID(accountID string) string {
	return strings.ToLower(strings.TrimSpace(accountID))
}
