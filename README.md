# CLIP - Common Lightning-node Information Payload (via Nostr)

**⚠️ Early Stage Project: This is a first protocol draft and experimental implementation.**

CLIP is a proposed protocol and CLI tool for publishing and discovering verifiable Lightning Network node information over Nostr. It aims to enable Lightning node operators to share extended metadata about their nodes (contact information, policies, requirements, etc.) in a decentralized manner.

## Table of Contents

- [Problem Statement](#problem-statement)
- [Protocol Description](#protocol-description)
  - [Event Tags](#event-tags)
  - [Message Types](#message-types)
- [Installation](#installation)
- [Getting Started](#getting-started)
  - [Generating a Nostr Key](#generating-a-nostr-key)
  - [Configuration](#configuration)
- [Usage](#usage)
  - [CLI Commands](#cli-commands)
  - [Publishing Node Information](#publishing-node-information)
  - [Querying Node Information](#querying-node-information)
- [Operating Modes](#operating-modes)
  - [LND Mode](#lnd-mode)
  - [Interactive Mode](#interactive-mode)
- [Example Configuration](#example-configuration)
- [License](#license)


## Problem Statement

Lightning Network nodes currently have limited ways to share operational information beyond what's available in the gossip protocol. Node operators may want to:

- Share contact information (Nostr, email, other channels), which also works in case the node is offline.
- Announce operational policies (acceptance criteria, closing policies, maintenance schedules).
- Provide arbitrary information useful for the other nodes.

While it's already possible to share such information via Nostr or centralized directories, there is no standardized way to cryptographically verify that the information actually originates from the operator of a specific Lightning node. CLIP addresses this verification challenge by linking Lightning node signatures with Nostr identities.

## Protocol Description

CLIP uses Nostr events with kind **38171** (addressable event) to publish Lightning node information. As an addressable event, relays only need to store the most recent version for each unique combination of `d` tag and author pubkey (npub), automatically replacing older versions.

### Event Tags

- **`d` tag** (identifier): Unique identifier for the event
  - For Node Announcements: `<lightning_pubkey>`
  - For Node Info: `<kind>:<lightning_pubkey>:<network>` (e.g., `1:03abc...def:mainnet`)
  
- **`k` tag** (kind): CLIP message kind
  - `0` = Node Announcement (trust anchor, requires Lightning signature)
  - `1` = Node Info (metadata, no Lightning signature required)

- **`sig` tag** (Lightning signature): Present only on Node Announcements
  - Format: zbase32-encoded signature created by the Lightning node's identity key
  - Signs the Nostr event ID (hex-encoded SHA256 hash of the event without the `sig` tag). This hash is computed over the event fields including the Nostr public key and the Lightning node public key (`d` tag)
  - During verification, the signing node's public key can be recovered from the signature and compared with the public key in the `d` tag to ensure authenticity

### Message Types

**Node Announcement (Kind 0)** - An announcement with empty content that serves as a trust anchor for a Lightning node. This message must be signed by both the Lightning node's identity key (via the `sig` tag) and a Nostr key (standard Nostr signature). Once published, it links the Lightning node's public key to a specific Nostr public key (npub).

If a new Node Announcement is published with a different Nostr key, relays will store both announcements (different author pubkeys). However, CLIP clients must only accept messages signed by the most recently announced Nostr key (determined by `created_at` timestamp), as the previous Nostr key may have been compromised. All other messages signed with the previous Nostr key must be rejected by the client.

**Node Info (Kind 1)** - Contains detailed information about the Lightning node (contact info, channel policies, operational metadata, etc.). The content is structured as JSON with predefined fields to ensure consistent parsing and interpretation across different users. The `custom_records` field allows for arbitrary key-value pairs beyond the standardized fields. This message type does not require a Lightning signature and only needs to be signed by the Nostr key that was bound in the Node Announcement.

Example content structure:
```json
{
  "about": "Human-readable description of the node",
  "max_channel_size_sat": 16777215,
  "min_channel_size_sat": 40000,
  "contact_info": [
    {
      "type": "nostr",
      "value": "npub1...",
      "note": "Primary contact method",
      "primary": true
    },
    {
      "type": "email",
      "value": "node@example.com"
    }
  ],
  "custom_records": {
    "acceptance_policy": "Accepting all channel requests",
    "closing_policy": "Will not force-close channels",
    "scheduled_maintenance": "First Sunday of each month, 02:00-04:00 UTC"
  }
}
```

## Installation

```bash
# Clone the repository
git clone https://github.com/feelancer21/clip.git
cd clip

# Install the CLI tool
make install
```

## Getting Started

### Generating a Nostr Key

`clip-cli` requires a Nostr private key (nsec) to sign events. You can either:

1. **Generate a new key**:
   ```bash
   clip-cli generatekey
   ```
   This creates a new key and saves it to `~/.config/clip/key`

2. **Use an existing key**: If you already have a Nostr key, save it to the key file:
   ```bash
   echo "nsec1your_private_key_here" > ~/.config/clip/key
   chmod 600 ~/.config/clip/key
   ```

**Note**: You can use any Nostr private key. It doesn't have to be generated by `clip-cli`. However, keep in mind that this key will be permanently associated with your Lightning node via the Node Announcement.

### Configuration

Create a configuration file at `~/.config/clip/config.yaml`:

```bash
mkdir -p ~/.config/clip
nano ~/.config/clip/config.yaml
```

See [Example Configuration](#example-configuration) below for a complete configuration template.

## Usage

### CLI Commands

```
NAME:
   clip-cli - CLIP (Common Lightning-node Information Payloader) - Sending and receiving verifiable Lightning node information over Nostr.

USAGE:
   clip-cli [global options] command [command options]

COMMANDS:
   getinfo                     Returns basic information about the connected Lightning node.
   generatekey                 Generates a new private key for Nostr.
   listnodeannouncements, lna  Fetches all node announcement events from the configured Nostr relays and displays them.
   listnodeinfo, lni           Fetches all node information from the configured Nostr relays and displays it.
   pubnodeannounce, pna        Publishes a node announcement event to the configured Nostr relays.
   pubnodeinfo, pni            Publishes the node information specified in the config to the configured Nostr relays.
   help, h                     Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --config value  name of the config file (default ~/.config/clip/config.yaml)
   --help, -h      show help
   --version, -v   print the version
```

### Publishing Node Information

#### Step 1: Publish Node Announcement

First, publish a Node Announcement to link your Lightning node to your Nostr identity:

```bash
clip-cli pubnodeannounce
# or
clip-cli pna
```

This command:
- Creates a Node Announcement event
- Signs it with your Lightning node's identity key
- Signs it with your Nostr key
- Publishes it to the configured relays

#### Step 2: Publish Node Info

After the announcement, you can publish your node's metadata:

```bash
clip-cli pubnodeinfo
# or
clip-cli pni
```

This publishes the `node_info` section from your configuration file to the relays.

Node Info events do not require a Lightning signature. They only need to be signed by the Nostr key that was bound in the Node Announcement.

### Querying Node Information

#### List Node Announcements

```bash
# List all node announcements
clip-cli listnodeannouncements
# or 
clip-cli lna

# List announcements from the last 7 days (default: 60 days)
clip-cli lna --since 168h

# List for a specific Lightning node (default: all nodes)
clip-cli lna --pubkey 03abc...def
```

#### List Node Info

```bash
# List all node info
clip-cli listnodeinfo
# or 
clip-cli lni

# Filter by time and node
clip-cli lni --since 24h --pubkey 03abc...def

```

## Operating Modes

### LND Mode

Connects directly to an LND node via gRPC for automated signing.
```yaml
lnclient: "lnd"
lnd:
  host: "localhost"
  port: 10009
  tls_cert_path: "/path/to/your/tls.cert"
  macaroon_path: "/path/to/your/macaroon.macaroon"
```
The provided macaroon must have permissions for `lnrpc.SignMessage`, `lnrpc.GetInfo` and `lnrpc.GetNodeInfo`. An `admin.macaroon` is not strictly required but is the most convenient option.

### Interactive Mode

Prompts for manual signing, making it compatible with any Lightning implementation (e.g., LND, CLN, Eclair).
```yaml
lnclient: "interactive"
interactive:
  network: "mainnet"
  pub_key: "03abc...def"
```
Use `lncli signmessage` or another API to sign the message prompted by the CLI.

## Example Configuration

A complete example configuration file can be found in [`config.example.yaml`](config.example.yaml). You can copy this file to `~/.config/clip/config.yaml` and edit it to your needs.

### Configuration Notes

- **Relay URLs**: Choose a mix of well-known relays for reliability.

- **Privacy**: Be mindful of the information you share publicly. Only include what you are comfortable making available to anyone on the internet. For enhanced privacy, consider running your communication with Nostr relays through a VPN to obfuscate your IP address.
  
- **Node Info Fields**: All fields under `node_info` are optional. Only include information you want to make public.
  
- **Contact Info**: You can have multiple contact methods. Set `primary: true` on your preferred method (only one contact can be primary).

- **Security**: The Nostr key is stored as plain text. Keep your key file (`key_store_path`) secure with appropriate file permissions (600). Back up your Nostr private key securely, as it allows you to publish information even when your Lightning node is down.

## License

See [LICENSE](LICENSE) file for details.
