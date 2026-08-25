package main

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// newCollabPeer joins a peer to hub without any real network/SSH
// involved, mirroring what serveCollabSession does — so collabWrap,
// broadcasts, and the Hub's locking all run for real, just without a
// terminal on the other end. Received broadcasts land on the returned
// channel instead of a *tea.Program.
func newCollabPeer(t *testing.T, hub *Hub, name string) (*Model, chan tea.Msg) {
	t.Helper()
	inbox := make(chan tea.Msg, 16)
	m := NewModel("")
	m.hub = hub
	id, color := hub.Join(name, func(msg tea.Msg) { inbox <- msg })
	m.peerID, m.peerName, m.peerColor = id, name, color
	m.readOnly = hub.readOnly
	hub.mu.Lock()
	m.setTabs(hub.snapshot())
	hub.mu.Unlock()
	return &m, inbox
}

// expectMsg waits briefly for a broadcast to arrive — broadcasts run on
// their own goroutine (see Hub.broadcastExcept), so a bare channel read
// without a timeout would hang the test forever if that ever regressed
// into not sending at all.
func expectMsg(t *testing.T, inbox chan tea.Msg) tea.Msg {
	t.Helper()
	select {
	case msg := <-inbox:
		return msg
	case <-time.After(time.Second):
		t.Fatal("expected a broadcast message, got none")
		return nil
	}
}

func expectNoMsg(t *testing.T, inbox chan tea.Msg) {
	t.Helper()
	select {
	case msg := <-inbox:
		t.Fatalf("expected no broadcast message, got %#v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestCollabActiveTabIsPerPeer is the regression test for the "everyone
// gets thrown around when one client changes tabs" bug: the active tab
// index used to live on the Hub and get pushed to every peer, so one
// peer switching tabs yanked every other connected peer's view along
// with it. It must now be purely local per peer — the tab *list* is
// shared (everyone sees a new tab appear), but which one each peer is
// looking at is not.
func TestCollabActiveTabIsPerPeer(t *testing.T) {
	hub := NewHub([]*Document{NewDocument()}, false)
	m1, inbox1 := newCollabPeer(t, hub, "alice")
	m2, inbox2 := newCollabPeer(t, hub, "bob")

	// alice creates a new tab and jumps to it locally.
	newModel, _ := m1.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		mm.doNewTab()
		return mm, nil
	})
	nm1 := newModel.(Model)
	m1 = &nm1
	if m1.active != 1 {
		t.Fatalf("alice should be on the new tab she just created, got active=%d", m1.active)
	}

	// bob should be told a tab was created...
	if _, ok := expectMsg(t, inbox2).(collabRefreshMsg); !ok {
		t.Fatal("expected bob to receive a refresh after a tab was created")
	}
	newModel, _ = m2.collabWrap(func(mm Model) (tea.Model, tea.Cmd) { return mm, nil })
	nm2 := newModel.(Model)
	m2 = &nm2
	if len(m2.tabs) != 2 {
		t.Fatalf("expected bob to see 2 tabs after the refresh, got %d", len(m2.tabs))
	}
	// ...but bob's own view must NOT have jumped to it.
	if m2.active != 0 {
		t.Fatalf("bob's active tab must stay put when alice creates a tab, got active=%d", m2.active)
	}

	// Now bob deliberately switches to the new tab himself.
	newModel, _ = m2.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		mm.doSelectTab(1)
		return mm, nil
	})
	nm2 = newModel.(Model)
	m2 = &nm2
	if m2.active != 1 {
		t.Fatalf("bob's own tab switch should apply to him, got active=%d", m2.active)
	}

	// alice must not have been sent anything for bob's pure tab switch —
	// no document changed, only which one bob is looking at.
	expectNoMsg(t, inbox1)
	if m1.active != 1 {
		t.Fatalf("alice's active tab must be unaffected by bob's switch, got active=%d", m1.active)
	}
}

// TestCollabPanZoomIsPerPeer covers the earlier, analogous bug: pan and
// zoom lived on the shared Document, so one peer's middle-mouse pan (or
// zoom) moved every other connected peer's view too.
func TestCollabPanZoomIsPerPeer(t *testing.T) {
	hub := NewHub([]*Document{NewDocument()}, false)
	m1, _ := newCollabPeer(t, hub, "alice")
	m2, _ := newCollabPeer(t, hub, "bob")

	newModel, _ := m1.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		mm.setViewOffset(Point{X: 500, Y: 500})
		mm.setViewZoom(4)
		return mm, nil
	})
	nm1 := newModel.(Model)
	m1 = &nm1

	if got := m1.viewOffset(); got != (Point{X: 500, Y: 500}) {
		t.Fatalf("alice's own pan should apply to her, got %+v", got)
	}
	if got := m2.viewZoom(); got != 1 {
		t.Fatalf("bob's zoom must be unaffected by alice panning/zooming, got %v", got)
	}
	if got := m2.viewOffset(); got != (Point{}) {
		t.Fatalf("bob's pan must be unaffected by alice panning, got %+v", got)
	}
}

// TestCollabFileIOIsHostOnly covers the regression where making the host
// a real Hub peer (for tab/cursor sync) accidentally started blocking the
// host's own Save/SaveAs/Open/Export too, since the guard in handleKey
// only checked "is this a collab session" (m.hub != nil) rather than "is
// this specifically a guest". Drives the real handleKey path with a
// SaveAs keypress — doSaveAs only opens the save-as prompt, it doesn't
// touch disk until Enter is pressed on a real path, so this is safe to
// run as a unit test — and checks who actually got prompted.
func TestCollabFileIOIsHostOnly(t *testing.T) {
	hub := NewHub([]*Document{NewDocument()}, false)
	guest, _ := newCollabPeer(t, hub, "alice")
	host := *guest
	host.isHost = true

	saveAsKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl | tea.ModShift}

	newModel, _ := guest.handleKey(saveAsKey)
	ng := newModel.(Model)
	if ng.mode == modePromptSaveAs {
		t.Fatal("a collab guest must not be able to trigger Save As (host-only file I/O)")
	}

	newModel, _ = host.handleKey(saveAsKey)
	nh := newModel.(Model)
	if nh.mode != modePromptSaveAs {
		t.Fatal("the host must still be able to trigger Save As despite m.hub != nil")
	}
}

// TestCollabReadOnlyBlocksEditingNotViewing checks a read-only guest can
// still switch tabs (pure viewing) but not create a new one (document
// structure mutation) — driven through the real handleKey path, since
// the guard lives there (isMutatingKey), not in doSelectTab/doNewTab
// themselves.
func TestCollabReadOnlyBlocksEditingNotViewing(t *testing.T) {
	hub := NewHub([]*Document{NewDocument(), NewDocument()}, true)
	guest, _ := newCollabPeer(t, hub, "alice")
	if !guest.readOnly {
		t.Fatal("guest on a read-only hub should have readOnly set")
	}

	newModel, _ := guest.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		return mm.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	})
	nm := newModel.(Model)
	if nm.active != 1 {
		t.Fatal("a read-only guest should still be able to switch tabs (viewing, not editing)")
	}

	before := len(nm.tabs)
	newModel, _ = nm.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		return mm.handleKey(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	})
	nm2 := newModel.(Model)
	if len(nm2.tabs) != before {
		t.Fatal("a read-only guest must not be able to create a tab")
	}
}
