// dirt — David's virtual UI. A k9s-style TUI for libvirt.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/lv"
	"github.com/llcoolkm/dirt/internal/ui"
)

func main() {
	uri := os.Getenv("LIBVIRT_DEFAULT_URI")
	client, err := lv.New(uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dirt: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	p := tea.NewProgram(ui.New(client), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dirt: %v\n", err)
		os.Exit(1)
	}
}
