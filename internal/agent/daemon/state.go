package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const stateFileName = "agent_state.json"

// State holds the persisted identity for a running daemon.
type State struct {
	AgentID   string `json:"agent_id"`
	PollToken string `json:"poll_token"`
	MachineID string `json:"machine_id"`
}

// LoadState reads State from <dir>/agent_state.json.
// Returns (nil, nil) when the file does not exist yet.
func LoadState(dir string) (*State, error) {
	path := filepath.Join(dir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the state as JSON to <dir>/agent_state.json (atomic via temp+rename).
func (s *State) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFileName+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stateFileName))
}
