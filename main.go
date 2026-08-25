// Command bdraw is a mouse-first paint program for the terminal.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

func main() {
	zone.NewGlobal()

	m := NewModel()
	if len(os.Args) > 1 {
		m.openInitialFile(os.Args[1])
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bdraw:", err)
		os.Exit(1)
	}
}
