package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "razpravljalnica"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// implementation of MessageBoard
type Server struct {
	pb.UnimplementedMessageBoardServer
	mutex      sync.RWMutex
	users      map[int64]*pb.User
	nextUserID int64
}

func NewServer() *Server {
	return &Server{
		users:      make(map[int64]*pb.User),
		nextUserID: 1,
	}
}

// createUser creates a new user and assigns it an ID
func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	user := &pb.User{
		Id:   s.nextUserID,
		Name: req.Name,
	}
	s.users[s.nextUserID] = user
	s.nextUserID++

	log.Printf("created user: ID=%d, name=%s", user.Id, user.Name)
	return user, nil
}

func (s *Server) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) PostMessage(ctx context.Context, req *pb.PostMessageRequest) (*pb.Message, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.Message, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*emptypb.Empty, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) LikeMessage(ctx context.Context, req *pb.LikeMessageRequest) (*pb.Message, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetSubscriptionNode(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.SubscriptionNodeResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) ListTopics(ctx context.Context, req *emptypb.Empty) (*pb.ListTopicsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) SubscribeTopic(req *pb.SubscribeTopicRequest, stream pb.MessageBoard_SubscribeTopicServer) error {
	return fmt.Errorf("not implemented")
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// create a new grpc server
	grpcServer := grpc.NewServer()

	// register MessageBoard service
	pb.RegisterMessageBoardServer(grpcServer, NewServer())

	log.Println("server is running on port 50051...")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
