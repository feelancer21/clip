package clip

import (
	"context"
	"fmt"
	"os"
)

type LnInteractive struct {
	pubKey  string
	network string
}

func NewLnInteractive(network string, pubKey string) *LnInteractive {
	return &LnInteractive{
		pubKey:  pubKey,
		network: network,
	}
}

func (l *LnInteractive) Close() error {
	return nil
}

func (l *LnInteractive) GetAlias(ctx context.Context, pubkey string) (string, error) {
	return "", nil
}

func (l *LnInteractive) GetNodeInfo(_ context.Context) (NodeInfoResponse, error) {
	return NodeInfoResponse{
		PubKey:  l.pubKey,
		Network: l.network,
	}, nil
}

func (l *LnInteractive) SignMessage(_ context.Context, msg []byte) (string, error) {
	// Printing the message to be signed to stdout and reading the signature from stdin.
	stringMsg := string(msg)
	fmt.Fprintf(os.Stderr, "\nPlease sign the following message with your Lightning node:\n%s\n\nEnter the signature here: ", stringMsg)

	var sig string
	_, err := fmt.Scanln(&sig)
	if err != nil {
		return "", fmt.Errorf("reading signature from stdin: %w", err)
	}
	fmt.Printf("\n")
	return sig, nil
}

var _ LightningNode = (*LnInteractive)(nil)
