// Command bdraw is a mouse-first paint program for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"sync/atomic"

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

	if !*collab {
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "bdraw:", err)
			os.Exit(1)
		}
		return
	}

	// The host is peer 0 in its own Hub, the same way every SSH guest
	// is — not a privileged bystander that guests happen to sync
	// against. That's what makes tab creation/switching/closing (and
	// every edit) broadcast out to guests instead of only becoming
	// visible to them once they generate their own event; before this,
	// only the shared *Document's content propagated (via pointer
	// aliasing), never the host's own tab-list changes.
	hub := NewHub(m.tabs, m.active, *collabReadOnly)

	go func() {
		if err := runCollabServer(*collabAddr, hub, m.cfg); err != nil {
			fmt.Fprintln(os.Stderr, "bdraw: collab server:", err)
			os.Exit(1)
		}
	}()

	// Same Join-before-Program-exists trick as serveCollabSession
	// (collab.go): Join needs a Send callback, the Program needs the
	// fully-populated model (with peerID/color from Join) to construct.
	// pRef is an atomic.Pointer rather than a plain var because
	// broadcasts run on their own goroutine (see Hub.broadcastExcept) and
	// could plausibly race the assignment below.
	var pRef atomic.Pointer[tea.Program]
	send := func(msg tea.Msg) {
		if p := pRef.Load(); p != nil {
			p.Send(msg)
		}
	}
	id, color := hub.Join("host", send)
	m.hub = hub
	m.peerID = id
	m.peerName = "host"
	m.isHost = true
	m.peerColor = color
	m.readOnly = false // the host always has full rights, regardless of -collab-readonly

	note := ""
	if *collabReadOnly {
		note = " (guests are read-only)"
	}
	m.status = fmt.Sprintf("collab server listening on %s — ssh <name>@host to join%s", *collabAddr, note)

	p := tea.NewProgram(m)
	pRef.Store(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bdraw:", err)
		os.Exit(1)
	}
}
