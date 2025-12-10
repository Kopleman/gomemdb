package network

import (
	"context"
	"fmt"
	"net"

	"github.com/Kopleman/gomemdb/api/proto"
	"github.com/Kopleman/gomemdb/internal/database"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// GRPCServerOption is a function type for configuring gRPC server.
type GRPCServerOption func(*GRPCServer, *[]grpc.ServerOption)

// GRPCServer wraps gRPC server implementation.
type GRPCServer struct {
	proto.UnimplementedDatabaseServiceServer
	server   *grpc.Server
	listener net.Listener
	db       *database.DataBase
	logger   *zap.Logger
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(
	address string,
	db *database.DataBase,
	logger *zap.Logger,
	options ...GRPCServerOption,
) (*GRPCServer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is invalid")
	}

	if db == nil {
		return nil, fmt.Errorf("database is invalid")
	}

	grpcServer := &GRPCServer{
		db:     db,
		logger: logger,
	}

	// Apply options
	var grpcOptions []grpc.ServerOption
	for _, option := range options {
		option(grpcServer, &grpcOptions)
	}

	grpcServer.server = grpc.NewServer(grpcOptions...)
	proto.RegisterDatabaseServiceServer(grpcServer.server, grpcServer)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer.listener = listener

	return grpcServer, nil
}

// ExecuteCommand implements DatabaseServiceServer interface.
func (s *GRPCServer) ExecuteCommand(ctx context.Context, req *proto.CommandRequest) (*proto.CommandResponse, error) {
	response := &proto.CommandResponse{}

	result, handleErr := s.db.HandleQuery(ctx, req.GetQuery())
	if handleErr != nil {
		s.logger.Error("failed to execute command", zap.Error(handleErr))
		response.Success = false
		response.Error = handleErr.Error()
	} else {
		response.Success = true
		response.Result = result
	}

	return response, nil
}

// Stop stops the gRPC server gracefully.
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// HandleQueries is a compatibility method that starts the server.
func (s *GRPCServer) HandleQueries(ctx context.Context) {
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			s.logger.Error("failed to serve gRPC", zap.Error(err))
		}
	}()

	s.logger.Info("gRPC server started", zap.String("address", s.listener.Addr().String()))

	// Wait for context cancellation
	<-ctx.Done()
	s.Stop()
}
