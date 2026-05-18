package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var (
	ErrNotFound   = errors.New("session not found")
	ErrInvalidSID = errors.New("invalid session id")
)

type Record struct {
	SID        string    `json:"sid"`
	Name       string    `json:"name,omitempty"`
	URL        string    `json:"url"`
	Title      string    `json:"title,omitempty"`
	BrowserPID int       `json:"browser_pid"`
	DebugURL   string    `json:"debug_url"`
	Headless   bool      `json:"headless"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Registry struct {
	dir string
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

func (r *Registry) Save(record Record) error {
	if err := validateSID(record.SID); err != nil {
		return err
	}

	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(r.dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := r.path(record.SID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (r *Registry) Get(sid string) (Record, error) {
	if err := validateSID(sid); err != nil {
		return Record{}, err
	}

	data, err := os.ReadFile(r.path(sid))
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r *Registry) List() ([]Record, error) {
	entries, err := os.ReadDir(r.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(r.dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	return records, nil
}

func (r *Registry) Delete(sid string) error {
	if err := validateSID(sid); err != nil {
		return err
	}

	if err := os.Remove(r.path(sid)); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

func (r *Registry) path(sid string) string {
	return filepath.Join(r.dir, sid+".json")
}

func validateSID(sid string) error {
	if sid == "" || sid != filepath.Base(sid) {
		return ErrInvalidSID
	}
	return nil
}
