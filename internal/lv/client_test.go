package lv

import "testing"

func TestParseOSFromXML(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "ubuntu 24.04",
			xml: `<domain><metadata>
				<libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0">
					<libosinfo:os id="http://ubuntu.com/ubuntu/24.04"/>
				</libosinfo:libosinfo>
			</metadata></domain>`,
			want: "Ubuntu 24.04",
		},
		{
			name: "debian 12",
			xml: `<domain><metadata>
				<libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0">
					<libosinfo:os id="http://debian.org/debian/12"/>
				</libosinfo:libosinfo>
			</metadata></domain>`,
			want: "Debian 12",
		},
		{
			name: "arch rolling",
			xml: `<domain><metadata>
				<libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0">
					<libosinfo:os id="http://archlinux.org/archlinux/rolling"/>
				</libosinfo:libosinfo>
			</metadata></domain>`,
			want: "Arch",
		},
		{
			name: "no metadata",
			xml:  `<domain><name>foo</name></domain>`,
			want: "",
		},
		{
			name: "malformed xml",
			xml:  `<domain<metadata`,
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseOSFromXML(c.xml)
			if got != c.want {
				t.Errorf("parseOSFromXML(%q) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestPrettyOSFromURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://ubuntu.com/ubuntu/24.04", "Ubuntu 24.04"},
		{"http://debian.org/debian/12", "Debian 12"},
		{"http://fedoraproject.org/fedora/40", "Fedora 40"},
		{"http://archlinux.org/archlinux/rolling", "Arch"},
		{"http://microsoft.com/win/11", "Windows 11"},
		{"http://freebsd.org/freebsd/14", "FreeBSD 14"},
		{"http://centos.org/centos/9", "CentOS 9"},
		{"http://opensuse.org/opensuse/15.5", "openSUSE 15.5"},
		{"http://novel.org/novel/1.0", "Novel 1.0"},
		{"", ""},
		{"not-a-url", "not-a-url"},
	}
	for _, c := range cases {
		got := prettyOSFromURL(c.in)
		if got != c.want {
			t.Errorf("prettyOSFromURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMeminfoSwap(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantTotalKB uint64
		wantFreeKB  uint64
	}{
		{
			name: "linux ubuntu",
			input: `MemTotal:        8123584 kB
MemFree:          254016 kB
SwapCached:        12000 kB
SwapTotal:       2097148 kB
SwapFree:        1500000 kB
`,
			wantTotalKB: 2097148,
			wantFreeKB:  1500000,
		},
		{
			name: "no swap",
			input: `MemTotal:        4194304 kB
SwapTotal:             0 kB
SwapFree:              0 kB
`,
			wantTotalKB: 0,
			wantFreeKB:  0,
		},
		{
			name:        "garbage",
			input:       "this is not /proc/meminfo",
			wantTotalKB: 0,
			wantFreeKB:  0,
		},
		{
			name: "only total",
			input: `SwapTotal:       1048576 kB
`,
			wantTotalKB: 1048576,
			wantFreeKB:  0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, free := parseMeminfoSwap(c.input)
			if total != c.wantTotalKB || free != c.wantFreeKB {
				t.Errorf("parseMeminfoSwap = (%d, %d), want (%d, %d)",
					total, free, c.wantTotalKB, c.wantFreeKB)
			}
		})
	}
}

func TestStateString(t *testing.T) {
	cases := []struct {
		s    State
		want string
	}{
		{StateRunning, "running"},
		{StatePaused, "paused"},
		{StateShutoff, "shut off"},
		{StateCrashed, "crashed"},
		{State(99), "—"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("State(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}
