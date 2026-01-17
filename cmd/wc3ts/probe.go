//nolint:forbidigo,mnd // Debug tool uses fmt.Print and has magic numbers
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/kradalby/wc3ts/config"
	"github.com/nielsAD/gowarcraft3/network"
	"github.com/nielsAD/gowarcraft3/protocol"
	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
	"github.com/peterbourgon/ff/v3/ffcli"
)

// Silence unused import warning - network is used for W3GSPacketConn.
var _ = network.W3GSPacketConn{}

// Errors for the probe command.
var (
	errNoHosts        = errors.New("at least one host required")
	errUnknownProduct = errors.New("unknown product (use W3XP or WAR3)")
	errPacketTooShort = errors.New("packet too short")
	errNotGameInfo    = errors.New("not a GameInfo packet")
)

func newProbeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	timeout := fs.Duration("timeout", 5*time.Second, "Response timeout")
	versionStr := fs.String("version", "26", "Game version (e.g., 26, 1.26, 27, 1.27, 28, 1.28)")
	product := fs.String("product", "W3XP", "Product code (W3XP for TFT, WAR3 for ROC)")

	return &ffcli.Command{
		Name:       "probe",
		ShortUsage: "wc3ts probe [flags] <host> [host...]",
		ShortHelp:  "Probe hosts for WC3 games",
		LongHelp: `Send SearchGame packets to one or more hosts and display any games found.

Version can be specified as "26" or "1.26" (both work).

Examples:
  wc3ts probe 127.0.0.1                  # Probe localhost (default: v1.26)
  wc3ts probe 100.64.0.1                 # Probe a Tailscale peer
  wc3ts probe 192.168.1.10 192.168.1.11  # Probe multiple hosts
  wc3ts probe -version 1.28 127.0.0.1    # Use WC3 1.28
  wc3ts probe -version 27 127.0.0.1      # Use WC3 1.27`,
		FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return errNoHosts
			}

			// Parse version
			version, err := config.ParseVersion(*versionStr)
			if err != nil {
				return err
			}

			// Parse product code
			var prod protocol.DWordString

			switch *product {
			case "W3XP", "TFT":
				prod = w3gs.ProductTFT
			case "WAR3", "ROC":
				prod = w3gs.ProductROC
			default:
				return fmt.Errorf("%w: %s", errUnknownProduct, *product)
			}

			return probeHosts(ctx, args, *timeout, prod, version)
		},
	}
}

func probeHosts(
	ctx context.Context,
	hosts []string,
	timeout time.Duration,
	product protocol.DWordString,
	version uint32,
) error {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	defer func() { _ = conn.Close() }()

	w3gsConn := &network.W3GSPacketConn{}
	w3gsConn.SetConn(conn, w3gs.NewFactoryCache(w3gs.DefaultFactory), w3gs.Encoding{})

	searchGame := &w3gs.SearchGame{
		GameVersion: w3gs.GameVersion{
			Product: product,
			Version: version,
		},
		HostCounter: 1,
	}

	fmt.Printf("Probing with: Product=%s Version=1.%d\n\n", product, version)

	sendSearchToHosts(ctx, hosts, w3gsConn, searchGame)

	return receiveResponses(conn, timeout)
}

func sendSearchToHosts(ctx context.Context, hosts []string, w3gsConn *network.W3GSPacketConn, pkt *w3gs.SearchGame) {
	for _, host := range hosts {
		addr := resolveHost(ctx, host)
		if addr == nil {
			continue
		}

		fmt.Printf("Sending SearchGame to %s...\n", addr)

		_, err := w3gsConn.Send(addr, pkt)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		}
	}
}

func resolveHost(ctx context.Context, host string) *net.UDPAddr {
	addr := &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: 6112,
	}

	if addr.IP == nil {
		resolver := &net.Resolver{}

		ips, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			fmt.Printf("Cannot resolve %s: %v\n", host, err)

			return nil
		}

		for _, ip := range ips {
			if ip4 := ip.IP.To4(); ip4 != nil {
				addr.IP = ip4

				break
			}
		}
	}

	if addr.IP == nil {
		fmt.Printf("No IPv4 address for %s\n", host)

		return nil
	}

	return addr
}

func receiveResponses(conn *net.UDPConn, timeout time.Duration) error {
	fmt.Printf("\nWaiting for responses (timeout: %s)...\n\n", timeout)

	err := conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	gamesFound := 0
	buf := make([]byte, 4096)

	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				break
			}

			return fmt.Errorf("read error: %w", err)
		}

		gamesFound += handlePacket(buf[:n], from)
	}

	printSummary(gamesFound)

	return nil
}

func handlePacket(data []byte, from *net.UDPAddr) int {
	if len(data) < 4 || data[0] != 0xF7 {
		fmt.Printf("Received non-W3GS data from %s (%d bytes)\n", from, len(data))

		return 0
	}

	packetID := data[1]
	fmt.Printf("Received W3GS packet 0x%02X from %s (%d bytes)\n", packetID, from, len(data))

	if packetID != 0x30 { // Not GameInfo
		return 0
	}

	gameInfo, err := parseGameInfo(data)
	if err != nil {
		fmt.Printf("  Failed to parse: %v\n", err)
		fmt.Printf("  Raw: %x\n", data)

		return 0
	}

	printGameInfo(gameInfo, from)

	return 1
}

func printGameInfo(gi *w3gs.GameInfo, from *net.UDPAddr) {
	fmt.Println()
	fmt.Printf("=== Game Found ===\n")
	fmt.Printf("  From:     %s\n", from)
	fmt.Printf("  Name:     %s\n", gi.GameName)
	fmt.Printf("  Map:      %s\n", gi.GameSettings.MapPath)
	fmt.Printf("  Players:  %d/%d\n", gi.SlotsUsed, gi.SlotsTotal)
	fmt.Printf("  Port:     %d\n", gi.GamePort)
	fmt.Printf("  Version:  %s 1.%d\n", gi.Product, gi.Version)
	fmt.Printf("  HostCtr:  %d\n", gi.HostCounter)
	fmt.Println()
}

func printSummary(count int) {
	if count == 0 {
		fmt.Println("No games found.")
	} else {
		fmt.Printf("Found %d game(s).\n", count)
	}
}

// parseGameInfo parses a raw GameInfo packet.
func parseGameInfo(data []byte) (*w3gs.GameInfo, error) {
	if len(data) < 4 {
		return nil, errPacketTooShort
	}

	decoder := w3gs.NewDecoder(w3gs.Encoding{}, w3gs.NewFactoryCache(w3gs.DefaultFactory))

	pkt, _, err := decoder.Deserialize(data)
	if err != nil {
		return nil, err
	}

	gi, ok := pkt.(*w3gs.GameInfo)
	if !ok {
		return nil, errNotGameInfo
	}

	return gi, nil
}
