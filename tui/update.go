package tui

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kradalby/wc3ts/config"
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

		// Calculate available height for tables and logs
		// Reserve space for: title, section headers, status bar, help, and spacing
		availableHeight := m.height - fixedUIHeight

		// Split available height between peers table, games table, and logs
		if availableHeight > 0 {
			peerHeight := availableHeight * peerTablePct / 100 //nolint:mnd
			gameHeight := availableHeight * gameTablePct / 100 //nolint:mnd
			m.logHeight = availableHeight * logAreaPct / 100   //nolint:mnd

			if peerHeight < minTableHeight {
				peerHeight = minTableHeight
			}

			if gameHeight < minTableHeight {
				gameHeight = minTableHeight
			}

			if m.logHeight < minLogHeight {
				m.logHeight = minLogHeight
			}

			m.peerTable.SetHeight(peerHeight)
			m.gameTable.SetHeight(gameHeight)
		}

		return m, nil

	case PeersMsg:
		m.peers = msg.Peers
		m.peerTable.SetRows(m.peerRows())

		return m, nil

	case GamesMsg:
		m.games = msg.Games
		m.updatePeerGameCounts()
		m.gameTable.SetRows(m.gameRows())
		m.peerTable.SetRows(m.peerRows()) // Update peers to show game counts

		return m, nil

	case VersionMsg:
		m.version = msg.Version

		return m, nil

	case WarningMsg:
		m.warning = msg.Message

		return m, nil

	case LogMsg:
		m.logs = append(m.logs, msg.Message)
		// Keep only the last maxLogLines
		if len(m.logs) > maxLogLines {
			m.logs = m.logs[len(m.logs)-maxLogLines:]
		}

		return m, nil

	case PortMsg:
		m.proxyPort = msg.Port

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

	case "tab":
		// Switch focus between panels
		m = m.toggleFocus()

		return m, nil

	case "up", "k":
		// Navigate up in focused table
		m = m.navigateUp()

		return m, nil

	case "down", "j":
		// Navigate down in focused table
		m = m.navigateDown()

		return m, nil

	case "[", "-":
		// Decrease version
		m = m.cycleVersion(-1)

		return m, nil

	case "]", "+", "=":
		// Increase version
		m = m.cycleVersion(1)

		return m, nil

	case "s":
		// Sort peers by games
		m = m.sortPeersByGames()

		return m, nil
	}

	return m, nil
}

// toggleFocus switches focus between peer and game tables.
func (m Model) toggleFocus() Model {
	if m.focus == FocusPeers {
		m.focus = FocusGames
		m.peerTable.Blur()
		m.gameTable.Focus()
	} else {
		m.focus = FocusPeers
		m.gameTable.Blur()
		m.peerTable.Focus()
	}

	return m
}

// navigateUp moves selection up in the focused table.
func (m Model) navigateUp() Model {
	if m.focus == FocusPeers {
		m.peerTable.MoveUp(1)
	} else {
		m.gameTable.MoveUp(1)
	}

	return m
}

// navigateDown moves selection down in the focused table.
func (m Model) navigateDown() Model {
	if m.focus == FocusPeers {
		m.peerTable.MoveDown(1)
	} else {
		m.gameTable.MoveDown(1)
	}

	return m
}

// cycleVersion changes the game version by delta.
func (m Model) cycleVersion(delta int) Model {
	versions := config.SupportedVersions()
	currentIdx := -1

	for i, v := range versions {
		if v == m.version.Version {
			currentIdx = i

			break
		}
	}

	if currentIdx == -1 {
		// Current version not in list, start at beginning
		currentIdx = 0
	} else {
		currentIdx += delta
		if currentIdx < 0 {
			currentIdx = len(versions) - 1
		} else if currentIdx >= len(versions) {
			currentIdx = 0
		}
	}

	m.version.Version = versions[currentIdx]

	// Notify callback if set
	if m.versionCb != nil {
		m.versionCb(m.version.Version)
	}

	return m
}

// sortPeersByGames sorts peers by number of games (descending).
func (m Model) sortPeersByGames() Model {
	sort.Slice(m.peers, func(i, j int) bool {
		iGames := m.peerGames[m.peers[i].IP.String()]
		jGames := m.peerGames[m.peers[j].IP.String()]
		// Sort by games descending, then by name ascending
		if iGames != jGames {
			return iGames > jGames
		}

		return m.peers[i].Name < m.peers[j].Name
	})
	m.peerTable.SetRows(m.peerRows())

	return m
}

// updatePeerGameCounts updates the map of peer IP to game count.
func (m Model) updatePeerGameCounts() {
	// Clear and rebuild the map
	for k := range m.peerGames {
		delete(m.peerGames, k)
	}

	for i := range m.games {
		if m.games[i].Source == game.SourceRemote {
			ip := m.games[i].PeerIP.String()
			m.peerGames[ip]++
		}
	}
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

		gameCount := m.peerGames[peer.IP.String()]
		games := "-"

		if gameCount > 0 {
			games = strconv.Itoa(gameCount)
		}

		rows = append(rows, table.Row{
			peer.Name,
			peer.IP.String(),
			status,
			games,
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
