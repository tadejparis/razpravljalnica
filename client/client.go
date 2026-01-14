package main

import (
	"context"
	"fmt"
	"os"
	pb "razpravljalnica"
	"strconv"

	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// odpremo povezavo
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	checkError(err)
	defer conn.Close()

	client := pb.NewMessageBoardClient(conn)

	var name string
	fmt.Print("Enter a username: ")
	fmt.Scanf("%s\n", &name)
	user, err := client.CreateUser(context.Background(), &pb.CreateUserRequest{Name: name})
	//fmt.Println(user.GetId())
	checkError(err)

	var subscribedTopics []int64
	var subClient pb.MessageBoard_SubscribeTopicClient

	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name:  "all_topics",
				Usage: "list all topics",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					allTopics, err := client.ListTopics(context.Background(), &emptypb.Empty{})
					checkError(err)
					for _, topic := range allTopics.GetTopics() {
						fmt.Printf("name: %s\tid: %d\n", topic.GetName(), topic.GetId())
					}
					return nil
				},
			},
			{
				Name:  "topic",
				Usage: "options for topics",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "topic_name",
					},
				},
				Commands: []*cli.Command{
					{
						Name:  "create",
						Usage: "create a new topic",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name: "topic_name",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, err := client.CreateTopic(context.Background(), &pb.CreateTopicRequest{Name: cmd.StringArg("topic_name")})
							checkError(err)
							return nil
						},
					},
					{
						Name:  "listall",
						Usage: "list all posts in topic",
						Arguments: []cli.Argument{
							&cli.Int64Arg{
								Name: "topic_id",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							posts, err := client.GetMessages(context.Background(), &pb.GetMessagesRequest{
								TopicId: cmd.Int64Arg("topic_id"),
								Limit:   1024})
							checkError(err)
							for _, post := range posts.Messages {
								fmt.Printf("id: %d\ntext: %s\nlikes: %d\n\n", post.GetId(), post.GetText(), post.GetLikes())
							}
							return nil
						},
					},
					{
						Name:  "subscribe",
						Usage: "subscribe to a topic",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							for _, arg := range cmd.Args().Slice() {
								ia, err := strconv.ParseInt(arg, 10, 64)
								checkError(err)
								subscribedTopics = append(subscribedTopics, ia)
							}
							fmt.Printf("Subscribed topics: ")
							for _, topicId := range subscribedTopics {
								fmt.Printf("%d  ", topicId)
							}
							fmt.Println()

							snr, err := client.GetSubscriptionNode(context.Background(), &pb.SubscriptionNodeRequest{
								UserId:  user.GetId(),
								TopicId: subscribedTopics})
							checkError(err)
							subClient, err = client.SubscribeTopic(context.Background(), &pb.SubscribeTopicRequest{
								UserId:         user.GetId(),
								TopicId:        subscribedTopics,
								FromMessageId:  1,
								SubscribeToken: snr.GetSubscribeToken()})
							checkError(err)

							for {
								me, err := subClient.Recv()
								checkError(err)

								fmt.Println(me.GetMessage().GetText())
							}
							//return nil
						},
					},
				},
			},
			{
				Name:  "post",
				Usage: "options for posts",
				Commands: []*cli.Command{
					{
						Name:  "create",
						Usage: "create a new post",
						Arguments: []cli.Argument{
							&cli.Int64Arg{
								Name: "topic_id",
							},
							&cli.StringArg{
								Name: "post_content",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, err := client.PostMessage(context.Background(), &pb.PostMessageRequest{
								TopicId: cmd.Int64Arg("topic_id"),
								UserId:  user.GetId(),
								Text:    cmd.StringArg("post_content")})
							checkError(err)
							return nil
						},
					},
					{
						Name:  "delete",
						Usage: "delete an existing post you've created",
						Arguments: []cli.Argument{
							&cli.Int64Arg{
								Name: "post_id",
							},
							&cli.Int64Arg{
								Name: "topic_id",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, err = client.DeleteMessage(context.Background(), &pb.DeleteMessageRequest{
								MessageId: cmd.Int64Arg("post_id"),
								UserId:    user.GetId(),
								TopicId:   cmd.Int64Arg("topic_id")})
							checkError(err)
							return nil
						},
					},
					{
						Name:  "like",
						Usage: "like an existing post",
						Arguments: []cli.Argument{
							&cli.Int64Arg{
								Name: "post_id",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, err = client.LikeMessage(context.Background(), &pb.LikeMessageRequest{
								MessageId: cmd.Int64Arg("post_id"),
								UserId:    user.GetId()})
							checkError(err)
							return nil
						},
					},
					{
						Name:  "update",
						Usage: "update an existing post you've created",
						Arguments: []cli.Argument{
							&cli.Int64Arg{
								Name: "post_id",
							},
							&cli.StringArg{
								Name: "post_content",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, err = client.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{
								MessageId: cmd.Int64Arg("post_id"),
								UserId:    user.GetId(),
								Text:      cmd.StringArg("post_content")})
							checkError(err)
							return nil
						},
					},
				},
			},
		},
	}

	err = cmd.Run(context.Background(), os.Args)
	checkError(err)
}
