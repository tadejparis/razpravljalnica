package main

import (
	"context"
	"fmt"

	"github.com/rivo/tview"
	pb "github.com/tadejparis/razpravljalnica/specifikacija"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func generateTopicsPage(client *pb.MessageBoardClient, pages *tview.Pages, user_id int64) {

	topicsList := tview.NewList()
	// get topics from server
	allTopics, err := (*client).ListTopics(context.Background(), &emptypb.Empty{})
	checkError(err)
	// populate topicsList with topics from server
	for _, topic := range allTopics.Topics {
		topicsList.AddItem(topic.Name, "", 0, func() {
			generateTopicPostsPage(client, pages, user_id, topic.Id)
			pages.SwitchToPage("topicPostsPage")
		})
	}

	topicsNavigationForm := tview.NewForm().
		AddButton("Back", func() {
			pages.SwitchToPage("mainMenuPage")
		})

	topicsFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topicsNavigationForm, 0, 1, false).
		AddItem(topicsList, 0, 7, false)

	topicsList.SetBorder(true).SetTitle("All topics").SetTitleAlign(tview.AlignLeft)
	(*pages).RemovePage("topicsPage") // odstrani star page najprej
	(*pages).AddPage("topicsPage", topicsFlex, true, true)
}

func generateTopicPostsPage(client *pb.MessageBoardClient, pages *tview.Pages, user_id int64, topic_id int64) {

	topicPostsList := tview.NewList()
	posts, err := (*client).GetMessages(context.Background(), &pb.GetMessagesRequest{TopicId: topic_id})
	checkError(err)
	for _, post := range posts.Messages {
		topicPostsList.AddItem(post.GetText(), fmt.Sprintf("Likes: %d", post.GetLikes()), 0, func() {
			generatePostPage(client, pages, user_id, post)
		})
	}

	topicPostsNavigationForm := tview.NewForm().
		AddButton("Back", func() {
			pages.SwitchToPage("topicsPage")
		}).
		AddButton("Create post", func() {
			generateCreatePostPage(client, pages, user_id, topic_id)
			pages.SwitchToPage("createPostPage")
		})

	topicPostsFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topicPostsNavigationForm, 0, 1, false).
		AddItem(topicPostsList, 0, 7, false)

	(*pages).RemovePage("topicPostsPage") // odstrani star page najprej
	(*pages).AddPage("topicPostsPage", topicPostsFlex, true, true)
}

func generateEditPostPage(client *pb.MessageBoardClient, pages *tview.Pages, user_id int64, post *pb.Message) {
	editPostForm := tview.NewForm()
	editPostForm.
		AddTextArea("", post.GetText(), 256, 0, 0, nil).
		AddButton("Save", func() {
			textArea := editPostForm.GetFormItem(0).(*tview.TextArea)
			text := textArea.GetText()
			_, err := (*client).UpdateMessage(context.Background(), &pb.UpdateMessageRequest{MessageId: post.Id, UserId: user_id, Text: text})
			checkError(err)
			pages.SwitchToPage("topicsPage")
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("postPage")
		})

	(*pages).RemovePage("generateEditPostPage") // odstrani star page najprej
	(*pages).AddPage("generateEditPostPage", editPostForm, true, true)
}

func generatePostPage(client *pb.MessageBoardClient, pages *tview.Pages, user_id int64, post *pb.Message) {

	owner := (user_id == post.UserId)
	postNavigationForm := tview.NewForm().
		AddButton("Back", func() {
			pages.SwitchToPage("topicsPage")
		}).
		AddButton("Like", func() {
			_, err := (*client).LikeMessage(context.Background(), &pb.LikeMessageRequest{MessageId: post.Id, UserId: user_id})
			checkError(err)
		})

	if owner {
		postNavigationForm.
			AddButton("Edit", func() {
				generateEditPostPage(client, pages, user_id, post)
			}).
			AddButton("Delete", func() {
				_, err := (*client).DeleteMessage(context.Background(), &pb.DeleteMessageRequest{MessageId: post.Id, UserId: user_id})
				checkError(err)
			})
	}

	postText := tview.NewTextView()
	fmt.Fprintf(postText, "%s", post.GetText())

	postFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(postNavigationForm, 0, 1, false).
		AddItem(postText, 0, 7, false)

	(*pages).RemovePage("postPage") // odstrani star page najprej
	(*pages).AddPage("postPage", postFlex, true, true)
}

func generateCreatePostPage(client *pb.MessageBoardClient, pages *tview.Pages, user_id int64, topic_id int64) {
	// forma: ustvari nov post v topicu
	createPostForm := tview.NewForm()
	createPostForm.
		AddTextArea("Post content", "", 256, 0, 0, nil).
		AddButton("Enter", func() {
			textArea := createPostForm.GetFormItem(0).(*tview.TextArea)
			text := textArea.GetText()
			_, err := (*client).PostMessage(context.Background(), &pb.PostMessageRequest{TopicId: topic_id, UserId: user_id, Text: text}) // TODO: proper field values
			checkError(err)
			generateTopicPostsPage(client, pages, user_id, topic_id)
			pages.SwitchToPage("topicPostsPage")
		}).
		AddButton("Cancel", func() {
		})

	createPostForm.SetBorder(true).SetTitle("New post").SetTitleAlign(tview.AlignLeft)

	(*pages).RemovePage("createPostPage") // odstrani star page najprej
	(*pages).AddPage("createPostPage", createPostForm, true, true)
}

func generateCreateTopicPage(client *pb.MessageBoardClient, pages *tview.Pages) {
	// forma: ustvari nov topic
	createTopicForm := tview.NewForm()
	createTopicForm.
		AddInputField("Topic name", "", 20, nil, nil).
		AddButton("Enter", func() {
			pages.SwitchToPage("mainMenuPage")
			inputField := createTopicForm.GetFormItem(0).(*tview.InputField)
			name := inputField.GetText()
			_, err := (*client).CreateTopic(context.Background(), &pb.CreateTopicRequest{Name: name})
			checkError(err)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("mainMenuPage")
		})

	createTopicForm.SetBorder(true).SetTitle("New topic").SetTitleAlign(tview.AlignLeft)

	(*pages).RemovePage("createTopicPage") // odstrani star page najprej
	(*pages).AddPage("createTopicPage", createTopicForm, true, true)

}

func main() {

	app := tview.NewApplication()
	pages := tview.NewPages()

	// TODO: Dial options?
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	checkError(err)
	defer conn.Close()

	client := pb.NewMessageBoardClient(conn)

	var name string
	fmt.Print("Enter a username: ")
	fmt.Scanf("%s\n", &name)
	user, err := client.CreateUser(context.Background(), &pb.CreateUserRequest{Name: name})
	checkError(err)

	// deklariramo main menu
	mainMenu := tview.NewList()

	// glavni menu razpravljalnice
	mainMenu.
		AddItem("All topics", "", 0, func() {
			generateTopicsPage(&client, pages, (*user).Id)
			pages.SwitchToPage("topicsPage")
		}).
		AddItem("Create Topic", "", 0, func() {
			generateCreateTopicPage(&client, pages)
			pages.SwitchToPage("createTopicPage")
		}).
		AddItem("Quit", "Press to exit", 'q', func() {
			app.Stop()
		})
	mainMenu.SetBorder(true).SetTitle("Razpravljalnica").SetTitleAlign(tview.AlignLeft)
	pages.AddPage("mainMenuPage", mainMenu, true, true)

	app.SetRoot(pages, true)
	app.EnableMouse(true)

	err = app.Run()
	checkError(err)

}
