package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/ssh"
)

// peerCursor is another connected peer's last-known canvas position, kept
// per-Model so every viewer can render every other viewer's cursor
// regardless of their own independent pan/zoom.
type peerCursor struct {
	Name    string
	Color   string
	Pt      Point
	Visible bool
}

// collabCursorMsg is broadcast to every other peer whenever one peer's
// cursor moves over the canvas.
type collabCursorMsg struct {
	ID      int
	Name    string
	Color   string
	Pt      Point
	Visible bool
}

// collabRefreshMsg is broadcast after any peer mutates the shared
// Document. It carries no data: the mutation already happened directly on
// the *Document every peer's Model shares a pointer to (under Hub's
// lock) — this message exists purely to make bubbletea run another
// Update/View cycle for the recipient, whose next render picks up the new
// Document.Version and re-rasterizes.
type collabRefreshMsg struct{}

// collabByeMsg tells a peer to drop a departed connection's cursor.
type collabByeMsg struct{ ID int }

var peerColors = []string{
	"#ff5f5f", "#5fd7ff", "#ffd75f", "#af87ff",
	"#87ff87", "#ff87d7", "#5fafff", "#ffaf5f",
}

// Hub coordinates a single shared Document across multiple SSH-connected
// peers. Every connection's Model.tabs[0] points at the same *Document;
// Hub.mu serializes every mouse/key event across all peers so no two
// connections ever mutate the document concurrently. That's coarser than
// necessary but simple and safe at collaboration-session scale (a handful
// of concurrent editors sharing a terminal-speed connection, not a
// high-throughput service).
type Hub struct {
	mu       sync.Mutex
	doc      *Document
	readOnly bool
	peers    map[int]*peer
	nextID   int
}

type peer struct {
	id    int
	name  string
	color string
	send  func(tea.Msg)
}

// NewHub creates a Hub around doc. readOnly, if true, means every guest
// (every peer except the local host, who never goes through the Hub) may
// view and follow along but not edit.
func NewHub(doc *Document, readOnly bool) *Hub {
	return &Hub{doc: doc, readOnly: readOnly, peers: map[int]*peer{}}
}

// Join registers a new peer and returns its ID and assigned cursor color.
func (h *Hub) Join(name string, send func(tea.Msg)) (id int, color string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id = h.nextID
	color = peerColors[id%len(peerColors)]
	h.peers[id] = &peer{id: id, name: name, color: color, send: send}
	return id, color
}

// Leave removes a peer and tells everyone else to drop its cursor.
func (h *Hub) Leave(id int) {
	h.mu.Lock()
	delete(h.peers, id)
	others := h.otherSendersLocked(id)
	h.mu.Unlock()
	for _, send := range others {
		send(collabByeMsg{ID: id})
	}
}

func (h *Hub) otherSendersLocked(exceptID int) []func(tea.Msg) {
	out := make([]func(tea.Msg), 0, len(h.peers))
	for _, p := range h.peers {
		if p.id != exceptID {
			out = append(out, p.send)
		}
	}
	return out
}

// broadcastExcept sends msg to every peer other than fromID. Must be
// called without h.mu held: peer.send is a *tea.Program.Send, which can
// block briefly, and holding the doc lock across that would stall other
// peers' edits for no reason.
func (h *Hub) broadcastExcept(fromID int, msg tea.Msg) {
	h.mu.Lock()
	senders := h.otherSendersLocked(fromID)
	h.mu.Unlock()
	for _, send := range senders {
		send(msg)
	}
}

// MoveCursor broadcasts a peer's new canvas position to everyone else.
func (h *Hub) MoveCursor(id int, name, color string, pt Point, visible bool) {
	h.broadcastExcept(id, collabCursorMsg{ID: id, Name: name, Color: color, Pt: pt, Visible: visible})
}

// collabWrap runs fn — a mouse or key event handler — with the Hub's doc
// lock held (so it can't race a concurrent edit from another peer), then
// broadcasts a refresh to everyone else if the document actually changed,
// plus the peer's current cursor position. When m.hub is nil (no
// collaboration active) it's just a passthrough.
func (m Model) collabWrap(fn func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if m.hub == nil {
		return fn(m)
	}
	before := m.doc().Version
	m.hub.mu.Lock()
	newModel, cmd := fn(m)
	m.hub.mu.Unlock()
	nm := newModel.(Model)
	if nm.doc().Version != before {
		m.hub.broadcastExcept(m.peerID, collabRefreshMsg{})
	}
	if nm.cursorVisible {
		m.hub.MoveCursor(m.peerID, m.peerName, m.peerColor, nm.cellToPoint(nm.cursorCol, nm.cursorRow), true)
	}
	return nm, cmd
}

// runCollabServer starts the SSH collaboration server on addr, serving
// doc to every connection. readOnly gates whether guests may edit;
// the server itself has no authentication — "connecting is just
// ssh-ing into the session", per the design brief — so anyone who can
// reach addr can join.
//
// Peer identity comes from the SSH login name (i.e. `ssh alice@host`),
// which is the one piece of "who is this" the SSH protocol reliably
// exposes without inventing a separate handshake.
func runCollabServer(addr string, doc *Document, readOnly bool, cfg Config) error {
	hub := NewHub(doc, readOnly)

	s := &ssh.Server{
		Addr:    addr,
		Handler: func(sess ssh.Session) { serveCollabSession(sess, hub, cfg) },
	}
	log.Printf("bdraw collab server listening on %s (read-only guests: %v)", addr, readOnly)
	return s.ListenAndServe()
}

// serveCollabSession wires one SSH session to its own bubbletea Program
// sharing the Hub's Document. It bypasses wish's bubbletea/ middleware
// helper entirely: that helper is hard-tied to the old
// github.com/charmbracelet/bubbletea (v1) API, while bdraw's Model
// implements charm.land/bubbletea/v2 — a different, incompatible
// tea.Model/tea.Program. Wiring a v2 Program directly to the session's
// I/O (tea.WithInput/tea.WithOutput) and forwarding Pty() resize events
// as v2's own tea.WindowSizeMsg sidesteps that entirely.
func serveCollabSession(sess ssh.Session, hub *Hub, cfg Config) {
	pty, winCh, ok := sess.Pty()
	if !ok {
		fmt.Fprintln(sess.Stderr(), "bdraw collab requires a PTY (try: ssh -t ...)")
		sess.Exit(1)
		return
	}

	name := sess.User()
	if name == "" {
		name = "guest"
	}

	m := NewModel("")
	m.cfg = cfg
	applyPaletteOverride(cfg.Palette)
	m.tabs = []*Document{hub.doc}
	m.active = 0
	m.readOnly = hub.readOnly
	m.width, m.height = pty.Window.Width, pty.Window.Height

	ctx, cancel := context.WithCancel(sess.Context())
	defer cancel()

	// Join needs a Send callback before the Program exists (its ID/color
	// feed into the model the Program is constructed with), and the
	// Program needs the fully-populated model before it exists — so this
	// closure defers to whatever *tea.Program p ends up being, set right
	// after construction and always resolved by the time any other peer's
	// broadcast could actually reach it (the session hasn't started
	// running yet).
	var p *tea.Program
	send := func(msg tea.Msg) {
		if p != nil {
			p.Send(msg)
		}
	}
	id, color := hub.Join(name, send)
	defer hub.Leave(id)
	m.hub = hub
	m.peerID = id
	m.peerName = name
	m.peerColor = color

	opts := []tea.ProgramOption{
		tea.WithInput(sess),
		tea.WithOutput(sess),
		tea.WithContext(ctx),
	}
	p = tea.NewProgram(m, opts...)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case w, ok := <-winCh:
				if !ok {
					return
				}
				p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(sess.Stderr(), "bdraw:", err)
	}
}
