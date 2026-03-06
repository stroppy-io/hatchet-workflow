package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
	"google.golang.org/protobuf/encoding/protojson"
)

type SettingsStore struct {
	path string
	mu   sync.RWMutex
}

func NewSettingsStore(path string) *SettingsStore {
	return &SettingsStore{path: path}
}

func (s *SettingsStore) Load() (*settings.Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &settings.Settings{}, nil
		}
		return nil, err
	}

	if !json.Valid(data) {
		return &settings.Settings{}, nil
	}

	var cfg settings.Settings
	if err := protojson.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *SettingsStore) Save(cfg *settings.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := protojson.MarshalOptions{Indent: "  "}.Marshal(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
