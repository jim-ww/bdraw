// Command bdraw is a mouse-first paint program for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

func usage() {
	fmt.Fprintln(os.Stderr, `bdraw - a mouse-first paint program for the terminal

Usage:
  bdraw [-c configfile] [file.json]

Flags:
  -c configfile   path to config.json (default: ~/.config/bdraw/config.json)
  --help          show this help

Arguments:
  file.json       project file to open on startup

Press ? inside bdraw for the full keybind reference.`)
}

func main() {
	configPath := flag.String("c", "", "path to config.json")
	flag.Usage = usage
	flag.Parse()

	zone.NewGlobal()

	m := NewModel(*configPath)
	if path := flag.Arg(0); path != "" {
		m.openInitialFile(path)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bdraw:", err)
		os.Exit(1)
	}
}
