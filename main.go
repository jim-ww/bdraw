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
  bdraw -collab [-collab-addr host:port] [-collab-readonly] [file.json]

Flags:
  -c configfile      path to config.json (default: ~/.config/bdraw/config.json)
  -collab            start an SSH collaboration server instead of the local UI
  -collab-addr addr  address to listen on (default: :2222)
  -collab-readonly   guests may view and follow along but not edit; only the
                      local terminal session (the one that started bdraw)
                      can edit
  --help             show this help

Arguments:
  file.json          project file to open on startup

In collaboration mode, joining is just SSHing into the session, e.g.
  ssh yourname@host -p 2222
Your SSH login name is used as your display name next to your cursor.

Press ? inside bdraw for the full keybind reference.`)
}

func main() {
	configPath := flag.String("c", "", "path to config.json")
	collab := flag.Bool("collab", false, "start an SSH collaboration server")
	collabAddr := flag.String("collab-addr", ":2222", "address for the collaboration server to listen on")
	collabReadOnly := flag.Bool("collab-readonly", false, "guests get read-only access; only the local session can edit")
	flag.Usage = usage
	flag.Parse()

	zone.NewGlobal()

	m := NewModel(*configPath)
	if path := flag.Arg(0); path != "" {
		m.openInitialFile(path)
	}

	if *collab {
		go func() {
			if err := runCollabServer(*collabAddr, m.doc(), *collabReadOnly, m.cfg); err != nil {
				fmt.Fprintln(os.Stderr, "bdraw: collab server:", err)
				os.Exit(1)
			}
		}()
		note := ""
		if *collabReadOnly {
			note = " (guests are read-only)"
		}
		m.status = fmt.Sprintf("collab server listening on %s — ssh <name>@host to join%s", *collabAddr, note)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bdraw:", err)
		os.Exit(1)
	}
}
