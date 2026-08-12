package lv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"libvirt.org/go/libvirt"
)

// GuestOSInfo is the live OS identity reported by the qemu-guest-agent's
// guest-get-osinfo command. Unlike the libosinfo metadata in the domain
// XML (which is written once at install time and never updated), this
// reflects what is actually running inside the guest right now.
type GuestOSInfo struct {
	Available bool   // true only if QGA answered and parsing succeeded
	ID        string // machine-readable id, e.g. "ubuntu", "mswindows"
	Pretty    string // compact human label, e.g. "Ubuntu 24.04"
	FetchedAt time.Time
	Err       error
}

// IsWindows reports whether the guest identified itself as Windows.
func (g GuestOSInfo) IsWindows() bool {
	return g.Available && g.ID == "mswindows"
}

// GuestOSInfo queries the qemu-guest-agent inside the named domain for its
// live OS identity via guest-get-osinfo. This is a single direct QGA call
// (no guest-exec), so it works on both Linux and Windows agents. Returns
// Available=false (with Err set) if QGA is not installed, not connected,
// or times out.
func (c *Client) GuestOSInfo(name string) GuestOSInfo {
	info := GuestOSInfo{FetchedAt: time.Now()}
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		resp, err := d.QemuAgentCommand(`{"execute":"guest-get-osinfo"}`, 2, 0)
		if err != nil {
			return fmt.Errorf("guest-get-osinfo: %w", err)
		}
		id, pretty, err := parseGuestOSInfo(resp)
		if err != nil {
			return err
		}
		info.ID = id
		info.Pretty = pretty
		info.Available = true
		return nil
	})
	if err != nil {
		info.Err = err
	}
	return info
}

// parseGuestOSInfo extracts a machine id and a compact display label from
// a guest-get-osinfo JSON response.
func parseGuestOSInfo(resp string) (id, pretty string, err error) {
	var r struct {
		Return struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			PrettyName string `json:"pretty-name"`
			Version    string `json:"version"`
			VersionID  string `json:"version-id"`
		} `json:"return"`
	}
	if err := json.Unmarshal([]byte(resp), &r); err != nil {
		return "", "", fmt.Errorf("decode guest-get-osinfo: %w", err)
	}
	ret := r.Return
	if ret.ID == "" && ret.Name == "" && ret.PrettyName == "" {
		return "", "", fmt.Errorf("empty guest-get-osinfo response")
	}
	return ret.ID, compactOSLabel(ret.Name, ret.PrettyName, ret.VersionID), nil
}

// compactOSLabel builds a short OS label that fits a table column:
// "Ubuntu 24.04" rather than "Ubuntu 24.04.2 LTS (Noble Numbat)".
// Windows drops the "Microsoft " prefix.
func compactOSLabel(name, prettyName, versionID string) string {
	name = strings.TrimPrefix(name, "Microsoft ")
	switch {
	case name != "" && versionID != "":
		return name + " " + versionID
	case name != "":
		return name
	default:
		return prettyName
	}
}
