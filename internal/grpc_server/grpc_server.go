package grpc_server

import (
	"errors"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go_vk/internal/config"
	"go_vk/internal/service"
	"go_vk/pkg/subpub"
)

// GRPCServer is the gRPC server structure.
type GRPCServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *zap.SugaredLogger
	cfg        config.GRPCServerConfig
	sp         subpub.SubPub
}

// New creates a new gRPC GRPCServer instance.
func New(cfg config.GRPCServerConfig, sp subpub.SubPub, logger *zap.SugaredLogger) (*GRPCServer, error) {
	logger.Info("Initializing grpc server with config", cfg)
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.MaxMessageSize),
		grpc.MaxSendMsgSize(cfg.MaxMessageSize),
	}
	grpcSrv := grpc.NewServer(opts...)

	pubSubSvc := service.NewPubSubService(sp, logger.Named("pubsub_service"))
	pubSubSvc.RegisterService(grpcSrv)

	reflection.Register(grpcSrv)
	logger.Info("gRPC reflection enabled")

	listenAddr := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Errorw("Failed to listen on gRPC port", "address", listenAddr, "error", err)
		return nil, fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}
	logger.Infof("gRPC server configured to listen on %s", lis.Addr().String())

	return &GRPCServer{
		grpcServer: grpcSrv,
		listener:   lis,
		logger:     logger,
		cfg:        cfg,
		sp:         sp,
	}, nil
}

// Start starts the gRPC server. This is a blocking call.
func (s *GRPCServer) Start() error {
	s.logger.Infof("gRPC server starting on %s", s.listener.Addr().String())
	if err := s.grpcServer.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		s.logger.Errorw("gRPC server failed to serve", "error", err)
		return fmt.Errorf("gRPC server Serve error: %w", err)
	}
	s.logger.Info("gRPC server stopped serving.")
	return nil
}

// Stop gracefully stops the gRPC server.
func (s *GRPCServer) Stop(timeout time.Duration) {
	s.logger.Info("Attempting to gracefully stop gRPC server...")

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.logger.Info("gRPC server stopped gracefully.")
	case <-time.After(timeout):
		s.logger.Warn("gRPC server GracefulStop timed out. Forcing stop.")
		s.grpcServer.Stop()
	}
}
