package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/harris/gemini-web-cli/internal/config"
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "gemini-web-cli"
	accountsFile   = "accounts.json"
)

// AccountCookies holds the cookies for a single account.
type AccountCookies struct {
	PSID    string `json:"__Secure-1PSID"`
	PSIDTS  string `json:"__Secure-1PSIDTS"`
	PSIDCC  string `json:"__Secure-1PSIDCC"`
	NID     string `json:"NID,omitempty"`
	Account string `json:"account"`
}

// AccountList tracks all known accounts.
type AccountList struct {
	Default  string   `json:"default"`
	Accounts []string `json:"accounts"`
}

// Store manages secure cookie storage.
type Store struct {
	useKeyring bool
	dir        string
}

// NewStore creates a new credential store.
func NewStore() (*Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	// Test if keyring is available
	s.useKeyring = testKeyring()
	return s, nil
}

func testKeyring() bool {
	err := keyring.Set(keyringService, "__test__", "test")
	if err != nil {
		return false
	}
	_ = keyring.Delete(keyringService, "__test__")
	return true
}

// SaveCookies stores cookies for an account.
func (s *Store) SaveCookies(cookies *AccountCookies) error {
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	if s.useKeyring {
		return keyring.Set(keyringService, cookies.Account, string(data))
	}
	return s.saveToFile(cookies.Account, data)
}

// LoadCookies retrieves cookies for an account.
func (s *Store) LoadCookies(account string) (*AccountCookies, error) {
	var data []byte
	if s.useKeyring {
		val, err := keyring.Get(keyringService, account)
		if err != nil {
			return nil, fmt.Errorf("account %q not found: %w", account, err)
		}
		data = []byte(val)
	} else {
		var err error
		data, err = s.loadFromFile(account)
		if err != nil {
			return nil, err
		}
	}
	var cookies AccountCookies
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, err
	}
	return &cookies, nil
}

// DeleteCookies removes cookies for an account.
func (s *Store) DeleteCookies(account string) error {
	if s.useKeyring {
		return keyring.Delete(keyringService, account)
	}
	path := filepath.Join(s.dir, "credentials", account+".enc")
	return os.Remove(path)
}

// ListAccounts returns the account list.
func (s *Store) ListAccounts() (*AccountList, error) {
	path := filepath.Join(s.dir, accountsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AccountList{}, nil
		}
		return nil, err
	}
	var list AccountList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// SaveAccountList persists the account list.
func (s *Store) SaveAccountList(list *AccountList) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, accountsFile), data, 0600)
}

// AddAccount registers an account in the list.
func (s *Store) AddAccount(account string) error {
	list, err := s.ListAccounts()
	if err != nil {
		return err
	}
	for _, a := range list.Accounts {
		if a == account {
			return nil // already exists
		}
	}
	list.Accounts = append(list.Accounts, account)
	if list.Default == "" {
		list.Default = account
	}
	return s.SaveAccountList(list)
}

// RemoveAccount removes an account from the list and deletes its cookies.
func (s *Store) RemoveAccount(account string) error {
	list, err := s.ListAccounts()
	if err != nil {
		return err
	}
	filtered := list.Accounts[:0]
	for _, a := range list.Accounts {
		if a != account {
			filtered = append(filtered, a)
		}
	}
	list.Accounts = filtered
	if list.Default == account {
		list.Default = ""
		if len(list.Accounts) > 0 {
			list.Default = list.Accounts[0]
		}
	}
	_ = s.DeleteCookies(account)
	return s.SaveAccountList(list)
}

// SetDefault sets the default account.
func (s *Store) SetDefault(account string) error {
	list, err := s.ListAccounts()
	if err != nil {
		return err
	}
	found := false
	for _, a := range list.Accounts {
		if a == account {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("account %q not found", account)
	}
	list.Default = account
	return s.SaveAccountList(list)
}

// --- File-based encrypted storage fallback ---

func (s *Store) encryptionKeyPath() string {
	return filepath.Join(s.dir, "credentials", ".key")
}

func (s *Store) getOrCreateKey() ([]byte, error) {
	credDir := filepath.Join(s.dir, "credentials")
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return nil, err
	}
	keyPath := s.encryptionKeyPath()
	key, err := os.ReadFile(keyPath)
	if err == nil && len(key) == 32 {
		return key, nil
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Store) saveToFile(account string, plaintext []byte) error {
	key, err := s.getOrCreateKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	path := filepath.Join(s.dir, "credentials", account+".enc")
	return os.WriteFile(path, ciphertext, 0600)
}

func (s *Store) loadFromFile(account string) ([]byte, error) {
	key, err := s.getOrCreateKey()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir, "credentials", account+".enc")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("account %q not found: %w", account, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
