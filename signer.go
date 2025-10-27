package clip

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

// Abstract interface to sign an event with nostr and ln identity key
type EventSigner interface {
	SignEvent(ctx context.Context, ev *Event) error
}

type CombinedSigner struct {
	// Interface to sign with nostr pk
	NostrSigner nostr.Signer
	// Interface to sign with ln identity key
	LnSigner LnSigner
}

func (s *CombinedSigner) SignEvent(ctx context.Context, ev *Event) error {
	if !ev.IsFinalized() {
		return fmt.Errorf("event not finalized")
	}

	if ev.RequiresLnSignature() {
		if err := s.signWithLn(ctx, ev); err != nil {
			return fmt.Errorf("signing with ln: %w", err)
		}
	}

	if err := s.NostrSigner.SignEvent(ctx, ev.NostrEvent); err != nil {
		return fmt.Errorf("signing with nostr: %w", err)
	}
	return nil
}

func (s *CombinedSigner) signWithLn(ctx context.Context, ev *Event) error {
	if ev.NostrEvent.Tags.Find("sig") != nil {
		return fmt.Errorf("event already has a 'sig' tag")
	}

	// Signing the event with the Lightning node.
	sig, err := s.LnSigner.SignMessage(ctx, ev.Hash())
	if err != nil {
		return err
	}
	ev.NostrEvent.Tags = append(ev.NostrEvent.Tags, nostr.Tag{"sig", sig})
	return nil
}

var _ EventSigner = (*CombinedSigner)(nil)
