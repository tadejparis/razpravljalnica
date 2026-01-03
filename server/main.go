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

	subscribers    []*subscription
	likesByMessage map[int64]map[int64]struct{}
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

		subscribers:    make([]*subscription, 0),
		likesByMessage: make(map[int64]map[int64]struct{}),
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

	// reads from eventChannel and sends out to all subscribers
	go func() {
		for ev := range eventChannel {
			s.mutex.RLock()
			// snapshot subscribers to avoid holding lock while sending
			subs := make([]*subscription, len(s.subscribers))
			copy(subs, s.subscribers)
			s.mutex.RUnlock()

			// broadcast to matching subscribers
			for _, sub := range subs {
				// check if subscriber is interested in this topic
				for _, tid := range sub.topics {
					if tid == ev.topicID {
						select {
						case sub.ch <- ev.Message:
						default:
							// subscriber channel full; drop event for this client
						}
						break
					}
				}
			}
		}
	}()

	return s
}

// prints all the information stored in the server for testing
func (s *Server) PrintState() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	fmt.Println("\n========== SERVER STATE ==========")

	fmt.Printf("\nServer Address: %s\n", IP)
	fmt.Printf("Event Channel Buffer: %d/%d\n", len(eventChannel), cap(eventChannel))

	fmt.Printf("\nUsers (%d):\n", len(s.users))
	for id, user := range s.users {
		fmt.Printf("  ID=%d, Name=%s\n", id, user.Name)
	}

	fmt.Printf("\nTopics (%d):\n", len(s.topics))
	for id, topic := range s.topics {
		msgCount := len(s.topicChat[id])
		fmt.Printf("  ID=%d, Name=%s, Messages=%d\n", id, topic.Name, msgCount)
	}

	fmt.Printf("\nMessages (total %d):\n", len(s.messages))
	for topicID, topic := range s.topics {
		if len(s.topicChat[topicID]) > 0 {
			fmt.Printf("  Topic ID=%d, Name=%s:\n", topicID, topic.Name)
			for _, msg := range s.topicChat[topicID] {
				fmt.Printf("    MsgID=%d, UserID=%d, Text=%q, Likes=%d\n",
					msg.Id, msg.UserId, msg.Text, msg.Likes)
			}
		}
	}

	fmt.Printf("\nActive Subscribers (%d):\n", len(s.subscribers))
	for _, sub := range s.subscribers {
		fmt.Printf("  UserID=%d, Topics=%v, ChannelBuffer=%d/%d\n",
			sub.userID, sub.topics, len(sub.ch), cap(sub.ch))
	}

	fmt.Printf("\nPending Subscription Requests (%d):\n", len(subscriptionRequests))
	for i, token := range subscriptionRequests {
		fmt.Printf("  %d: %s\n", i+1, token)
	}

	fmt.Println("\n==================================")
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

	s.likesByMessage[message.Id] = make(map[int64]struct{})
	s.nextMessageID++

	eventChannel <- event{Message: &pb.MessageEvent{SequenceNumber: 0, Op: pb.OpType_OP_POST, Message: message, EventAt: timestamppb.Now()}, topicID: message.TopicId}

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

	eventChannel <- event{Message: &pb.MessageEvent{SequenceNumber: 0, Op: pb.OpType_OP_UPDATE, Message: s.messages[req.MessageId], EventAt: timestamppb.Now()}, topicID: s.messages[req.MessageId].TopicId}

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

	for i, msg := range s.topicChat[req.TopicId] {
		if msg.Id == req.MessageId {
			s.topicChat[req.TopicId] = append(s.topicChat[req.TopicId][:i], s.topicChat[req.TopicId][i+1:]...)
			break
		}
	}

	delete(s.messages, req.MessageId)
	delete(s.likesByMessage, req.MessageId)

	eventChannel <- event{Message: &pb.MessageEvent{SequenceNumber: 0, Op: pb.OpType_OP_DELETE, Message: nil, EventAt: timestamppb.Now()}, topicID: req.TopicId}

	log.Printf("deleted message: messageID=%d", req.MessageId)
	return &emptypb.Empty{}, nil

}

// increments the like count of an existing message
func (s *Server) LikeMessage(ctx context.Context, req *pb.LikeMessageRequest) (*pb.Message, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	msg := s.messages[req.MessageId]
	if msg == nil {
		return nil, fmt.Errorf("message with ID %d does not exist", req.MessageId)
	}

	if _, liked := s.likesByMessage[req.MessageId][req.UserId]; liked {
		return msg, nil
	}

	s.likesByMessage[req.MessageId][req.UserId] = struct{}{}
	msg.Likes = msg.Likes + 1

	eventChannel <- event{Message: &pb.MessageEvent{SequenceNumber: 0, Op: pb.OpType_OP_LIKE, Message: msg, EventAt: timestamppb.Now()}, topicID: msg.TopicId}

	log.Printf("liked message: messageID=%d, totalLikes=%d", req.MessageId, msg.Likes)
	return msg, nil
}

// stores tokens which nodes will check to validate SubscribeTopicRequests
var subscriptionRequests = make([]string, 0)

// assigns a node to a user for subscriptions
func (s *Server) GetSubscriptionNode(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.SubscriptionNodeResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	token := fmt.Sprintf("%d", req.UserId)
	for _, topic := range req.TopicId {
		token += fmt.Sprintf("-%d", topic) // since we have only 1 node so far, we dont need to append the node ID
	}
	subscriptionRequests = append(subscriptionRequests, token)

	return &pb.SubscriptionNodeResponse{SubscribeToken: token, Node: &pb.NodeInfo{NodeId: "1", Address: IP}}, nil
}

// lists all topics
func (s *Server) ListTopics(ctx context.Context, req *emptypb.Empty) (*pb.ListTopicsResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	topics := make([]*pb.Topic, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}

	return &pb.ListTopicsResponse{Topics: topics}, nil
}

// lists messages in a topic
func (s *Server) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	messages := make([]*pb.Message, 0)
	var i int64
	var count int32 = 0
	for i = req.FromMessageId; i < int64(len(s.topicChat[req.TopicId])) && count < req.Limit; i++ {
		messages = append(messages, s.topicChat[req.TopicId][i])
		count++
	}

	return &pb.GetMessagesResponse{Messages: messages}, nil
}

type subscription struct {
	userID int64
	topics []int64
	stream pb.MessageBoard_SubscribeTopicServer
	ch     chan *pb.MessageEvent
}

type event struct {
	Message *pb.MessageEvent
	topicID int64
}

// internal event channel
var eventChannel = make(chan event, 100)

func (s *Server) SubscribeTopic(req *pb.SubscribeTopicRequest, stream pb.MessageBoard_SubscribeTopicServer) error {
	// validate token
	s.mutex.Lock()
	expectedToken := fmt.Sprintf("%d", req.UserId)
	for _, topic := range req.TopicId {
		expectedToken += fmt.Sprintf("-%d", topic)
	}

	var valid bool = false
	for i, token := range subscriptionRequests {
		if token == expectedToken {
			valid = true
			subscriptionRequests = append(subscriptionRequests[:i], subscriptionRequests[i+1:]...)
			break
		}
	}
	s.mutex.Unlock()

	if !valid {
		return fmt.Errorf("invalid subscription token for user ID %d", req.UserId)
	}

	// create per-subscriber channel and subscription
	sub := &subscription{
		userID: req.UserId,
		topics: req.TopicId,
		stream: stream,
		ch:     make(chan *pb.MessageEvent, 32),
	}

	// register subscriber
	s.mutex.Lock()
	s.subscribers = append(s.subscribers, sub)
	s.mutex.Unlock()

	// ensure cleanup on exit
	defer func() {
		s.mutex.Lock()
		for i, subscriber := range s.subscribers {
			if subscriber == sub {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				break
			}
		}
		s.mutex.Unlock()
		close(sub.ch)
		log.Printf("user ID %d unsubscribed", req.UserId)
	}()

	log.Printf("user ID %d subscribed to topics %v from message ID %d", req.UserId, req.TopicId, req.FromMessageId)

	// replay historical messages for subscribed topics (from from_message_id onwards)
	s.mutex.RLock()
	sequenceNumber := int64(0)
	for _, topicID := range req.TopicId {
		for _, msg := range s.topicChat[topicID] {
			if msg.Id >= req.FromMessageId {
				sequenceNumber++
				histEvent := &pb.MessageEvent{
					SequenceNumber: sequenceNumber,
					Op:             pb.OpType_OP_POST,
					Message:        msg,
					EventAt:        msg.CreatedAt,
				}
				if err := stream.Send(histEvent); err != nil {
					s.mutex.RUnlock()
					log.Printf("error sending history to user ID %d: %v", req.UserId, err)
					return err
				}
			}
		}
	}
	s.mutex.RUnlock()

	// stream live events to this subscriber (single-sender per stream)
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			// client disconnected
			return ctx.Err()
		case msg, ok := <-sub.ch:
			if !ok {
				return nil
			}
			// assign sequence number per subscriber
			sequenceNumber++
			msg.SequenceNumber = sequenceNumber

			if err := stream.Send(msg); err != nil {
				log.Printf("error sending to user ID %d: %v", req.UserId, err)
				return err
			}
		}
	}
}

var IP string

func outboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Get the actual IP clients can use
	ip, err := outboundIP()
	if err != nil {
		log.Fatalf("failed to get outbound IP: %v", err)
	}

	// Extract port from listener
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	IP = net.JoinHostPort(ip.String(), port)

	// create a new grpc server
	grpcServer := grpc.NewServer()

	s := NewServer()

	// register MessageBoard service
	pb.RegisterMessageBoardServer(grpcServer, s)

	log.Printf("server is running on %s...", IP)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
