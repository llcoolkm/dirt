// Package config handles dirt's on-disk configuration.
//
// For now only the hosts list is persisted; a broader config.yaml is
// planned for later (refresh interval, colour theme, default sort, …).
// The hosts file is a simple whitespace-separated table so we do not
// need a YAML dependency to edit it from code or hand.
package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Host is a single named libvirt URI.
type Host struct {
	Name string
	URI  string
}

// HostsPath returns the absolute path of the hosts file, honouring
// $XDG_CONFIG_HOME when set and falling back to ~/.config otherwise.
func HostsPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "dirt", "hosts")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "dirt", "hosts")
	}
	return filepath.Join(home, ".config", "dirt", "hosts")
}

// LoadHosts reads the hosts file and returns its entries. A missing file
// is not an error — it returns (nil, nil). Malformed lines are skipped
// silently so a partial edit does not lock the user out; the first token
// on each non-comment, non-empty line is the name, the rest is the URI.
func LoadHosts() ([]Host, error) {
	path := HostsPath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var hosts []Host
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		uri := strings.Join(fields[1:], " ")
		hosts = append(hosts, Host{Name: name, URI: uri})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

// SaveHosts writes the list atomically by writing to a temp file in the
// same directory and renaming it over the target. Creates parent
// directories as needed with 0700 permissions since this is per-user
// config.
func SaveHosts(hosts []Host) error {
	path := HostsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "hosts.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Clean up the temp file if anything below fails.
	defer func() { _ = os.Remove(tmpName) }()

	w := bufio.NewWriter(tmp)
	fmt.Fprintln(w, "# dirt hosts — one libvirt endpoint per line.")
	fmt.Fprintln(w, "# Format: <name> <uri>")
	fmt.Fprintln(w, "# Lines starting with # and empty lines are ignored.")
	fmt.Fprintln(w)
	// Align the URI column for readability — find the longest name.
	maxName := 4
	for _, h := range hosts {
		if n := len(h.Name); n > maxName {
			maxName = n
		}
	}
	for _, h := range hosts {
		fmt.Fprintf(w, "%-*s  %s\n", maxName, h.Name, h.URI)
	}
	if err := w.Flush(); err != nil {
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

// SeedIfMissing creates the hosts file with a single entry derived from
// initialURI if the file does not yet exist. Returns the resulting list
// (whether freshly seeded or already present). Never overwrites an
// existing file.
func SeedIfMissing(initialURI string) ([]Host, error) {
	existing, err := LoadHosts()
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}
	seed := Host{
		Name: nickFromURI(initialURI),
		URI:  normaliseURI(initialURI),
	}
	if err := SaveHosts([]Host{seed}); err != nil {
		return nil, err
	}
	return []Host{seed}, nil
}

// nickFromURI derives a short, readable nickname from a libvirt URI.
// Local URIs become "local"; remote URIs use the host part (first label).
// Falls back to "default" when parsing fails.
func nickFromURI(uri string) string {
	uri = normaliseURI(uri)
	if strings.HasPrefix(uri, "qemu:///") {
		return "local"
	}
	// libvirt URIs are URL-ish: "qemu+ssh://user@host.example/system".
	u, err := url.Parse(uri)
	if err != nil || u.Host == "" {
		return "default"
	}
	host := u.Hostname()
	if host == "" {
		return "default"
	}
	if dot := strings.IndexByte(host, '.'); dot > 0 {
		return host[:dot]
	}
	return host
}

// normaliseURI resolves the empty URI to the libvirt default. Mirrors
// the behaviour of lv.New so names assigned by nickFromURI match what
// lv will actually connect to.
func normaliseURI(uri string) string {
	if strings.TrimSpace(uri) == "" {
		return "qemu:///system"
	}
	return uri
}
