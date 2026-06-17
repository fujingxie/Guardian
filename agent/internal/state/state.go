package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// State 是 agent 在本机持久化的状态：换回 agent_token 后的 server_id + agent_token。
// 路径默认 /var/lib/guardian/state.json，开发可用 GUARDIAN_STATE_DIR 覆盖。
type State struct {
	ServerID   string `json:"serverId"`
	AgentToken string `json:"agentToken"`
	ConsoleURL string `json:"consoleUrl"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore() *Store {
	dir := os.Getenv("GUARDIAN_STATE_DIR")
	if dir == "" {
		dir = "/var/lib/guardian"
	}
	return &Store{path: filepath.Join(dir, "state.json")}
}

func (s *Store) Load() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // 尚未 enroll
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &st, nil
}

func (s *Store) Save(st *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("mkdir state: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // 原子写入
}
