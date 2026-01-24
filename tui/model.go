// Package tui provides a Bubble Tea terminal user interface.
package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kradalby/wc3ts/game"
	"github.com/kradalby/wc3ts/tailscale"
	"github.com/kradalby/wc3ts/version"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

// Table column widths and layout constants.
const (
	colWidthName    = 20
	colWidthIP      = 16
	colWidthOS      = 10
	colWidthStatus  = 10
	colWidthGames   = 8
	colWidthGame    = 30
	colWidthHost    = 15
	colWidthPlayers = 10
	colWidthSource  = 10
	minTableHeight  = 3
	minLogHeight    = 3
	maxLogLines     = 10
	// fixedUIHeight accounts for title, headers, status bar, help, and spacing.
	fixedUIHeight = 11
	// Layout percentages for splitting available height.
	peerTablePct = 35
	gameTablePct = 35
	logAreaPct   = 30
)

// ViewMode indicates which view is currently displayed.
type ViewMode int

// View mode constants.
const (
	ViewModeList ViewMode = iota
	ViewModeDetailPeer
	ViewModeDetailGame
)

// FocusedPanel indicates which panel has focus.
type FocusedPanel int

// Focus panel constants.
const (
	FocusPeers FocusedPanel = iota
	FocusGames
)

// Model is the Bubble Tea model for the TUI.
type Model struct {
	peers        []tailscale.Peer
	games        []game.Game
	peerGames    map[string]int // IP -> game count
	version      w3gs.GameVersion
	buildVersion version.Info
	proxyPort    int
	peerTable    table.Model
	gameTable    table.Model
	logs         []string
	logHeight    int // calculated log area height
	width        int
	height       int
	ready        bool
	quitting     bool
	focus        FocusedPanel
	viewMode     ViewMode
	selectedPeer *tailscale.Peer // selected peer for detail view
	selectedGame *game.Game      // selected game for detail view
	versionCb    func(uint32)    // callback to notify version changes
	refreshCb    func()          // callback to trigger manual refresh
}

// PeersMsg is sent when the peer list changes.
type PeersMsg struct {
	Peers []tailscale.Peer
}

// GamesMsg is sent when the game list changes.
type GamesMsg struct {
	Games []game.Game
}

// LogMsg is sent when a log message should be displayed.
type LogMsg struct {
	Message string
}

// PortMsg is sent to update the proxy port after initialization.
type PortMsg struct {
	Port int
}

// NewModel creates a new TUI model.
// The versionCb callback is called when the user changes the game version.
// The refreshCb callback is called when the user requests a manual refresh.
func NewModel(
	proxyPort int,
	gameVersion w3gs.GameVersion,
	buildVersion version.Info,
	versionCb func(uint32),
	refreshCb func(),
) Model {
	peerColumns := []table.Column{
		{Title: "Name", Width: colWidthName},
		{Title: "IP", Width: colWidthIP},
		{Title: "OS", Width: colWidthOS},
		{Title: "Status", Width: colWidthStatus},
		{Title: "Games", Width: colWidthGames},
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
		table.WithFocused(true), // Start with peers focused
		table.WithHeight(minTableHeight),
	)

	gameTable := table.New(
		table.WithColumns(gameColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(minTableHeight),
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
		peers:        make([]tailscale.Peer, 0),
		games:        make([]game.Game, 0),
		peerGames:    make(map[string]int),
		version:      gameVersion,
		buildVersion: buildVersion,
		proxyPort:    proxyPort,
		peerTable:    peerTable,
		gameTable:    gameTable,
		logs:         make([]string, 0, maxLogLines),
		focus:        FocusPeers,
		viewMode:     ViewModeList,
		versionCb:    versionCb,
		refreshCb:    refreshCb,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}
