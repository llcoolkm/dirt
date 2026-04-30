package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State is dirt's runtime preferences — values that change as the
// master uses the TUI (sort column, theme override, column toggles,
// mark behaviour). Persisted by `:save` to ~/.local/state/dirt/state.yaml
// (XDG_STATE_HOME convention) so config.yaml and any comments the
// master added there remain pristine.
//
// Pointer fields distinguish "not set" from a zero value, so a
// state file that doesn't mention a field doesn't quietly clobber
// what config.yaml said. Only fields whose pointers are non-nil
// override the corresponding config field at startup.
type State struct {
	Theme       string          `yaml:"theme,omitempty"`
	SortBy      string          `yaml:"sort_by,omitempty"`
	SortReverse *bool           `yaml:"sort_reverse,omitempty"`
	MarkAdvance string          `yaml:"mark_advance,omitempty"`
	Columns     map[string]bool `yaml:"columns,omitempty"`
}

// StatePath returns the absolute path of the state file, honouring
// $XDG_STATE_HOME when set and falling back to ~/.local/state otherwise.
func StatePath() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "dirt", "state.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".local", "state", "dirt", "state.yaml")
	}
	return filepath.Join(home, ".local", "state", "dirt", "state.yaml")
}

// LoadState reads the state file. A missing file is not an error —
// it returns a zero State.
func LoadState() (State, error) {
	path := StatePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var s State
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return State{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// SaveState writes s atomically. Creates parent directories with 0700.
func SaveState(s State) error {
	path := StatePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	var buf bytes.Buffer
	buf.WriteString("# dirt runtime state — written by :save / :w / :wq.\n")
	buf.WriteString("# Hand-editing is fine; values here override config.yaml at startup.\n\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "state.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// MergeInto overlays s onto cfg in place. Only fields the state file
// explicitly set (non-empty strings, non-nil pointers, non-nil maps)
// override cfg. The Columns map is overlaid key-by-key so a sparse
// state file can flip individual columns without nuking the rest.
func (s State) MergeInto(cfg *Config) {
	if cfg == nil {
		return
	}
	if s.Theme != "" {
		cfg.Theme = s.Theme
	}
	if s.SortBy != "" {
		cfg.List.SortBy = s.SortBy
	}
	if s.SortReverse != nil {
		cfg.List.SortReverse = *s.SortReverse
	}
	if s.MarkAdvance != "" {
		cfg.List.MarkAdvance = s.MarkAdvance
	}
	if s.Columns != nil {
		if cfg.List.Columns == nil {
			cfg.List.Columns = make(map[string]bool, len(s.Columns))
		}
		for k, v := range s.Columns {
			cfg.List.Columns[k] = v
		}
	}
}
