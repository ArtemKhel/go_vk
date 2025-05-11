package service

import (
	"context"
	"errors"
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"go_vk/pkg/subpub"
	pb "go_vk/proto/subpub"
)

// PubSubService implements the gRPC PubSub service.
type PubSubService struct {
	pb.UnimplementedPubSubServer
	sp     subpub.SubPub
	logger *zap.SugaredLogger
}

// NewPubSubService creates a new PubSubService.
func NewPubSubService(sp subpub.SubPub, logger *zap.SugaredLogger) *PubSubService {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &PubSubService{
		sp:     sp,
		logger: logger,
	}
}

// RegisterService registers this service with a gRPC server.
func (s *PubSubService) RegisterService(srv *grpc.Server) {
	pb.RegisterPubSubServer(srv, s)
}

func (s *PubSubService) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	key := req.GetKey()
	data := req.GetData()

	s.logger.Debugw("Publish request received", "key", key, "data_len", len(data))

	if key == "" {
		s.logger.Warnw("Publish failed: empty key", "key", key)
		return nil, status.Errorf(codes.InvalidArgument, "key cannot be empty")
	}

	err := s.sp.Publish(key, data)
	if err != nil {
		s.logger.Errorw("Failed to publish to SubPub", "key", key, "error", err)
		if errors.Is(err, subpub.ErrClosed) {
			return nil, status.Errorf(codes.Unavailable, "service is shutting down or closed")
		}
		return nil, status.Errorf(codes.Internal, "failed to publish event: %v", err)
	}

	s.logger.Debugw("Publish successful", "key", key)
	return &emptypb.Empty{}, nil
}

func (s *PubSubService) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	key := req.GetKey()
	s.logger.Debugw("Subscribe request received", "key", key)

	if key == "" {
		s.logger.Warnw("Subscribe failed: empty key", "key", key)
		return status.Errorf(codes.InvalidArgument, "key cannot be empty")
	}

	handlerDone := make(chan struct{})

	handler := func(msg interface{}) {
		msgStr, ok := msg.(string)
		if !ok {
			s.logger.Errorw("Received non-string message from subpub, skipping", "key", key, "type", "%T", msg)
			return
		}
		event := &pb.Event{Data: msgStr}

		select {
		case <-handlerDone:
			return
		default:
		}

		if err := stream.Send(event); err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled || status.Code(err) == codes.Unavailable {
				s.logger.Infow("Stream closed by client or transport error while sending event", "key", key, "error_code", status.Code(err))
			} else {
				s.logger.Errorw("Failed to send event to stream", "key", key, "error", err)
			}
			// Signal handler to stop attempting sends if `stream.Send` fails.
			// The unsubscription will be handled by stream.Context().Done() and its defer.
			select {
			case <-handlerDone:
			default:
				close(handlerDone)
			}
			return
		}
	}

	subscription, err := s.sp.Subscribe(key, handler)
	if err != nil {
		s.logger.Errorw("Failed to subscribe to SubPub", "key", key, "error", err)
		if errors.Is(err, subpub.ErrClosed) {
			return status.Errorf(codes.Unavailable, "service is shutting down or closed")
		}
		return status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}
	s.logger.Infow("Successfully subscribed to SubPub", "key", key)

	defer func() {
		s.logger.Infow("Unsubscribing from SubPub in defer", "key", key)
		subscription.Unsubscribe()
	}()

	<-stream.Context().Done() // Block until client disconnects or server shuts down stream

	select {
	case <-handlerDone:
	default:
		close(handlerDone)
	}

	err = stream.Context().Err()
	s.logger.Infow("Subscribe stream ended", "key", key, "reason", err)

	if errors.Is(err, context.Canceled) {
		return status.FromContextError(err).Err()
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return status.Errorf(codes.Unknown, "stream ended with unexpected error: %v", err)
	}

	return nil
}
