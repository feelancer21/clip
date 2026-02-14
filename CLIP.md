CLIP
====

Common Lightning-node Information Payload
-----------------------------------------

`draft` `optional`

## Abstract

CLIP enables Lightning node operators to cryptographically link their node identity to a Nostr pubkey and publish verifiable node metadata (contact information, channel policies, custom records) over Nostr.

CLIP uses Nostr addressable events of kind `38171`, differentiated by a `k` tag into distinct message types. A Lightning node signature provides the trust anchor that binds a Lightning node identity to a Nostr pubkey.

## Event Format

All CLIP events use a single Nostr event kind:

```jsonc
{
  "kind": 38171,
  "pubkey": "<nostr-pubkey-hex>",
  "created_at": <timestamp>,
  "tags": [
    ["d", "<identifier>"],
    ["k", "<clip-kind>"],
    // additional tags depending on kind
  ],
  "content": "<json-payload-or-empty>",
  "sig": "<nostr-signature>"
}
```

As addressable events, relays only store the most recent version for each unique combination of `d` tag and author pubkey, automatically replacing older versions.

### Tag Definitions

| Tag   | Required | Description |
|-------|----------|-------------|
| `d`   | yes      | Unique identifier. Format depends on the CLIP kind (see Message Types). |
| `k`   | yes      | CLIP message kind as a string (`"0"`, `"1"`, ...). |
| `sig` | kind 0   | Lightning node signature. zbase32-encoded ECDSA signature over the event hash computed _without_ the `sig` tag (see Signature Verification). Present only on Node Announcements. |

### Validation Rules

In addition to standard Nostr event validation (NIP-01), clients MUST enforce:

- `content` MUST NOT exceed 1,048,576 bytes (1 MB).
- For Kind 1 events, the `k` tag value MUST match the kind encoded in the `d` tag.

Clients MAY reject events from Lightning nodes with no on-chain channel capacity as a spam protection measure.

## Message Types

### Kind 0 -- Node Announcement

A Node Announcement is a trust anchor that links a Lightning node's public key to a Nostr pubkey. It MUST be signed by both the Lightning node's identity key (via the `sig` tag) and a Nostr key (standard Nostr event signature).

**`d` tag:** `<lightning_pubkey>`

The `d` tag contains the compressed public key of the Lightning node (66-character hex string).

**`content`:** empty (`""` or `"{}"`)

**Additional tags:**

| Tag   | Description |
|-------|-------------|
| `sig` | zbase32-encoded Lightning signature over the event hash (see Signature Verification). |

**Example:**

```json
{
  "kind": 38171,
  "pubkey": "a1b2c3d4e5f6...",
  "created_at": 1700000000,
  "tags": [
    ["d", "03abc123def456..."],
    ["k", "0"],
    ["sig", "ryz1bnx5..."]
  ],
  "content": "{}",
  "sig": "nostr-signature..."
}
```

### Kind 1 -- Node Info

A Node Info event contains metadata about a Lightning node. It only requires a Nostr signature and MUST be signed by the Nostr key that was bound in the most recent Node Announcement for that Lightning node.

**`d` tag:** `<kind>:<lightning_pubkey>:<network>`

Example: `1:03abc123def456...:mainnet`

Valid networks: `mainnet`, `testnet`, `testnet4`, `signet`, `simnet`, `regtest`.

**`content`:** JSON object with the following schema:

| Field                | Type              | Description |
|----------------------|-------------------|-------------|
| `about`              | string            | Human-readable description of the node. |
| `max_channel_size_sat` | integer         | Maximum channel size the operator is willing to accept (satoshis). |
| `min_channel_size_sat` | integer         | Minimum channel size the operator requires (satoshis). |
| `contact_info`       | array of objects  | Contact methods (see below). At most one entry MAY have `primary: true`. |
| `custom_records`     | object            | Arbitrary string key-value pairs for additional information. |

All fields are optional.

If both `max_channel_size_sat` and `min_channel_size_sat` are present, `max_channel_size_sat` MUST be greater than or equal to `min_channel_size_sat`.

**Contact Info object:**

| Field     | Type    | Required | Description |
|-----------|---------|----------|-------------|
| `type`    | string  | yes      | Contact type (e.g., `"nostr"`, `"email"`, `"telegram"`). |
| `value`   | string  | yes      | Contact address or handle. |
| `note`    | string  | no       | Additional context about this contact method. |
| `primary` | boolean | no       | Whether this is the preferred contact method. At most one contact MAY be primary. |

**Example:**

```json
{
  "kind": 38171,
  "pubkey": "a1b2c3d4e5f6...",
  "created_at": 1700000100,
  "tags": [
    ["d", "1:03abc123def456...:mainnet"],
    ["k", "1"]
  ],
  "content": "{\"about\":\"High-uptime routing node.\",\"max_channel_size_sat\":16777215,\"min_channel_size_sat\":100000,\"contact_info\":[{\"type\":\"nostr\",\"value\":\"npub1...\",\"primary\":true},{\"type\":\"email\",\"value\":\"node@example.com\"}],\"custom_records\":{\"acceptance_policy\":\"Accepting channels from reliable nodes.\"}}",
  "sig": "nostr-signature..."
}
```

## Signature Verification

Node Announcements (Kind 0) require a Lightning node signature in the `sig` tag. This signature binds the Lightning node identity to the Nostr event.

### Signed Message

The signed message is the **event hash** of the event _without_ the `sig` tag. This hash is the Nostr event ID, i.e., the hex-encoded SHA-256 of the canonical JSON serialization `[0, pubkey, created_at, kind, tags_without_sig, content]`.

Specifically, the `sig` tag is excluded from the tags array before computing the event ID. This means the signature covers the Nostr pubkey, the Lightning pubkey (in the `d` tag), the timestamp, and all other event content.

### Signature Format

The signature follows the LND `SignMessage` RPC format:

1. The message bytes are prefixed with `"Lightning Signed Message:"` (no null terminator).
2. The prefixed message is double-SHA-256 hashed: `SHA256(SHA256("Lightning Signed Message:" || message))`.
3. The result is signed with the node's identity key using ECDSA (secp256k1), producing a compact recoverable signature.
4. The signature is encoded as zbase32.

### Verification Procedure

To verify a Node Announcement:

1. Extract the `sig` tag value and decode from zbase32.
2. Compute the event hash (event ID) over the event _without_ the `sig` tag.
3. Prefix the hash with `"Lightning Signed Message:"` and compute the double-SHA-256.
4. Recover the public key from the compact ECDSA signature.
5. Compare the recovered public key (compressed, hex-encoded) against the `d` tag value.
6. If they match, the signature is valid.

## Trust Model

The trust model ensures that only the legitimate node operator can publish information for a given Lightning node.

### Establishing Trust

A Node Announcement (Kind 0) establishes which Nostr pubkey is authorized to publish events for a Lightning node. The Lightning signature proves the announcement was created by (or authorized by) the node operator.

### Key Rotation (nsec Compromise)

If a node operator's Nostr key is compromised, they can publish a new Node Announcement with a different Nostr pubkey. Clients MUST handle this as follows:

1. Among all Node Announcements for a given Lightning pubkey, only the one with the most recent `created_at` is considered valid.
2. If the new announcement uses a different Nostr pubkey than the previous one, **all prior events** for that node MUST be discarded.
3. Only events signed by the Nostr pubkey from the most recent announcement are accepted.

Note: Since Node Announcements are addressable events with different Nostr author pubkeys, relays will store both the old and new announcements. Clients are responsible for selecting the most recent one.

### Event Acceptance

Before accepting a Kind 1 event, clients MUST:

1. Fetch all Node Announcements for the corresponding Lightning pubkey to establish the trust anchor.
2. Determine the most recent Node Announcement (by `created_at`).
3. Reject the event if its Nostr pubkey does not match the most recent announcement's Nostr pubkey.

## References

- [Nostr Protocol (NIP-01)](https://github.com/nostr-protocol/nips/blob/master/01.md) -- Basic protocol
- [Addressable Events (NIP-33)](https://github.com/nostr-protocol/nips/blob/master/33.md) -- Parameterized replaceable events
- [clip](https://github.com/feelancer21/clip) -- Reference implementation
