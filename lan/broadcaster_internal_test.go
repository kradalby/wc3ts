package lan

import (
	"bytes"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"

	"github.com/kradalby/wc3ts/game"
)

const testProxyPort = 0x1234

// rawGameInfo stands in for a GameInfo packet; only the trailing port matters.
func rawGameInfo() []byte {
	return []byte{0xF7, 0x30, 0x08, 0x00, 0xAA, 0xBB, 0x00, 0x00}
}

func remoteGame(hostCounter uint32) game.Game {
	return game.Game{
		Info:    w3gs.GameInfo{GameName: "remote", HostCounter: hostCounter},
		RawData: rawGameInfo(),
		Source:  game.SourceRemote,
		PeerIP:  netip.MustParseAddr("100.64.0.1"),
	}
}

// newTestBroadcaster returns a broadcaster whose "LAN" is a loopback socket.
func newTestBroadcaster(t *testing.T, registry *game.Registry) (*Broadcaster, *net.UDPConn) {
	t.Helper()

	b, err := NewBroadcaster(testProxyPort, registry)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = b.Close() })

	lan, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = lan.Close() })

	addr, ok := lan.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr() = %T, want *net.UDPAddr", lan.LocalAddr())
	}

	b.broadcastAddr = addr

	return b, lan
}

func readPacket(t *testing.T, conn *net.UDPConn) []byte {
	t.Helper()

	err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 512)

	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	return buf[:n]
}

func TestBroadcastReadsRegistry(t *testing.T) {
	t.Parallel()

	registry := game.NewRegistry(nil)
	registry.Add(remoteGame(2))

	b, lan := newTestBroadcaster(t, registry)
	b.broadcastGames()

	want := []byte{0xF7, 0x30, 0x08, 0x00, 0xAA, 0xBB, 0x34, 0x12}
	if got := readPacket(t, lan); !bytes.Equal(got, want) {
		t.Fatalf("GameInfo = % x, want % x", got, want)
	}

	if got := readPacket(t, lan); got[1] != 0x32 || got[4] != 2 {
		t.Fatalf("RefreshGame = % x, want opcode 0x32 for hostCounter 2", got)
	}

	if raw := registry.Games()[0].RawData; !bytes.Equal(raw, rawGameInfo()) {
		t.Fatalf("registry RawData patched in place: % x", raw)
	}
}
