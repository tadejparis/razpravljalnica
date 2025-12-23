package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	pb "razpravljalnica"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// implementation of MessageBoard
type Server struct {
	pb.UnimplementedMessageBoardServer
	mutex sync.RWMutex

	users      map[int64]*pb.User
	nextUserID int64

	topics      map[int64]*pb.Topic
	topicChat   map[int64][]*pb.Message
	nextTopicID int64

	messages      map[int64]*pb.Message
	nextMessageID int64
}

func NewServer() *Server {
	s := &Server{
		users:      make(map[int64]*pb.User),
		nextUserID: 1,

		topics:      make(map[int64]*pb.Topic),
		topicChat:   make(map[int64][]*pb.Message),
		nextTopicID: 1,

		messages:      make(map[int64]*pb.Message),
		nextMessageID: 1,
	}

	// press enter in the server terminal to print the state
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			s.PrintState()
		}
	}()

	return s
}

// prints all the information stored in the server for testing
func (s *Server) PrintState() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	fmt.Println("users:")
	for id, user := range s.users {
		fmt.Printf("ID = %d, Name = %s\n", id, user.Name)
	}

	fmt.Println("\ntopics:")
	for id, topic := range s.topics {
		fmt.Printf("ID = %d, Name = %s\n", id, topic.Name)
	}

	fmt.Println("\nmessages:")
	for topicID, Name := range s.topics {
		fmt.Printf("Topic ID = %d, Name = %s\n", topicID, Name.Name)
		for _, msg := range s.topicChat[topicID] {
			fmt.Printf("\tUserID = %d, Text = %s\n", msg.UserId, msg.Text)
		}
	}
}

// creates a new user and assigns it an ID
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

// creates a new topic and assigns it an ID
func (s *Server) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.Topic, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	topic := &pb.Topic{
		Id:   s.nextTopicID,
		Name: req.Name,
	}
	s.topics[topic.Id] = topic
	s.nextTopicID++

	log.Printf("created topic: ID=%d, name=%s", topic.Id, topic.Name)
	return topic, nil
}

// adds a new message to a topic
func (s *Server) PostMessage(ctx context.Context, req *pb.PostMessageRequest) (*pb.Message, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.topics[req.TopicId] == nil {
		return nil, fmt.Errorf("topic with ID %d does not exist", req.TopicId)
	}

	message := &pb.Message{
		Id:        s.nextMessageID,
		TopicId:   req.TopicId,
		UserId:    req.UserId,
		Text:      req.Text,
		CreatedAt: timestamppb.Now(),
		Likes:     0,
	}

	s.topicChat[req.TopicId] = append(s.topicChat[req.TopicId], message)
	s.messages[s.nextMessageID] = message
	s.nextMessageID++

	log.Printf("posted message: topicID=%d, userID=%d, text=%s", req.TopicId, req.UserId, req.Text)
	return message, nil
}

// updates the text of an existing message
func (s *Server) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.Message, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.messages[req.MessageId] == nil {
		return nil, fmt.Errorf("message with ID %d does not exist", req.MessageId)
	}

	if s.messages[req.MessageId].UserId != req.UserId {
		return nil, fmt.Errorf("user with ID %d is not the owner of message with ID %d", req.UserId, req.MessageId)
	}

	s.messages[req.MessageId].Text = req.Text

	log.Printf("updated message: topicID=%d, userID=%d, text=%s", req.TopicId, req.UserId, req.Text)
	return s.messages[req.MessageId], nil
}

// deletes an existing message
func (s *Server) DeleteMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*emptypb.Empty, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.messages[req.MessageId] == nil {
		return nil, fmt.Errorf("message with ID %d does not exist", req.MessageId)
	}

	if s.messages[req.MessageId].UserId != req.UserId {
		return nil, fmt.Errorf("user with ID %d is not the owner of message with ID %d", req.UserId, req.MessageId)
	}

	delete(s.messages, req.MessageId)

	log.Printf("deleted message: messageID=%d", req.MessageId)
	return &emptypb.Empty{}, nil

}

// increments the like count of an existing message
func (s *Server) LikeMessage(ctx context.Context, req *pb.LikeMessageRequest) (*pb.Message, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.messages[req.MessageId] == nil {
		return nil, fmt.Errorf("message with ID %d does not exist", req.MessageId)
	}

	s.messages[req.MessageId].Likes++

	log.Printf("liked message: messageID=%d, totalLikes=%d", req.MessageId, s.messages[req.MessageId].Likes)
	return s.messages[req.MessageId], nil
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

	s := NewServer()

	// register MessageBoard service
	pb.RegisterMessageBoardServer(grpcServer, s)

	log.Println("server is running on port 50051...")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
