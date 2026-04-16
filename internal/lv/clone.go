package lv

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Clone runs `virt-clone` to duplicate an existing (stopped) domain
// under a new name. Disk files are copied into the same directory
// with "-clone" appended; MAC addresses are regenerated.
//
// The operation can be slow — disk copies are sequential and
// sparse-aware. Intended to be called from a Job goroutine.
func (c *Client) Clone(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("clone: src and dst must be set")
	}
	// virt-clone supports --connect for a remote daemon so we pass the
	// same URI the rest of dirt is using. The --auto-clone flag lets
	// virt-clone pick disk paths automatically (<name>-clone.qcow2 in
	// the same directory as the source).
	cmd := exec.Command("virt-clone",
		"--connect", c.uri,
		"--original", src,
		"--name", dst,
		"--auto-clone",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("virt-clone: %s", msg)
	}
	return nil
}
