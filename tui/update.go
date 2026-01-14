package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kradalby/wc3ts/game"
)

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		return m, nil

	case PeersMsg:
		m.peers = msg.Peers
		m.peerTable.SetRows(m.peerRows())

		return m, nil

	case GamesMsg:
		m.games = msg.Games
		m.gameTable.SetRows(m.gameRows())

		return m, nil

	case VersionMsg:
		m.version = msg.Version

		return m, nil
	}

	return m, nil
}

// handleKey handles keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true

		return m, tea.Quit
	}

	return m, nil
}

// peerRows converts peers to table rows.
func (m Model) peerRows() []table.Row {
	rows := make([]table.Row, 0, len(m.peers))

	for i := range m.peers {
		peer := &m.peers[i]
		status := "Offline"

		if peer.Online {
			status = "Online"
		}

		rows = append(rows, table.Row{
			peer.Name,
			peer.IP.String(),
			status,
		})
	}

	return rows
}

// gameRows converts games to table rows.
func (m Model) gameRows() []table.Row {
	rows := make([]table.Row, 0, len(m.games))

	for i := range m.games {
		g := &m.games[i]
		host := "Local"

		if g.Source == game.SourceRemote {
			host = g.PeerName
		}

		players := fmt.Sprintf("%d/%d", g.Info.SlotsUsed, g.Info.SlotsTotal)

		rows = append(rows, table.Row{
			g.Info.GameName,
			host,
			players,
			string(g.Source),
		})
	}

	return rows
}
