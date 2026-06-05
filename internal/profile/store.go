package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("profile not found")
	ErrExists   = errors.New("profile already exists")
)

type Record struct {
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"created_at"`
	CookiesImported bool      `json:"cookies_imported"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(metaPath string) *Store {
	return &Store{path: metaPath}
}

func (s *Store) load() ([]Record, error) {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create profiles dir: %w", err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	return records, nil
}

func (s *Store) save(records []Record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (s *Store) Create(name string, cookiesImported bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.load()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Name == name {
			return ErrExists
		}
	}
	records = append(records, Record{
		Name:            name,
		CreatedAt:       time.Now().UTC(),
		CookiesImported: cookiesImported,
	})
	return s.save(records)
}

func (s *Store) List() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.load()
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []Record{}
	}
	return records, nil
}

func (s *Store) Get(name string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.load()
	if err != nil {
		return Record{}, err
	}
	for _, r := range records {
		if r.Name == name {
			return r, nil
		}
	}
	return Record{}, ErrNotFound
}

func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i, r := range records {
		if r.Name == name {
			records = append(records[:i], records[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	return s.save(records)
}
