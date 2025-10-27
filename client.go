package clip

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Client struct {
	// Responsible for publishing and subscribing to events
	pool *nostr.SimplePool

	// A simple in-memory store
	store *MapStore

	// Responsible for signing events
	signer EventSigner

	// Responsible for interacting with the Lightning node (signing, getting info, etc)
	ln LightningNode

	// Nostr pubkey
	pub string

	// Cache of the node info
	info NodeInfoResponse
}

func NewClient(ctx context.Context, nostrSigner nostr.Signer, ln LightningNode) (*Client, error) {
	combinedSigner := &CombinedSigner{
		NostrSigner: nostrSigner,
		LnSigner:    ln,
	}

	c := &Client{
		pool:   nostr.NewSimplePool(ctx),
		store:  NewMapStore(),
		signer: combinedSigner,
		ln:     ln,
	}

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Caching the nostr public key now
	pub, err := nostrSigner.GetPublicKey(initCtx)
	if err != nil {
		return nil, err
	}
	c.pub = pub

	// Caching info of the node now (network, pubkey, etc)
	info, err := c.GetNodeInfo(initCtx)
	if err != nil {
		return nil, err
	}
	c.info = info
	return c, nil
}

func (c *Client) GetNodeInfo(ctx context.Context) (NodeInfoResponse, error) {
	info, err := c.ln.GetNodeInfo(ctx)
	if err == nil && !info.checkNetwork() {
		err = fmt.Errorf("invalid network: %s", info.Network)
	}
	return info, err
}

// GetEvents fetches events from relays and returns them along with any errors encountered.
// Return values: ([]*Event, error, []error)
// - error (2nd return): Critical errors that prevent operation (returned immediately)
// - []error (3rd return): Non-fatal warnings collected during processing (fetchErrors)
//
// fetchErrors are designed NOT to interrupt execution. They collect recoverable issues like:
// - Individual event verification failures
// - Event storage failures
// - Relay-specific errors
// This allows partial success: valid events are returned even if some events/relays fail.
func (c *Client) GetEvents(ctx context.Context, kind Kind, pubkeys map[string]struct{}, urls []string,
	from time.Time) ([]*Event, error, []error) {

	since := nostr.Timestamp(from.Unix())
	filter := nostr.Filter{
		Kinds: []int{KindLightningInformation},
		Since: &since,
	}

	var fetchErrors []error
	// We have to sync our store twice: once for node announcements and
	// once for the specific kind. Node announcements have to be fetched
	// first to ensure that we have all relevant announcements in our store
	// when processing the other kinds.
	filter.Tags = nostr.TagMap{"k": {strconv.Itoa(int(KindNodeAnnouncement))}}
	err, err2 := c.syncStoreWithPool(ctx, urls, filter)
	if err != nil {
		return nil, fmt.Errorf("fetching node announcements: %v", err), nil
	}
	fetchErrors = append(fetchErrors, err2...)

	if kind != KindNodeAnnouncement {
		filter.Tags = nostr.TagMap{"k": {strconv.Itoa(int(kind))}}
		err, err2 = c.syncStoreWithPool(ctx, urls, filter)
		if err != nil {
			return nil, fmt.Errorf("fetching events of kind %d: %v", kind, err), nil
		}
		fetchErrors = append(fetchErrors, err2...)
	}
	return c.store.GetEvents(kind, pubkeys), nil, fetchErrors
}

// syncStoreWithPool fetches events from the given URLs using the provided filter
// and stores them in the client's store.
// Returns (error, []error): critical error + non-fatal warnings (fetchErrors).
// fetchErrors collect per-event issues without stopping the fetch process,
// enabling resilient operation across multiple relays and events.
func (c *Client) syncStoreWithPool(ctx context.Context, urls []string, filter nostr.Filter) (error, []error) {

	var fetchErrors []error
	appendErrs := func(err error) { fetchErrors = append(fetchErrors, err) }

	res := c.pool.FetchManyReplaceable(ctx, urls, filter)

	res.Range(func(k nostr.ReplaceableKey, ev *nostr.Event) bool {
		if err := ctx.Err(); err != nil {
			return false
		}
		// Process each event
		lev, err := NewEventFromNostrRelay(ev)
		if err != nil {
			appendErrs(fmt.Errorf("creating event from nostr relay: %v", err))
			return true
		}

		if ok, err := lev.Verify(); !ok || err != nil {
			appendErrs(fmt.Errorf("invalid event %v: %v", lev.NostrEvent.ID, err))
			return true
		}
		if err := c.store.StoreEvent(lev); err != nil {
			appendErrs(fmt.Errorf("storing event failed %v: %v", lev.NostrEvent.ID, err))
		}

		return true
	})

	if ctx.Err() != nil {
		return ctx.Err(), nil
	}
	return nil, fetchErrors
}

// GetEventEnvelopes wraps events with additional metadata (like node aliases).
// Like GetEvents, it returns ([]EventEnvelope, error, []error) where fetchErrors
// accumulate non-critical issues (envelope creation failures, alias lookup failures)
// without interrupting the overall operation.
func GetEventEnvelopes[T any](c *Client, ctx context.Context, kind Kind, pubkeys map[string]struct{},
	urls []string, from time.Time) ([]EventEnvelope[T], error, []error) {

	events, err, fetchErrors := c.GetEvents(ctx, kind, pubkeys, urls, from)
	if err != nil {
		return nil, err, nil
	}

	envelopes := make([]EventEnvelope[T], 0, len(events))
	for _, ev := range events {
		env, err := NewEventEnvelope[T](ev)
		if err != nil {
			fetchErrors = append(fetchErrors, fmt.Errorf("creating event envelope: %v", err))
			continue
		}
		alias, err := c.ln.GetAlias(ctx, env.Id.PubKey)
		if err != nil {
			// We can continue with empty alias if it fails.
			fetchErrors = append(fetchErrors, fmt.Errorf("getting alias for pubkey %s: %v", env.Id.PubKey, err))
		}
		env.Alias = alias
		envelopes = append(envelopes, *env)
	}

	return envelopes, nil, fetchErrors
}

type PublishResult struct {
	Event   *nostr.Event
	Channel chan nostr.PublishResult
}

func (c *Client) Publish(ctx context.Context, data any, kind Kind, urls []string,
	opts ...string) (PublishResult, error) {

	// Serialize to JSON for Nostr event content
	b, err := json.Marshal(data)
	if err != nil {
		return PublishResult{}, fmt.Errorf("marshaling node info: %w", err)
	}

	ev := Event{NostrEvent: &nostr.Event{
		PubKey:    c.pub,
		CreatedAt: nostr.Now(),
		Content:   string(b),
	}}

	if err := ev.Finalize(c.info.Network, c.info.PubKey, kind, opts); err != nil {
		return PublishResult{}, fmt.Errorf("finalizing event: %w", err)
	}
	if err := c.signer.SignEvent(ctx, &ev); err != nil {
		return PublishResult{}, fmt.Errorf("signing event: %w", err)
	}

	// We verify before publishing, especially to ensure the LN signature is valid.
	if ok, err := ev.Verify(); !ok || err != nil {
		return PublishResult{}, fmt.Errorf("verifying event before publish: %v", err)
	}

	res := c.pool.PublishMany(ctx, urls, *ev.NostrEvent)
	return PublishResult{Event: ev.NostrEvent, Channel: res}, nil
}

func (c *Client) Close() error {
	c.pool.Close("")
	return c.ln.Close()
}
