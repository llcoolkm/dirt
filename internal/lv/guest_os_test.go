package lv

import "testing"

func TestParseGuestOSInfoLinux(t *testing.T) {
	resp := `{"return":{"name":"Ubuntu","kernel-release":"6.8.0-45-generic",` +
		`"version":"24.04.2 LTS (Noble Numbat)","pretty-name":"Ubuntu 24.04.2 LTS",` +
		`"version-id":"24.04","kernel-version":"#45-Ubuntu SMP","machine":"x86_64","id":"ubuntu"}}`
	id, pretty, err := parseGuestOSInfo(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "ubuntu" {
		t.Errorf("id = %q, want ubuntu", id)
	}
	if pretty != "Ubuntu 24.04" {
		t.Errorf("pretty = %q, want 'Ubuntu 24.04'", pretty)
	}
}

func TestParseGuestOSInfoWindows(t *testing.T) {
	resp := `{"return":{"name":"Microsoft Windows","kernel-release":"26100",` +
		`"version":"Microsoft Windows 11","pretty-name":"Windows 11 Pro",` +
		`"version-id":"11","kernel-version":"10.0","machine":"x86_64","id":"mswindows"}}`
	id, pretty, err := parseGuestOSInfo(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "mswindows" {
		t.Errorf("id = %q, want mswindows", id)
	}
	if pretty != "Windows 11" {
		t.Errorf("pretty = %q, want 'Windows 11'", pretty)
	}
	if !(GuestOSInfo{Available: true, ID: id}).IsWindows() {
		t.Error("IsWindows() = false for mswindows")
	}
}

func TestParseGuestOSInfoEmpty(t *testing.T) {
	if _, _, err := parseGuestOSInfo(`{"return":{}}`); err == nil {
		t.Error("expected error for empty response")
	}
	if _, _, err := parseGuestOSInfo(`not json`); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCompactOSLabel(t *testing.T) {
	cases := []struct {
		name, prettyName, versionID, want string
	}{
		{"Ubuntu", "Ubuntu 24.04.2 LTS", "24.04", "Ubuntu 24.04"},
		{"Microsoft Windows", "Windows 11 Pro", "11", "Windows 11"},
		{"Arch Linux", "Arch Linux", "", "Arch Linux"},
		{"", "Some OS 1.0", "", "Some OS 1.0"},
	}
	for _, c := range cases {
		if got := compactOSLabel(c.name, c.prettyName, c.versionID); got != c.want {
			t.Errorf("compactOSLabel(%q,%q,%q) = %q, want %q", c.name, c.prettyName, c.versionID, got, c.want)
		}
	}
}
