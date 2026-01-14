# wc3ts - Warcraft III LAN over Tailscale

Automatically discover and join Warcraft III LAN games across your Tailscale network.

Built for classic pre-Reforged Warcraft III (1.26-1.29) - the version we use at LAN parties.

## Features

- **Automatic discovery**: No manual IP configuration needed
- **Peer-to-peer**: All nodes run the proxy, games appear automatically
- **Version detection**: Automatically detects your WC3 version from network traffic
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
3. Advertise remote games to the local LAN
4. Proxy TCP connections to remote game hosts

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
3. Remote games appear with `[hostname]` prefix in the game list

The proxy will automatically:
- Detect your WC3 version from network traffic
- Discover peers on your Tailscale network
- Advertise remote games to your local LAN
- Proxy connections to remote hosts

## Requirements

- Tailscale installed and connected
- Warcraft III 1.26 - 1.29 (auto-detected)

## How It Works

### Game Discovery

When you search for LAN games in WC3, `wc3ts` intercepts the `SearchGame` broadcast and:
1. Extracts the game version for future probing
2. Responds with any remote games discovered from peers

### Peer Probing

`wc3ts` periodically sends `SearchGame` packets to all online Tailscale peers. When a peer has a hosted game, their `wc3ts` instance responds with `GameInfo` packets.

### Game Advertising

Remote games are advertised locally using the WC3 LAN protocol. The game port is replaced with the local TCP proxy port, so connections are automatically forwarded to the remote host.

### Connection Proxying

When you join a remote game, WC3 connects to the local TCP proxy. The proxy looks up the actual host from the game registry and establishes a connection via Tailscale.

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
