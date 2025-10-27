package clip

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/tv42/zbase32"
)

type Kind int

const (
	// Define a new nip1 kind
	KindLightningInformation int = 38171

	KindNodeAnnouncement Kind = 0
	KindNodeInfo         Kind = 1

	MaxContentSize = 1 * 1024 * 1024 // 1 MB

	EventGracePeriodSeconds = 600 // 10 minutes
)

var (
	// Prefix used by lnd.
	signedMsgPrefix = []byte("Lightning Signed Message:")
)

type Event struct {
	NostrEvent *nostr.Event

	// Set during Finalize
	kind      Kind
	finalized bool

	// Identifier for the event
	id *Identifier
}

func NewEventFromNostrRelay(ev *nostr.Event) (*Event, error) {
	e := &Event{
		NostrEvent: ev,
	}
	id, err := e.GetIdentifier()
	if err != nil {
		return nil, fmt.Errorf("getting identifier: %w", err)
	}
	e.kind = id.Kind
	e.finalized = true
	return e, nil
}

// Hash returns the hash of the event to be signed. We use the ID of a preliminary event,
// which is the hex-encoded sha256 of the serialized event without the 'sig' tags.
func (e *Event) Hash() []byte {
	return []byte(e.copyWithoutSig().GetID())
}

// copyWithoutSig removes the signatures from the event and returns a copy.
func (e *Event) copyWithoutSig() *nostr.Event {
	var filteredTags nostr.Tags
	for _, tag := range e.NostrEvent.Tags {
		if len(tag) > 0 && tag[0] == "sig" {
			continue
		}
		filteredTags = append(filteredTags, tag)
	}

	return &nostr.Event{
		PubKey:    e.NostrEvent.PubKey,
		CreatedAt: e.NostrEvent.CreatedAt,
		Kind:      e.NostrEvent.Kind,
		Tags:      filteredTags,
		Content:   e.NostrEvent.Content,
	}
}

func (e *Event) Finalize(network string, pubkey string, kind Kind, opts []string) error {
	ev := e.NostrEvent
	for _, t := range []string{"d", "k"} {
		if ev.Tags.Find(t) != nil {
			return fmt.Errorf("event already has a '%s' tag", t)
		}
	}

	kindStr := strconv.Itoa(int(kind))

	// Constructing the "d" tag  --
	var tagD string
	switch kind {
	case KindNodeAnnouncement:
		// "d" tag is just the pubkey for node announcements
		tagD = pubkey
	default:
		// otherwise kind:pubkey:network:opts...
		parts := append([]string{kindStr, pubkey, network}, opts...)
		tagD = strings.Join(parts, ":")
	}

	e.kind = kind
	ev.Kind = KindLightningInformation
	ev.Tags = append(ev.Tags,
		nostr.Tag{"d", tagD},
		nostr.Tag{"k", kindStr},
	)
	e.finalized = true
	return nil
}

func (e *Event) IsFinalized() bool {
	return e.finalized
}

func (e *Event) Verify() (bool, error) {
	createdAtLimitUpper := nostr.Now() + EventGracePeriodSeconds
	if e.NostrEvent.CreatedAt > createdAtLimitUpper {
		return false, fmt.Errorf("event is too far in the future")
	}

	// See https://github.com/nbd-wtf/go-nostr/pull/119
	if e.NostrEvent.ID != e.NostrEvent.GetID() {
		return false, fmt.Errorf("event ID mismatch")
	}

	// Checking that the content size is within limits
	if len(e.NostrEvent.Content) > MaxContentSize {
		return false, fmt.Errorf("content size exceeds (%d bytes) maximum limit (%d bytes)",
			len(e.NostrEvent.Content), MaxContentSize)
	}

	// Checking that the public key matches the one in the 'd' tag
	idx, err := e.GetIdentifier()
	if err != nil {
		return false, err
	}

	if idx.Kind != KindNodeAnnouncement {
		if !IsValidNetwork(idx.Network) {
			return false, fmt.Errorf("invalid network: %s", idx.Network)
		}
	}

	k := e.NostrEvent.Tags.Find("k")
	// Integrity checks
	if k == nil || len(k) < 2 || k[1] != strconv.Itoa(int(idx.Kind)) {
		return false, fmt.Errorf("missing or invalid 'k' tag")
	}

	// Checking nostr signature first
	if ok, err := e.NostrEvent.CheckSignature(); err != nil || !ok {
		return false, err
	}

	if e.RequiresLnSignature() {
		ok, err := e.checkLightningSig(idx.PubKey)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (e *Event) checkLightningSig(pubKeyID string) (bool, error) {
	// Extracting the ln signature and checking there is exactly one

	var sigs []nostr.Tag
	// GetAll deprecated, using FindAll instead
	for t := range e.NostrEvent.Tags.FindAll("sig") {
		sigs = append(sigs, t)
	}
	if len(sigs) > 1 {
		return false, fmt.Errorf("more than one 'sig' tag")
	}
	if len(sigs) == 0 {
		return false, fmt.Errorf("no 'sig' tag found")
	}
	sig := sigs[0][1]

	// Verifying signature according to lnd's code
	// https://github.com/lightningnetwork/lnd/blob/9a7b526c0cf35ebf03d91c773dbaa0ce7d20f323/rpcserver.go#L1762
	s, err := zbase32.DecodeString(sig)
	if err != nil {
		return false, fmt.Errorf("decoding signature: %w", err)
	}

	msg := e.Hash()
	b := chainhash.DoubleHashB(append(signedMsgPrefix, msg[:]...))

	pubKey, _, err := ecdsa.RecoverCompact(s, b)
	if err != nil {
		return false, fmt.Errorf("recovering public key: %w", err)
	}

	pubKeyHex := hex.EncodeToString(pubKey.SerializeCompressed())
	if pubKeyHex != pubKeyID {
		return false, fmt.Errorf("public key does not match")
	}
	return true, nil
}

func (e *Event) RequiresLnSignature() bool {
	switch e.kind {
	case KindNodeAnnouncement:
		return true
	}
	return false
}

type Identifier struct {
	TagD    string   `json:"tag_d"`
	Network string   `json:"network"`
	PubKey  string   `json:"pub_key"`
	Kind    Kind     `json:"kind"`
	Opts    []string `json:"opts"`
}

func (e *Event) GetIdentifier() (*Identifier, error) {
	if e.id != nil {
		return e.id, nil
	}

	tagD := e.NostrEvent.Tags.Find("d")
	if tagD == nil || len(tagD) < 2 {
		return nil, fmt.Errorf("missing or invalid 'd' tag")
	}

	tagK := e.NostrEvent.Tags.Find("k")
	if tagK == nil || len(tagK) < 2 {
		return nil, fmt.Errorf("missing or invalid 'k' tag")
	}

	kindInt, err := strconv.Atoi(tagK[1])
	if err != nil {
		return nil, fmt.Errorf("invalid kind in 'k' tag: %w", err)
	}
	kind := Kind(kindInt)

	id := &Identifier{
		TagD: tagD[1],
		Kind: kind,
	}

	switch kind {
	case KindNodeAnnouncement:
		id.PubKey = tagD[1]
		id.Opts = []string{}
	default:
		parts := strings.Split(tagD[1], ":")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid 'd' tag format for kind %d", kind)
		}
		
		id.PubKey = parts[1]
		id.Network = parts[2]
		id.Opts = parts[3:]
	}

	e.id = id
	return e.id, nil
}

type EventEnvelope[T any] struct {
	Id        *Identifier `json:"id"`
	Alias     string      `json:"alias"`
	NostrId   string      `json:"nostr_id"`
	Npub      string      `json:"npub"`
	CreatedAt int64       `json:"created_at"`
	Payload   *T          `json:"payload"`
}

func NewEventEnvelope[T any](ev *Event) (*EventEnvelope[T], error) {
	var payload T
	if err := json.Unmarshal([]byte(ev.NostrEvent.Content), &payload); err != nil {
		return nil, err
	}
	id, err := ev.GetIdentifier()
	if err != nil {
		return nil, err
	}

	npub, err := nip19.EncodePublicKey(ev.NostrEvent.PubKey)
	if err != nil {
		return nil, err
	}

	return &EventEnvelope[T]{
		Id:        id,
		NostrId:   ev.NostrEvent.ID,
		CreatedAt: int64(ev.NostrEvent.CreatedAt),
		Npub:      npub,
		Payload:   &payload,
	}, nil
}
