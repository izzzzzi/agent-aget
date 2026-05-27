package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/izzzzzi/agent-aget/internal/ids"
)

var (
	ErrNotFound    = errors.New("snapshot not found")
	ErrRefNotFound = errors.New("ref not found")
	ErrInvalidSID  = errors.New("invalid session id")
)

type Element struct {
	Ref      string `json:"ref"`
	Kind     string `json:"kind"`
	Text     string `json:"text,omitempty"`
	Selector string `json:"selector"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Visible  bool   `json:"visible"`
	Enabled  bool   `json:"enabled"`
}

type Record struct {
	SID       string    `json:"sid"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Elements  []Element `json:"elements"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Save(record Record) error {
	if err := validateSID(record.SID); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := s.path(record.SID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s *Store) Load(sid string) (Record, error) {
	if err := validateSID(sid); err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(s.path(sid))
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

func (s *Store) Resolve(sid, ref string) (Element, error) {
	record, err := s.Load(sid)
	if err != nil {
		return Element{}, err
	}
	for _, element := range record.Elements {
		if element.Ref == ref {
			return element, nil
		}
	}
	return Element{}, ErrRefNotFound
}

func (s *Store) path(sid string) string {
	return filepath.Join(s.dir, sid+".json")
}

func validateSID(sid string) error {
	if !ids.ValidSessionID(sid) || sid != filepath.Base(sid) {
		return ErrInvalidSID
	}
	return nil
}
