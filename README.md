# wc3ts - Warcraft III LAN over Tailscale

Automatically discover and join Warcraft III LAN games across your Tailscale network.

Built for classic pre-Reforged Warcraft III (1.26-1.29) - the version we use at LAN parties.

## Features

- **Automatic discovery**: No manual IP configuration needed
- **Peer-to-peer**: All nodes run the proxy, games appear automatically
- **Raw packet forwarding**: Preserves exact game data including HostCounter
- **Real-time updates**: Uses Tailscale IPN bus for instant peer notifications
- **Cross-platform**: Works on macOS, Linux, and Windows

## Architecture

```
┌─────────────┐    Tailscale    ┌─────────────┐
│   Host PC   │◄───────────────►│  Client PC  │
│   (WC3)     │                 │   (WC3)     │
│   wc3ts     │                 │   wc3ts     │
└─────────────┘                 └─────────────┘
```

Each machine runs `wc3ts` alongside Warcraft III. The proxies:

1. Subscribe to Tailscale peer updates via the IPN bus
2. Probe peers for hosted games using WC3 LAN protocol
3. Respond to queries from remote peers with local game info
4. Broadcast remote games to the local LAN (raw packets)
5. Proxy TCP connections to remote game hosts

## Installation

### Using Nix (recommended)

```bash
nix develop  # Enter dev shell
make build   # Build the binary
```

### Manual

```bash
go build -o wc3ts ./cmd/wc3ts
```

## Usage

1. Start `wc3ts` on all machines in your Tailscale network
2. Start Warcraft III and create/join LAN games as normal
3. Remote games appear in your LAN game list automatically

The proxy will automatically:
- Discover peers on your Tailscale network
- Probe peers and localhost for hosted games
- Broadcast remote games to your local LAN
- Proxy connections to remote hosts

## Requirements

- Tailscale installed and connected
- Warcraft III 1.26 - 1.29 (defaults to 1.26)

## How It Works

### Peer Probing

`wc3ts` periodically sends `SearchGame` packets to all online Tailscale peers and localhost. When a peer has a hosted game, their `wc3ts` responder sends back `GameInfo` packets. Raw packet bytes are preserved for accurate forwarding.

### Query Response

When a remote peer probes us, our responder replies with any locally hosted games. This enables bidirectional discovery - you can join their games and they can join yours.

### Game Broadcasting

Remote games are broadcast to the local LAN using raw packet forwarding. Only the game port is modified to point to our TCP proxy. This preserves the exact `HostCounter` value that WC3 uses to identify games.

### Connection Proxying

When you join a remote game, WC3 connects to our TCP proxy. The proxy reads the `Join` packet to extract the `HostCounter`, looks up the corresponding game in the registry, and forwards the connection to the actual remote host via Tailscale.

## Credits & Acknowledgements

This project builds upon excellent prior work:

### Libraries

- **[gowarcraft3](https://github.com/nielsAD/gowarcraft3)** by Niels A.D.
  - Complete Warcraft III protocol implementation in Go
  - LAN discovery and advertising
  - Licensed under Mozilla Public License 2.0

- **[Tailscale](https://tailscale.com)**
  - LocalAPI for peer discovery
  - Secure mesh networking

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** by Charm
  - Terminal UI framework

### Prior Art / Inspiration

- **[WC3LanGame](https://github.com/Qyperion/WC3LanGame)** by Qyperion
  - C# WC3 LAN proxy with GUI
  - Protocol documentation

- **[wc3proxy](https://github.com/leonardodino/wc3proxy)** by Leonardo Dino
  - Original C# CLI proxy implementation

[Claude](https://claude.ai) was used to build this project.

## License

BSD-3-Clause
