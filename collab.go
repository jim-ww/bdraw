package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/ssh"
	gossh "golang.org/x/crypto/ssh"
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

// collabRefreshMsg is broadcast after any peer mutates shared state — an
// edit to the active Document, or a tab created/closed/switched. It
// carries no data: on receipt, the recipient re-reads m.tabs/m.active
// from the Hub (see app.go's Update) to pick up any tab-list change, and
// its next render picks up each Document's new Version to re-rasterize.
// A pure edit to the existing active Document doesn't strictly need the
// tabs/active resync (that pointer is already shared), but doing it
// unconditionally keeps this one code path simple.
type collabRefreshMsg struct{}

// collabByeMsg tells a peer to drop a departed connection's cursor.
type collabByeMsg struct{ ID int }

var peerColors = []string{
	"#ff5f5f", "#5fd7ff", "#ffd75f", "#af87ff",
	"#87ff87", "#ff87d7", "#5fafff", "#ffaf5f",
}

// Hub coordinates a shared set of tabs across multiple SSH-connected
// peers, including the host's own local session (see runHostAndCollab in
// main.go — the host is peer 0, just like every guest, so its edits and
// tab changes broadcast out the same way guests' do). Hub.mu serializes
// every mouse/key event across all peers so no two connections ever
// mutate a document, or the tab list itself, concurrently. That's coarser
// than necessary but simple and safe at collaboration-session scale (a
// handful of concurrent editors sharing a terminal-speed connection, not
// a high-throughput service).
type Hub struct {
	mu       sync.Mutex
	tabs     []*Document
	active   int
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

// NewHub creates a Hub around the given tab list. readOnly, if true, means
// every non-host peer may view and follow along — including switching
// between tabs — but not edit or create/close tabs.
func NewHub(tabs []*Document, active int, readOnly bool) *Hub {
	return &Hub{tabs: tabs, active: active, readOnly: readOnly, peers: map[int]*peer{}}
}

// snapshot returns a copy of the current tab list and active index, safe
// to hand to a Model. Must be called with h.mu held or right after Join,
// before any other peer's mutation can race it.
func (h *Hub) snapshot() (tabs []*Document, active int) {
	return h.tabs, h.active
}

// docsSignature is a cheap fingerprint of every tab's identity and edit
// version plus which one is active, used by collabWrap to decide whether
// a peer's turn through Update actually changed shared state worth
// broadcasting (a new/closed/switched tab, or an edit) versus something
// purely local (cursor motion, a slider drag with no committed change).
func docsSignature(tabs []*Document, active int) string {
	sig := fmt.Sprintf("%d:", active)
	for _, d := range tabs {
		sig += fmt.Sprintf("%p=%d,", d, d.Version)
	}
	return sig
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
		go send(collabByeMsg{ID: id})
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
// called without h.mu held: peer.send is a *tea.Program.Send, which
// writes to that program's unbuffered message channel and so blocks
// until its event loop is idle and ready to receive.
//
// That send happens in its own goroutine, not inline on the caller's
// goroutine, for a reason that isn't optional: the caller is itself
// running inside a bubbletea Update() call (collabWrap calls this from
// there), on that program's own single event-loop goroutine. If peer A
// and peer B broadcast to each other at close enough to the same moment,
// an inline send would have A's event-loop goroutine blocked writing to
// B's channel while B's event-loop goroutine is simultaneously blocked
// writing to A's — neither loop is ever back around to actually receive,
// so both UIs freeze permanently. Sending from a fresh goroutine means
// the caller's own event loop is free to keep reading immediately,
// breaking that circular wait; the message still arrives, just not
// synchronously with the broadcast call.
func (h *Hub) broadcastExcept(fromID int, msg tea.Msg) {
	h.mu.Lock()
	senders := h.otherSendersLocked(fromID)
	h.mu.Unlock()
	for _, send := range senders {
		go send(msg)
	}
}

// MoveCursor broadcasts a peer's new canvas position to everyone else.
func (h *Hub) MoveCursor(id int, name, color string, pt Point, visible bool) {
	h.broadcastExcept(id, collabCursorMsg{ID: id, Name: name, Color: color, Pt: pt, Visible: visible})
}

// collabWrap runs fn — a mouse or key event handler — with the Hub's lock
// held (so it can't race a concurrent edit, or tab add/close/switch, from
// another peer), then broadcasts a refresh to everyone else if shared
// state actually changed, plus the peer's current cursor position. When
// m.hub is nil (no collaboration active) it's just a passthrough.
func (m Model) collabWrap(fn func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if m.hub == nil {
		return fn(m)
	}
	m.hub.mu.Lock()
	m.tabs, m.active = m.hub.snapshot()
	before := docsSignature(m.tabs, m.active)
	newModel, cmd := fn(m)
	nm := newModel.(Model)
	nm.hub.tabs, nm.hub.active = nm.tabs, nm.active
	after := docsSignature(nm.tabs, nm.active)
	m.hub.mu.Unlock()
	if after != before {
		m.hub.broadcastExcept(m.peerID, collabRefreshMsg{})
	}
	if nm.cursorVisible {
		m.hub.MoveCursor(m.peerID, m.peerName, m.peerColor, nm.cellToPoint(nm.cursorCol, nm.cursorRow), true)
	}
	return nm, cmd
}

// runCollabServer starts the SSH collaboration server on addr, serving
// hub's shared tabs to every connection. hub.readOnly gates whether
// guests may edit; the server itself has no authentication — "connecting
// is just ssh-ing into the session", per the design brief — so anyone who
// can reach addr can join.
//
// Peer identity comes from the SSH login name (i.e. `ssh alice@host`),
// which is the one piece of "who is this" the SSH protocol reliably
// exposes without inventing a separate handshake.
func runCollabServer(addr string, hub *Hub, cfg Config) error {
	signer, err := loadOrCreateHostKey()
	if err != nil {
		return fmt.Errorf("collab host key: %w", err)
	}

	s := &ssh.Server{
		Addr:    addr,
		Handler: func(sess ssh.Session) { serveCollabSession(sess, hub, cfg) },
	}
	s.AddHostKey(signer)
	log.Printf("bdraw collab server listening on %s (read-only guests: %v)", addr, hub.readOnly)
	return s.ListenAndServe()
}

// collabHostKeyPath is where the collab server's SSH host key is kept, in
// bdraw's data dir (paths.go) alongside recent-files/autosave — it's
// app-managed, not something a user edits, and losing it just means
// guests see a one-time "host key changed" warning, not lost work.
func collabHostKeyPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "collab_host_ed25519"), nil
}

// loadOrCreateHostKey returns the collab server's persistent SSH host
// key, generating and saving one on first use. Without this, ssh.Server
// generates a fresh random key every process start (see
// ensureHostSigner in charm.land/ssh), which makes every restart look
// like a different, possibly-spoofed server to any guest who connected
// before — SSH clients rightly refuse to proceed and warn about a
// possible man-in-the-middle attack. A stable key means a guest only
// ever has to accept the fingerprint once.
func loadOrCreateHostKey() (gossh.Signer, error) {
	path, err := collabHostKeyPath()
	if err != nil {
		return nil, err
	}

	if pemBytes, err := os.ReadFile(path); err == nil {
		return gossh.ParsePrivateKey(pemBytes)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := gossh.MarshalPrivateKey(priv, "bdraw collab host key")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, err
	}
	return gossh.NewSignerFromKey(priv)
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
	hub.mu.Lock()
	m.tabs, m.active = hub.snapshot()
	hub.mu.Unlock()
	m.readOnly = hub.readOnly
	m.width, m.height = pty.Window.Width, pty.Window.Height

	ctx, cancel := context.WithCancel(sess.Context())
	defer cancel()

	// Join needs a Send callback before the Program exists (its ID/color
	// feed into the model the Program is constructed with), and the
	// Program needs the fully-populated model before it exists — so this
	// closure defers to whatever *tea.Program pRef ends up holding, set
	// right after construction. Broadcasts now run on their own goroutine
	// (see broadcastExcept), so a peer could plausibly be sent a message
	// in the brief window between Join and the Set below; pRef is an
	// atomic.Pointer specifically so that race is a benign "message
	// arrives a moment before the pointer is visible and gets dropped by
	// the nil check" rather than a data race flagged by -race.
	var pRef atomic.Pointer[tea.Program]
	send := func(msg tea.Msg) {
		if p := pRef.Load(); p != nil {
			p.Send(msg)
		}
	}
	id, color := hub.Join(name, send)
	defer hub.Leave(id)
	m.hub = hub
	m.peerID = id
	m.peerName = name
	m.peerColor = color

	// Without this, bubbletea's color-profile/capability detection reads
	// the *server* process's own environment (TERM, COLORTERM, ...) —
	// not the connecting client's — since sess's I/O is an SSH channel,
	// not a real local pty the usual isatty/env probing was written
	// for. That's what caused missing colors and rendering artifacts
	// over SSH even though both ends run a perfectly capable terminal
	// (foot) locally: the server likely wasn't launched from an
	// interactive foot session itself (TERM=dumb or unset), so every
	// guest inherited that. Forwarding the client's own TERM (from the
	// pty-req it sent) plus its other env vars fixes detection for each
	// session independently.
	opts := []tea.ProgramOption{
		tea.WithInput(sess),
		tea.WithOutput(sess),
		tea.WithContext(ctx),
		tea.WithEnvironment(append(sess.Environ(), "TERM="+pty.Term)),
	}
	p := tea.NewProgram(m, opts...)
	pRef.Store(p)

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
