// Package tui provides a Bubble Tea terminal user interface.
package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kradalby/wc3ts/game"
	"github.com/kradalby/wc3ts/tailscale"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Table column widths.
const (
	colWidthName    = 20
	colWidthIP      = 15
	colWidthStatus  = 10
	colWidthGame    = 30
	colWidthHost    = 15
	colWidthPlayers = 10
	colWidthSource  = 10
	tableHeight     = 5
)

// Model is the Bubble Tea model for the TUI.
type Model struct {
	peers     []tailscale.Peer
	games     []game.Game
	version   w3gs.GameVersion
	proxyPort int
	peerTable table.Model
	gameTable table.Model
	width     int
	height    int
	ready     bool
	quitting  bool
	warning   string
}

// PeersMsg is sent when the peer list changes.
type PeersMsg struct {
	Peers []tailscale.Peer
}

// GamesMsg is sent when the game list changes.
type GamesMsg struct {
	Games []game.Game
}

// VersionMsg is sent when the game version is detected.
type VersionMsg struct {
	Version w3gs.GameVersion
}

// WarningMsg is sent when a warning needs to be displayed.
type WarningMsg struct {
	Message string
}

// NewModel creates a new TUI model.
func NewModel(proxyPort int) Model {
	peerColumns := []table.Column{
		{Title: "Name", Width: colWidthName},
		{Title: "IP", Width: colWidthIP},
		{Title: "Status", Width: colWidthStatus},
	}

	gameColumns := []table.Column{
		{Title: "Name", Width: colWidthGame},
		{Title: "Host", Width: colWidthHost},
		{Title: "Players", Width: colWidthPlayers},
		{Title: "Source", Width: colWidthSource},
	}

	peerTable := table.New(
		table.WithColumns(peerColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(tableHeight),
	)

	gameTable := table.New(
		table.WithColumns(gameColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(tableHeight),
	)

	// Apply styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	peerTable.SetStyles(s)
	gameTable.SetStyles(s)

	return Model{
		peers:     make([]tailscale.Peer, 0),
		games:     make([]game.Game, 0),
		proxyPort: proxyPort,
		peerTable: peerTable,
		gameTable: gameTable,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}
