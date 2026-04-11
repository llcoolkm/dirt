package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNickFromURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"", "local"},
		{"qemu:///system", "local"},
		{"qemu:///session", "local"},
		{"qemu+ssh://root@david.grogg.org/system", "david"},
		{"qemu+ssh://km@raspi/system", "raspi"},
		{"qemu+tls://libvirt.lab.example.com/system", "libvirt"},
		{"garbage:::nothing", "default"},
	}
	for _, tc := range tests {
		if got := nickFromURI(tc.uri); got != tc.want {
			t.Errorf("nickFromURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

func TestLoadHostsMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, err := LoadHosts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hosts != nil {
		t.Errorf("expected nil hosts for missing file, got %v", hosts)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	in := []Host{
		{Name: "local", URI: "qemu:///system"},
		{Name: "david", URI: "qemu+ssh://root@david.grogg.org/system"},
		{Name: "lab", URI: "qemu+tls://libvirt.lab.example.com/system"},
	}
	if err := SaveHosts(in); err != nil {
		t.Fatalf("SaveHosts: %v", err)
	}
	out, err := LoadHosts()
	if err != nil {
		t.Fatalf("LoadHosts: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n want: %v\n got:  %v", in, out)
	}
}

func TestLoadIgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "dirt"), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `# a comment
# another

local    qemu:///system
   # indented comment — skipped because TrimSpace + HasPrefix
david    qemu+ssh://root@david.grogg.org/system

# trailing comment
`
	if err := os.WriteFile(filepath.Join(dir, "dirt", "hosts"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts, err := LoadHosts()
	if err != nil {
		t.Fatalf("LoadHosts: %v", err)
	}
	want := []Host{
		{Name: "local", URI: "qemu:///system"},
		{Name: "david", URI: "qemu+ssh://root@david.grogg.org/system"},
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("parse mismatch:\n want: %v\n got:  %v", want, hosts)
	}
}

func TestSeedIfMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hosts, err := SeedIfMissing("qemu+ssh://root@david.grogg.org/system")
	if err != nil {
		t.Fatalf("SeedIfMissing: %v", err)
	}
	want := []Host{{Name: "david", URI: "qemu+ssh://root@david.grogg.org/system"}}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("seed mismatch:\n want: %v\n got:  %v", want, hosts)
	}

	// Second call must not overwrite; returns the existing list.
	again, err := SeedIfMissing("qemu:///system")
	if err != nil {
		t.Fatalf("SeedIfMissing (second call): %v", err)
	}
	if !reflect.DeepEqual(again, want) {
		t.Errorf("seed overwrote existing file:\n want: %v\n got:  %v", want, again)
	}
}
