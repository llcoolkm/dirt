// dirt — David's virtual UI. A k9s-style TUI for libvirt.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/lv"
	"github.com/llcoolkm/dirt/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		uriFlag     = flag.String("uri", "", "libvirt URI (default: $LIBVIRT_DEFAULT_URI or qemu:///system)")
		refreshFlag = flag.Duration("refresh", 2*time.Second, "refresh interval (e.g. 1s, 500ms, 5s)")
		versionFlag = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dirt — k9s-style TUI for libvirt / QEMU / KVM\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  dirt [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dirt %s\n", version)
		return
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

	model := ui.New(client).WithRefreshInterval(*refreshFlag)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dirt: %v\n", err)
		os.Exit(1)
	}
}
