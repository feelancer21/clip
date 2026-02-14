# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is CLIP?

CLIP (Common Lightning-node Information Payload) is a protocol and CLI tool for publishing and discovering verifiable Lightning Network node information over Nostr. It uses Nostr addressable events (kind 38171) with Lightning node signatures to cryptographically link node metadata to node identities.

## Build & Run

```bash
make build      # builds ./clip-cli binary
make install    # installs clip-cli to $GOPATH/bin
go test ./...   # run all tests (no tests exist yet)
```

Version is injected via `-ldflags` from `git rev-parse --short HEAD`.

## Architecture

### Package Structure

- **Root package (`clip`)**: Core library with protocol logic, client, event handling, store, and Lightning node abstraction.
- **`cmd/clip-cli`**: CLI application using `urfave/cli/v2`. Wires config, keystore, and the core library together.

### Key Interfaces

- **`LightningNode`** (`lightning.go`): Abstraction for Lightning node interaction — signing messages, getting node info, looking up aliases. Embeds `LnSigner`.
- **`EventSigner`** (`signer.go`): Signs events with both Nostr and Lightning keys. `CombinedSigner` is the concrete implementation.

### LightningNode Implementations

- **`LND`** (`lnd.go`): Connects to LND via gRPC with TLS + macaroon auth. Requires `lnrpc.SignMessage`, `lnrpc.GetInfo`, `lnrpc.GetNodeInfo` permissions.
- **`LnInteractive`** (`lninter.go`): Prompts the user to manually sign messages via stdin/stdout. Works with any Lightning implementation.

### Event Model (`event.go`)

All CLIP data uses a single Nostr event kind (38171) differentiated by a `k` tag:
- **Kind 0 (Node Announcement)**: Trust anchor linking a Lightning pubkey to a Nostr pubkey. Requires a Lightning signature (`sig` tag) verified via ECDSA recovery (zbase32-encoded, using LND's `signedMsgPrefix`). The `d` tag is the Lightning pubkey.
- **Kind 1 (Node Info)**: Node metadata (contact info, channel policies, custom records). Only requires Nostr signature. The `d` tag format is `kind:pubkey:network`.

Verification recovers the signer's public key from the signature and compares it against the `d` tag pubkey.

### Store (`store.go`)

`MapStore` is an in-memory store keyed by Lightning node pubkey. It enforces the trust model:
- Only the most recent Node Announcement's Nostr pubkey is trusted.
- If a new announcement uses a different Nostr pubkey, all prior events for that node are purged (handles nsec compromise).
- Non-announcement events are rejected unless their Nostr pubkey matches the latest announcement.

### Client (`client.go`)

`Client` orchestrates everything: holds a `nostr.SimplePool`, `MapStore`, `EventSigner`, and `LightningNode`. When fetching events, it always syncs Node Announcements first (to establish trust), then fetches the requested kind. Uses a three-return-value pattern `(result, error, []error)` where the third value collects non-fatal per-event errors for resilient partial-success operation.

### Payload Types (`payloads.go`)

`NodeInfo` struct defines the standardized fields (about, channel size limits, contact info, custom records). Validation uses `go-playground/validator` with a custom check ensuring at most one primary contact.

### Config & Keystore (`cmd/clip-cli/`)

- Config is YAML with strict unknown-field rejection (`decoder.KnownFields(true)`).
- Default paths: `~/.config/clip/config.yaml` and `~/.config/clip/key`.
- Nostr private key stored as nsec (bech32-encoded) in a plain text file with 0600 permissions.
