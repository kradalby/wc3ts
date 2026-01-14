package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// styles holds the TUI styling configuration.
type styles struct {
	title     lipgloss.Style
	header    lipgloss.Style
	statusBar lipgloss.Style
	help      lipgloss.Style
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

	var b strings.Builder

	// Title bar
	title := s.title.Render("wc3ts - WC3 LAN over Tailscale")
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

	// Status bar
	statusBar := m.statusBar()
	b.WriteString(s.statusBar.Render(statusBar))
	b.WriteString("\n")

	// Help
	help := s.help.Render("q: quit")
	b.WriteString(help)

	return b.String()
}

// versionString returns the version display string.
func (m Model) versionString() string {
	if m.version.Version == 0 {
		return "[detecting version...]"
	}

	return fmt.Sprintf("[%s v%d]", m.version.Product.String(), m.version.Version)
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
