package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kradalby/wc3ts/game"
)

// Detail view styling constants.
const (
	detailBoxPaddingVert  = 1
	detailBoxPaddingHoriz = 2
	detailLabelWidth      = 14
)

// styles holds the TUI styling configuration.
type styles struct {
	title       lipgloss.Style
	header      lipgloss.Style
	statusBar   lipgloss.Style
	help        lipgloss.Style
	logLine     lipgloss.Style
	detailBox   lipgloss.Style
	detailLabel lipgloss.Style
	detailValue lipgloss.Style
}

// newStyles creates the TUI styles.
func newStyles() styles {
	return styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 1),
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")),
		statusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		logLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		detailBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(detailBoxPaddingVert, detailBoxPaddingHoriz),
		detailLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Width(detailLabelWidth),
		detailValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")),
	}
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing...\n"
	}

	s := newStyles()

	// Handle detail views
	switch m.viewMode {
	case ViewModeDetailPeer:
		return m.viewPeerDetail(s)
	case ViewModeDetailGame:
		return m.viewGameDetail(s)
	case ViewModeList:
		// Fall through to render list view below
	}

	var b strings.Builder

	// Title bar
	titleText := "wc3ts " + m.buildVersion.String()
	title := s.title.Render(titleText)
	versionInfo := m.versionString()

	titleBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		"  ",
		versionInfo,
	)

	b.WriteString(titleBar)
	b.WriteString("\n\n")

	// Peers section
	b.WriteString(s.header.Render("Tailscale Peers"))
	b.WriteString("\n")
	b.WriteString(m.peerTable.View())
	b.WriteString("\n\n")

	// Games section
	b.WriteString(s.header.Render("Games"))
	b.WriteString("\n")
	b.WriteString(m.gameTable.View())
	b.WriteString("\n\n")

	// Debug logs section
	b.WriteString(s.header.Render("Debug Log"))
	b.WriteString("\n")

	if len(m.logs) == 0 {
		b.WriteString(s.logLine.Render("  (no logs yet)"))
		b.WriteString("\n")
	} else {
		// Show only the last logHeight lines (or maxLogLines if logHeight not set)
		displayLines := m.logHeight
		if displayLines <= 0 || displayLines > maxLogLines {
			displayLines = maxLogLines
		}

		startIdx := 0
		if len(m.logs) > displayLines {
			startIdx = len(m.logs) - displayLines
		}

		for _, line := range m.logs[startIdx:] {
			b.WriteString(s.logLine.Render("  " + line))
			b.WriteString("\n")
		}
	}

	// Status bar
	statusBar := m.statusBar()
	b.WriteString(s.statusBar.Render(statusBar))
	b.WriteString("\n")

	// Help
	focusIndicator := "peers"
	if m.focus == FocusGames {
		focusIndicator = "games"
	}

	help := s.help.Render(fmt.Sprintf(
		"↑/↓: navigate | tab: switch (%s) | enter: details | r: refresh | [/]: version | s: sort | q: quit",
		focusIndicator,
	))
	b.WriteString(help)

	return b.String()
}

// viewPeerDetail renders the peer detail view.
func (m Model) viewPeerDetail(s styles) string {
	if m.selectedPeer == nil {
		return "No peer selected"
	}

	peer := m.selectedPeer

	var b strings.Builder

	// Title
	title := s.title.Render("Peer Details")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Detail content
	var content strings.Builder

	content.WriteString(m.detailRow(s, "Name:", peer.Name))
	content.WriteString(m.detailRow(s, "IP:", peer.IP.String()))

	osDisplay := peer.OS
	if osDisplay == "" {
		osDisplay = "-"
	} else {
		osDisplay = strings.ToUpper(osDisplay[:1]) + osDisplay[1:]
	}

	content.WriteString(m.detailRow(s, "OS:", osDisplay))

	status := "Offline"
	if peer.Online {
		status = "Online"
	}

	content.WriteString(m.detailRow(s, "Status:", status))

	// Count games hosted by this peer
	gameCount := 0

	var peerGames []game.Game

	for i := range m.games {
		if m.games[i].Source == game.SourceRemote && m.games[i].PeerIP == peer.IP {
			gameCount++

			peerGames = append(peerGames, m.games[i])
		}
	}

	content.WriteString(m.detailRow(s, "Games:", strconv.Itoa(gameCount)))

	// List games hosted by this peer
	if len(peerGames) > 0 {
		content.WriteString("\n")
		content.WriteString(s.detailLabel.Render("Hosted games:"))
		content.WriteString("\n")

		for _, g := range peerGames {
			gameLine := fmt.Sprintf("  - %s (%d/%d players)",
				g.Info.GameName,
				g.Info.SlotsUsed,
				g.Info.SlotsTotal,
			)
			content.WriteString(s.detailValue.Render(gameLine))
			content.WriteString("\n")
		}
	}

	// Render box
	box := s.detailBox.Render(content.String())
	b.WriteString(box)
	b.WriteString("\n\n")

	// Help
	help := s.help.Render("Press Escape to return")
	b.WriteString(help)

	return b.String()
}

// viewGameDetail renders the game detail view.
func (m Model) viewGameDetail(s styles) string {
	if m.selectedGame == nil {
		return "No game selected"
	}

	g := m.selectedGame

	var b strings.Builder

	// Title
	title := s.title.Render("Game Details")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Detail content
	var content strings.Builder

	content.WriteString(m.detailRow(s, "Name:", g.Info.GameName))
	content.WriteString(m.detailRow(s, "Map:", g.Info.GameSettings.MapPath))
	content.WriteString(m.detailRow(s, "Players:", fmt.Sprintf("%d/%d", g.Info.SlotsUsed, g.Info.SlotsTotal)))

	// Host player name (from WC3 game)
	hostPlayer := g.Info.GameSettings.HostName
	if hostPlayer == "" {
		hostPlayer = "-"
	}

	content.WriteString(m.detailRow(s, "Host Player:", hostPlayer))

	// Version info
	versionStr := fmt.Sprintf("%s 1.%d", g.Info.Product.String(), g.Info.Version)
	content.WriteString(m.detailRow(s, "Version:", versionStr))
	content.WriteString(m.detailRow(s, "Source:", string(g.Source)))

	// Host peer info (for remote games)
	if g.Source == game.SourceRemote {
		peerName := g.PeerName
		if peerName == "" {
			peerName = "-"
		}

		content.WriteString(m.detailRow(s, "Host Peer:", peerName))
		content.WriteString(m.detailRow(s, "Host IP:", g.PeerIP.String()))
	}

	content.WriteString(m.detailRow(s, "Game Port:", strconv.FormatUint(uint64(g.Info.GamePort), 10)))

	// Timestamps
	if !g.FirstSeen.IsZero() {
		content.WriteString(m.detailRow(s, "First Seen:", formatDuration(time.Since(g.FirstSeen))))
	}

	if !g.LastSeen.IsZero() {
		content.WriteString(m.detailRow(s, "Last Seen:", formatDuration(time.Since(g.LastSeen))))
	}

	// Render box
	box := s.detailBox.Render(content.String())
	b.WriteString(box)
	b.WriteString("\n\n")

	// Help
	help := s.help.Render("Press Escape to return")
	b.WriteString(help)

	return b.String()
}

// detailRow creates a formatted detail row with label and value.
func (m Model) detailRow(s styles, label, value string) string {
	return s.detailLabel.Render(label) + " " + s.detailValue.Render(value) + "\n"
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	}

	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}

	return fmt.Sprintf("%d hours ago", int(d.Hours()))
}

// versionString returns the version display string.
func (m Model) versionString() string {
	if m.version.Version == 0 {
		return "[detecting version...]"
	}

	return fmt.Sprintf("[%s 1.%d]", m.version.Product.String(), m.version.Version)
}

// statusBar returns the status bar content.
func (m Model) statusBar() string {
	onlinePeers := 0

	for i := range m.peers {
		if m.peers[i].Online {
			onlinePeers++
		}
	}

	localGames := 0
	remoteGames := 0

	for i := range m.games {
		if m.games[i].Source == "local" {
			localGames++
		} else {
			remoteGames++
		}
	}

	return fmt.Sprintf(
		"UDP 6112 | TCP Proxy: %d | Peers: %d online | Games: %d local, %d remote",
		m.proxyPort,
		onlinePeers,
		localGames,
		remoteGames,
	)
}
