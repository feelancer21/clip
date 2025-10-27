package clip

import (
	"fmt"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

type announcementState struct {
	createdAt nostr.Timestamp
	pub       string
}

type nodeState struct {
	mu               sync.RWMutex
	lastAnnouncement announcementState

	// map with 'd' tag as key
	events map[string]*Event
}

func newNodeState() *nodeState {
	return &nodeState{
		events: make(map[string]*Event),
	}
}

type MapStore struct {
	mu sync.RWMutex
	// map with node pubkey as key
	records map[string]*nodeState
}

func NewMapStore() *MapStore {
	return &MapStore{
		records: make(map[string]*nodeState),
	}
}

func (s *MapStore) StoreEvent(ev *Event) error {
	id, err := ev.GetIdentifier()
	if err != nil {
		return err
	}

	ns := s.getNodeState(id.PubKey)

	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ev.kind == KindNodeAnnouncement {
		return s.registerAnnouncement(ns, ev, id)
	}

	return s.storeRegularEvent(ns, ev, id)
}

func (s *MapStore) registerAnnouncement(ns *nodeState, ev *Event, id *Identifier) error {
	// Skip if existing announcement is newer or same
	if ns.lastAnnouncement.createdAt >= ev.NostrEvent.CreatedAt {
		return fmt.Errorf("existing announcement is newer or same: %d >= %d",
			ns.lastAnnouncement.createdAt, ev.NostrEvent.CreatedAt)
	}

	// Purge old events if pubkey changed (potential nsec compromise)
	if ns.lastAnnouncement.pub != ev.NostrEvent.PubKey {
		ns.events = make(map[string]*Event)
	}

	// Store the new announcement
	ns.events[id.TagD] = ev
	ns.lastAnnouncement = announcementState{
		createdAt: ev.NostrEvent.CreatedAt,
		pub:       ev.NostrEvent.PubKey,
	}

	return nil
}

func (s *MapStore) storeRegularEvent(ns *nodeState, ev *Event, id *Identifier) error {
	// Only accept events matching the last announcement pubkey
	if ns.lastAnnouncement.pub != ev.NostrEvent.PubKey {
		return fmt.Errorf("event pubkey %s does not match last announcement pubkey %s",
			ev.NostrEvent.PubKey, ns.lastAnnouncement.pub)
	}

	// Skip if existing record is newer or same
	if lastRecord, exists := ns.events[id.TagD]; exists {
		if lastRecord.NostrEvent.CreatedAt >= ev.NostrEvent.CreatedAt {
			return fmt.Errorf("existing record is newer or same: %d >= %d",
				lastRecord.NostrEvent.CreatedAt, ev.NostrEvent.CreatedAt)
		}
	}

	ns.events[id.TagD] = ev
	return nil
}

func (s *MapStore) getNodeState(pubkey string) *nodeState {
	// Fast path: read lock to check if exists
	s.mu.RLock()
	ns, exists := s.records[pubkey]
	s.mu.RUnlock()

	if exists {
		return ns
	}

	// Slow path: write lock to create if still missing
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check: might have been created by another goroutine
	ns, exists = s.records[pubkey]
	if exists {
		return ns
	}

	// Create new node state
	ns = newNodeState()
	s.records[pubkey] = ns
	return ns
}

func (s *MapStore) GetEvents(kind Kind, pubKeys map[string]struct{}) []*Event {
	events := []*Event{}

	pubFilter := newInFilter[string](pubKeys)

	// Snapshot node pointers
	s.mu.RLock()
	nodes := make([]*nodeState, 0, len(s.records))
	for pubKey, ns := range s.records {
		if !pubFilter(pubKey) {
			continue
		}
		nodes = append(nodes, ns)
	}
	s.mu.RUnlock()

	for _, ns := range nodes {
		ns.mu.RLock()
		for _, ev := range ns.events {
			if ev.kind != kind {
				continue
			}
			events = append(events, ev)
		}
		ns.mu.RUnlock()
	}
	return events
}

// newInFilter returns a filter function that checks if an item is in the provided set.
// If the set is empty, all items are considered to be in the set.
func newInFilter[T comparable](set map[T]struct{}) func(T) bool {
	return func(item T) bool {
		if len(set) == 0 {
			return true
		}
		_, exists := set[item]
		return exists
	}
}
