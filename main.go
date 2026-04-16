// dirt — a terminal UI for libvirt.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
	"github.com/llcoolkm/dirt/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
// When unset (the default for `go install`), we fall back to the module
// version embedded in the binary by the Go toolchain.
var version = "dev"

// resolveVersion returns the build-time version if it was injected, otherwise
// the module version recorded by `go install` / `go build`.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	var (
		uriFlag     = flag.String("uri", "", "libvirt URI (default: $LIBVIRT_DEFAULT_URI or qemu:///system)")
		refreshFlag = flag.Duration("refresh", 0, "refresh interval (e.g. 1s, 500ms, 5s) — overrides the config file")
		versionFlag = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dirt — a terminal UI for libvirt / QEMU / KVM\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  dirt [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dirt %s\n", resolveVersion())
		return
	}

	// Load (or seed) the persistent user config. A load error is
	// non-fatal — we fall through to defaults and warn the user so the
	// TUI still starts on a broken config file.
	cfg, err := config.SeedConfigIfMissing()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dirt: warning — could not read %s: %v\n", config.ConfigPath(), err)
	}

	// CLI --refresh overrides the config file. The zero default lets
	// us distinguish "flag unset" from an intentional 0 (which would
	// be clamped to the 200ms floor anyway).
	refresh := cfg.Refresh
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "refresh" {
			refresh = *refreshFlag
		}
	})
	if refresh <= 0 {
		refresh = 1 * time.Second
	}

	uri := *uriFlag
	if uri == "" {
		uri = os.Getenv("LIBVIRT_DEFAULT_URI")
	}

	client, err := lv.New(uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dirt: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	model := ui.New(client).WithConfig(cfg).WithRefreshInterval(refresh)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	// Wire the program reference so background goroutines (live
	// migration progress, snapshot jobs, etc.) can push messages back
	// into the loop.
	ui.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dirt: %v\n", err)
		os.Exit(1)
	}
}
