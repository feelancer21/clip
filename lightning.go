package clip

import "context"

type LnSigner interface {
	// SignMessage signs a message with the node's private key.
	SignMessage(ctx context.Context, msg []byte) (string, error)
}

type LightningNode interface {
	// Close closes the connection to the Lightning node.
	Close() error

	// GetAlias returns the alias of a node identified by its pubkey.
	GetAlias(ctx context.Context, pubkey string) (string, error)

	// GetNodeInfo returns the basic info of the connected node.
	GetNodeInfo(ctx context.Context) (NodeInfoResponse, error)

	// GetNodeCapacity returns the total capacity of a node identified by its pubkey.
	// Not needed at the moment.
	//GetNodeCapacity(ctx context.Context, pubkey string) (int64, error)

	LnSigner
}

type NodeInfoResponse struct {
	PubKey  string `json:"pubkey"`
	Network string `json:"network"`
}

func (n *NodeInfoResponse) checkNetwork() bool {
	return IsValidNetwork(n.Network)
}

func IsValidNetwork(network string) bool {
	switch network {
	case "mainnet", "testnet", "testnet4", "signet", "simnet", "regtest":
		return true
	default:
		return false
	}
}
