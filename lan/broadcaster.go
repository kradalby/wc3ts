package lan

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/kradalby/wc3ts/game"
)

// DefaultPort is the standard WC3 LAN port.
const DefaultPort = 6112

// BroadcastInterval is how often to send game broadcasts.
const BroadcastInterval = 3 * time.Second

// writeBufferSize is the UDP write buffer size.
const writeBufferSize = 64 * 1024

// minPacketSize is the minimum valid GameInfo packet size.
const minPacketSize = 4

// portFieldSize is the size of the port field at the end of GameInfo packets.
const portFieldSize = 2

// byteShift8 is the bit shift for the second byte of a uint16.
const byteShift8 = 8

// byteShift16 is the bit shift for the third byte of a uint32.
const byteShift16 = 16

// byteShift24 is the bit shift for the fourth byte of a uint32.
const byteShift24 = 24

// Broadcaster periodically broadcasts remote games to the local LAN.
// It forwards raw packet bytes with only the port modified.
type Broadcaster struct {
	conn             *net.UDPConn
	games            []game.Game
	previousGameKeys map[string]uint32 // game key -> HostCounter for tracking removed games
	proxyPort        uint16
	broadcastAddr    *net.UDPAddr
	mu               sync.RWMutex
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster(proxyPort uint16) (*Broadcaster, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}

	err = conn.SetWriteBuffer(writeBufferSize)
	if err != nil {
		slog.Debug("failed to set write buffer", "error", err)
	}

	return &Broadcaster{
		conn:             conn,
		proxyPort:        proxyPort,
		broadcastAddr:    &net.UDPAddr{IP: net.IPv4bcast, Port: DefaultPort},
		previousGameKeys: make(map[string]uint32),
	}, nil
}

// Run starts the broadcast loop.
func (b *Broadcaster) Run(ctx context.Context) error {
	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			b.broadcastGames()
		}
	}
}

// OnGamesChanged updates the list of games to broadcast.
func (b *Broadcaster) OnGamesChanged(games []game.Game) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.games = games
}

// Close closes the broadcaster.
func (b *Broadcaster) Close() error {
	return b.conn.Close()
}

// broadcastGames sends raw GameInfo packets for all remote games,
// and DecreateGame for any games that have been removed.
func (b *Broadcaster) broadcastGames() {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentKeys := make(map[string]uint32)

	for i := range b.games {
		g := &b.games[i]

		if g.Source != game.SourceRemote {
			continue
		}

		key := g.Key()
		currentKeys[key] = g.Info.HostCounter

		// Forward raw packet with modified port
		b.sendRawGameInfo(g)

		// Send RefreshGame to update player counts
		b.sendRefreshGame(g.Info.HostCounter, g.Info.SlotsUsed, g.Info.SlotsAvailable)
	}

	// Send DecreateGame for removed games
	for key, hostCounter := range b.previousGameKeys {
		if _, exists := currentKeys[key]; !exists {
			b.sendDecreateGame(hostCounter)

			slog.Debug("sent game cancellation",
				"key", key,
				"hostCounter", hostCounter,
			)
		}
	}

	b.previousGameKeys = currentKeys
}

// sendRawGameInfo forwards the raw GameInfo packet with the port modified.
func (b *Broadcaster) sendRawGameInfo(g *game.Game) {
	if len(g.RawData) < minPacketSize {
		slog.Debug("skipping game with no raw data", "game", g.Info.GameName)

		return
	}

	// Copy raw data to avoid modifying the original
	data := make([]byte, len(g.RawData))
	copy(data, g.RawData)

	// Modify port at last 2 bytes (little-endian uint16)
	portIdx := len(data) - portFieldSize
	data[portIdx] = byte(b.proxyPort)
	data[portIdx+1] = byte(b.proxyPort >> byteShift8)

	// Only send to broadcast address - sending to both broadcast and localhost
	// causes WC3 to show duplicate games
	_, err := b.conn.WriteTo(data, b.broadcastAddr)
	if err != nil {
		slog.Debug("failed to broadcast game", "game", g.Info.GameName, "error", err)
	}

	slog.Debug("broadcast game",
		"name", g.Info.GameName,
		"hostCounter", g.Info.HostCounter,
		"proxyPort", b.proxyPort,
	)
}

// sendRefreshGame sends a RefreshGame (0x32) packet to update player counts.
func (b *Broadcaster) sendRefreshGame(hostCounter, slotsUsed, slotsAvailable uint32) {
	packet := []byte{
		0xF7, 0x32, 0x10, 0x00, // Header: magic, opcode, length=16
		byte(hostCounter), byte(hostCounter >> byteShift8),
		byte(hostCounter >> byteShift16), byte(hostCounter >> byteShift24),
		byte(slotsUsed), byte(slotsUsed >> byteShift8),
		byte(slotsUsed >> byteShift16), byte(slotsUsed >> byteShift24),
		byte(slotsAvailable), byte(slotsAvailable >> byteShift8),
		byte(slotsAvailable >> byteShift16), byte(slotsAvailable >> byteShift24),
	}

	_, err := b.conn.WriteTo(packet, b.broadcastAddr)
	if err != nil {
		slog.Debug("failed to send refresh", "error", err)
	}
}

// sendDecreateGame sends a DecreateGame (0x33) packet to notify game removal.
func (b *Broadcaster) sendDecreateGame(hostCounter uint32) {
	packet := []byte{
		0xF7, 0x33, 0x08, 0x00, // Header: magic, opcode, length=8
		byte(hostCounter), byte(hostCounter >> byteShift8),
		byte(hostCounter >> byteShift16), byte(hostCounter >> byteShift24),
	}

	_, err := b.conn.WriteTo(packet, b.broadcastAddr)
	if err != nil {
		slog.Debug("failed to send decreate", "error", err)
	}
}
