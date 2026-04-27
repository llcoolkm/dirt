package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is dirt's persistent user-level configuration. Values here
// act as defaults for Model state at startup; CLI flags and runtime
// key presses may override them for the current session.
//
// Unknown YAML fields are silently tolerated so downgrading dirt
// against a newer config file does not erase the extra fields on
// save — we re-read before any write.
type Config struct {
	// Refresh is the snapshot tick rate. Clamped to a 200ms floor
	// by the Model regardless of what this says.
	Refresh time.Duration `yaml:"refresh"`

	// Theme selects a colour palette: default, light, solarized, gruvbox.
	Theme string `yaml:"theme"`

	// List holds VM-table-specific preferences.
	List ListConfig `yaml:"list"`
}

// ListConfig holds preferences for the main VM list.
type ListConfig struct {
	// SortBy is the column to sort on at startup. Valid values match
	// the column ids used in ui.vmColumns: name, state, ip, os, vcpu,
	// mem, mem_pct, cpu, uptime. Unknown values fall back to state.
	SortBy string `yaml:"sort_by"`

	// SortReverse flips the sort direction from the column's natural
	// order. Natural order is A→Z for text columns (name, state, ip,
	// os) and largest-first for numeric columns (vcpu, mem, cpu,
	// uptime) — matching the dirt convention where you press a number
	// key once to get the most interesting VMs on top.
	SortReverse bool `yaml:"sort_reverse"`

	// Columns controls which optional columns are shown. NAME, STATE,
	// IP are required and cannot be hidden. Any column absent from the
	// map is treated as visible (true), so a fresh config doesn't
	// accidentally hide everything.
	Columns map[string]bool `yaml:"columns"`

	// MarkAdvance controls how SPACE moves the cursor after marking:
	//   - "directional" (default) — follow the last cursor direction
	//     (j/G → down, k/g → up).
	//   - "down"  — always advance down regardless of last motion.
	//   - "none"  — do not move the cursor; SPACE is a pure toggle.
	MarkAdvance string `yaml:"mark_advance"`
}

// DefaultConfig returns dirt's built-in defaults. These match the
// values seeded into a freshly-created config.yaml.
func DefaultConfig() Config {
	return Config{
		Refresh: 1 * time.Second,
		Theme:   "default",
		List: ListConfig{
			SortBy:      "state",
			SortReverse: false,
			MarkAdvance: "directional",
			Columns: map[string]bool{
				"os":       true,
				"vcpu":     true,
				"mem":      true,
				"mem_pct":  true,
				"cpu":      true,
				"uptime":   true,
				"io_r":     true,
				"io_w":     true,
				"cpu_bar":    false,
				"mem_bar":    false,
				"disk_bar":   false,
				"net_rx":     false,
				"net_tx":     false,
				"autostart":  false,
				"persistent": false,
				"arch":       false,
				"tag":        false,
			},
		},
	}
}

// ConfigPath returns the absolute path of the config file, honouring
// $XDG_CONFIG_HOME when set and falling back to ~/.config otherwise.
func ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "dirt", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "dirt", "config.yaml")
	}
	return filepath.Join(home, ".config", "dirt", "config.yaml")
}

// LoadConfig reads the config file and returns its contents merged
// with built-in defaults (so missing fields get sensible values). A
// missing file is not an error — it returns DefaultConfig() unchanged.
func LoadConfig() (Config, error) {
	path := ConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parse %s: %w", path, err)
	}

	// Fill in any field the user omitted. yaml.Unmarshal leaves the
	// destination unchanged for absent keys, but zero values slip
	// through, so we post-process the obvious ones.
	if cfg.Refresh <= 0 {
		cfg.Refresh = DefaultConfig().Refresh
	}
	if cfg.Theme == "" {
		cfg.Theme = DefaultConfig().Theme
	}
	if cfg.List.SortBy == "" {
		cfg.List.SortBy = DefaultConfig().List.SortBy
	}
	if cfg.List.Columns == nil {
		cfg.List.Columns = DefaultConfig().List.Columns
	}
	return cfg, nil
}

// SaveConfig writes cfg atomically by writing to a temp file in the
// same directory and renaming it over the target. Creates parent
// directories with 0700.
func SaveConfig(cfg Config) error {
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Render with a header comment so a hand-editing user sees what
	// the fields mean. yaml.v3 cannot attach comments directly to
	// Marshal output, so we prepend a raw banner and then the body.
	var buf bytes.Buffer
	buf.WriteString("# dirt configuration file\n")
	buf.WriteString("#\n")
	buf.WriteString("# refresh: snapshot tick rate (e.g. 1s, 500ms, 2s). Floor is 200ms.\n")
	buf.WriteString("# theme:   colour palette — one of: default, light, solarized, gruvbox\n")
	buf.WriteString("#\n")
	buf.WriteString("# list.sort_by:      one of name, state, ip, os, vcpu, mem, mem_pct, cpu, uptime\n")
	buf.WriteString("# list.sort_reverse: flip the natural sort order (A→Z / largest-first)\n")
	buf.WriteString("# list.columns:      toggle optional columns. NAME, STATE, IP are required.\n")
	buf.WriteString("\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "config.*.tmp")
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

// SeedConfigIfMissing returns the config file contents, creating the
// file from defaults if it does not exist. Any load error on an
// existing file is returned unchanged so the caller can decide
// whether to warn the user.
func SeedConfigIfMissing() (Config, error) {
	path := ConfigPath()
	if _, err := os.Stat(path); err == nil {
		return LoadConfig()
	} else if !os.IsNotExist(err) {
		return DefaultConfig(), err
	}
	cfg := DefaultConfig()
	if err := SaveConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
