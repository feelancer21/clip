package clip

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// MacaroonCredential implements the credentials.PerRPCCredentials interface
type MacaroonCredential struct {
	MacaroonHex string
}

func (m *MacaroonCredential) GetRequestMetadata(ctx context.Context,
	uri ...string) (map[string]string, error) {

	return map[string]string{
		"macaroon": m.MacaroonHex,
	}, nil
}

func (m *MacaroonCredential) RequireTransportSecurity() bool {
	return true
}

type LND struct {
	conn   *grpc.ClientConn
	client lnrpc.LightningClient
}

func NewLND(tlsCertPath string, macaroonPath string, host string,
	port int) (*LND, error) {

	// Read TLS certificate
	tlsCert, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("reading TLS cert: %w", err)
	}

	// Read macaroon
	macBytes, err := os.ReadFile(macaroonPath)
	if err != nil {
		return nil, fmt.Errorf("reading macaroon: %w", err)
	}

	// Create gRPC connection // Dial is deprecated!
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(tlsCert),
		grpc.WithPerRPCCredentials(&MacaroonCredential{
			MacaroonHex: hex.EncodeToString(macBytes),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC channel to LND: %w", err)
	}

	return &LND{
		conn:   conn,
		client: lnrpc.NewLightningClient(conn),
	}, nil
}

func (l *LND) Close() error {
	return l.conn.Close()
}

func (l *LND) GetAlias(ctx context.Context, pubkey string) (string, error) {
	info, err := l.client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		return "", fmt.Errorf("lnd getting node info: %w", err)
	}

	return info.Node.Alias, nil
}

func (l *LND) GetNodeInfo(ctx context.Context) (NodeInfoResponse, error) {
	info, err := l.client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return NodeInfoResponse{}, fmt.Errorf("lnd getting node info: %w", err)
	}

	if len(info.Chains) == 0 {
		return NodeInfoResponse{}, fmt.Errorf("lnd no chain info available")
	}

	res := NodeInfoResponse{
		PubKey:  info.IdentityPubkey,
		Network: info.Chains[0].Network,
	}
	return res, nil
}

func (l *LND) GetNodeCapacity(ctx context.Context, pubkey string) (int64, error) {
	info, err := l.client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		return 0, fmt.Errorf("lnd getting node info: %w", err)
	}

	return info.GetTotalCapacity(), nil
}

func (l *LND) SignMessage(ctx context.Context, msg []byte) (string, error) {
	resp, err := l.client.SignMessage(ctx, &lnrpc.SignMessageRequest{
		Msg: msg,
	})
	if err != nil {
		return "", fmt.Errorf("lnd signing message: %w", err)
	}

	return resp.GetSignature(), nil
}

// compile-time check to ensure LND implements the LightningNode interface
var _ LightningNode = (*LND)(nil)
