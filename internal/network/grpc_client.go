package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kopleman/gomemdb/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient wraps gRPC client implementation.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client proto.DatabaseServiceClient
}

// GRPCClientOption is a function type for configuring gRPC client.
type GRPCClientOption func(*[]grpc.DialOption)

const (
	defaultRPCRequestTimeout = 30 * time.Second
)

// NewGRPCClient creates a new gRPC client.
func NewGRPCClient(address string, options ...GRPCClientOption) (*GRPCClient, error) {
	// Default options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Apply custom options
	for _, option := range options {
		option(&opts)
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	client := proto.NewDatabaseServiceClient(conn)

	return &GRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Send sends a command request and returns the response.
func (c *GRPCClient) Send(request []byte) ([]byte, error) {
	query := strings.TrimSpace(string(request))

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCRequestTimeout)
	defer cancel()

	req := &proto.CommandRequest{
		Query: query,
	}

	resp, err := c.client.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	if resp.GetSuccess() {
		if resp.GetResult() != "" {
			return []byte(fmt.Sprintf("[ok] %s", resp.GetResult())), nil
		}
		return []byte("[ok] "), nil
	}

	return []byte(fmt.Sprintf("[error] %s", resp.GetError())), nil
}

func (c *GRPCClient) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
