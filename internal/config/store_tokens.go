package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type persistedAccountToken struct {
	AccountID string `json:"account_id"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (s *Store) loadPersistedAccountTokens() error {
	if s == nil || !s.accountTokenPersistenceEnabled() {
		return nil
	}
	for i := range s.cfg.Accounts {
		accountID := s.cfg.Accounts[i].Identifier()
		if strings.TrimSpace(accountID) == "" {
			continue
		}
		item, err := s.loadPersistedAccountToken(accountID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(item.Token) != "" {
			s.cfg.Accounts[i].Token = strings.TrimSpace(item.Token)
		}
	}
	return nil
}

func (s *Store) syncPersistedAccountTokens() error {
	if s == nil || !s.accountTokenPersistenceEnabled() {
		return nil
	}
	for _, acc := range s.cfg.Accounts {
		accountID := acc.Identifier()
		if strings.TrimSpace(accountID) == "" {
			continue
		}
		if err := s.persistAccountToken(accountID, acc.Token); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadPersistedAccountToken(accountID string) (persistedAccountToken, error) {
	var item persistedAccountToken
	data, err := os.ReadFile(s.accountTokenPath(accountID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return item, nil
		}
		return item, fmt.Errorf("read account token: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return item, nil
	}
	if err := json.Unmarshal(data, &item); err != nil {
		return item, fmt.Errorf("decode account token: %w", err)
	}
	if item.AccountID != "" && canonicalAccountTokenID(item.AccountID) != canonicalAccountTokenID(accountID) {
		return persistedAccountToken{}, nil
	}
	return item, nil
}

func (s *Store) persistAccountToken(accountID, token string) error {
	accountID = strings.TrimSpace(accountID)
	token = strings.TrimSpace(token)
	if s == nil || !s.accountTokenPersistenceEnabled() || accountID == "" {
		return nil
	}
	path := s.accountTokenPath(accountID)
	if token == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove account token: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(s.tokenDir, 0o755); err != nil {
		return fmt.Errorf("mkdir account tokens dir: %w", err)
	}
	data, err := json.MarshalIndent(persistedAccountToken{
		AccountID: accountID,
		Token:     token,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode account token: %w", err)
	}
	tmp, err := os.CreateTemp(s.tokenDir, ".account-token-*.tmp")
	if err != nil {
		return fmt.Errorf("create account token temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		closeErr := tmp.Close()
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("write account token temp file: %w", errors.Join(err, closeErr, removeErr))
	}
	if err := tmp.Close(); err != nil {
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("close account token temp file: %w", errors.Join(err, removeErr))
	}
	if err := os.Rename(tmpPath, path); err != nil {
		removeErr := os.Remove(tmpPath)
		return fmt.Errorf("replace account token file: %w", errors.Join(err, removeErr))
	}
	return nil
}

func (s *Store) syncPersistedAccountTokensBestEffort() {
	if err := s.syncPersistedAccountTokens(); err != nil {
		Logger.Warn("[account_tokens] sync failed", "dir", s.tokenDir, "error", err)
	}
}

func (s *Store) persistAccountTokenBestEffort(accountID, token string) {
	if err := s.persistAccountToken(accountID, token); err != nil {
		Logger.Warn("[account_tokens] persist failed", "account", accountID, "error", err)
	}
}

func (s *Store) accountTokenPersistenceEnabled() bool {
	if strings.TrimSpace(s.tokenDir) == "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("DS2API_ACCOUNT_TOKENS_DIR")) != "" {
		return true
	}
	return !s.fromEnv
}

func (s *Store) accountTokenPath(accountID string) string {
	return filepath.Join(s.tokenDir, accountTokenFileName(accountID))
}

func accountTokenFileName(accountID string) string {
	sum := sha256.Sum256([]byte(canonicalAccountTokenID(accountID)))
	return hex.EncodeToString(sum[:]) + ".json"
}

func canonicalAccountTokenID(accountID string) string {
	return strings.ToLower(strings.TrimSpace(accountID))
}
