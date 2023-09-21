package dfgcp


import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"golang.org/x/net/context"
)
type DialogflowRequest struct {
	DetectIntentResponseID string            `json:"detectIntentResponseId"`
	PageInfo               map[string]string `json:"pageInfo"`
	SessionInfo            SessionInfo       `json:"sessionInfo"`
	FulfillmentInfo        FulfillmentInfo   `json:"fulfillmentInfo"`
	Text                   string            `json:"text"`
	LanguageCode           string            `json:"languageCode"`
}

type SessionInfo struct {
	Session string `json:"session"`
}

type FulfillmentInfo struct {
	Tag string `json:"tag"`
}

type DialogflowResponse struct {
	FulfillmentResponse FulfillmentResponse `json:"fulfillmentResponse"`
	SessionInfo         SessionInfo         `json:"sessionInfo"`
}

type Text struct {
	Text []string `json:"text"`
}

type ResponseMessage struct {
	Text Text `json:"text"`
}

type FulfillmentResponse struct {
	Messages []ResponseMessage `json:"messages"`
}

type Conversation struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Turn    int    `json:"turn"`
}

var app *firebase.App
var ctx context.Context

func init() {
	functions.HTTP("wtfunc", wtfunc)
	ctx = context.Background()
	conf := &firebase.Config{
		DatabaseURL: " realtimedb",
	}
	var err error
	app, err = firebase.NewApp(ctx, conf)
	if err != nil {
		fmt.Println("Error initializing app:", err)
		return
	}
}


func converse(userMessage string, conversationID string) string {
	client, err := app.Database(ctx)
	if err != nil {
		fmt.Println("Error initializing database client:", err)
		return ""
	}

	ref := client.NewRef("conversations/" + conversationID)
	var conversation []Conversation
	if err := ref.Get(ctx, &conversation); err != nil {
		fmt.Println("Error getting conversation:", err)
		return ""
	}

	fmt.Printf("Conversation history: %v\n", conversation)

	var messages []Conversation
	if conversation == nil {
		messages = []Conversation{
			{Role: "user", Content: userMessage, Turn: 0},
		}
	} else {
		messages = conversation
		userTurn := len(messages)
		messages = append(messages, Conversation{Role: "user", Content: userMessage, Turn: userTurn})
	}

	fmt.Printf("Messages to OpenAI API: %v\n", messages)

	assistantMessage := "Response from webhook"
	assistantTurn := len(messages)
	messages = append(messages, Conversation{Role: "assistant", Content: assistantMessage, Turn: assistantTurn})

	fmt.Printf("Updated conversation history: %v\n", messages)

	if err := ref.Set(ctx, messages); err != nil {
		fmt.Println("Error updating conversation:", err)
		return ""
	}

	return assistantMessage
}


func wtfunc(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprint(w, "Error reading request body")
		return
	}

	var dfRequest DialogflowRequest
	err = json.Unmarshal(body, &dfRequest)
	if err != nil {
		fmt.Fprint(w, "Error unmarshalling JSON")
		return
	}

	fullSessionID := dfRequest.SessionInfo.Session
	lastSlashIndex := strings.LastIndex(fullSessionID, "/")
	if lastSlashIndex == -1 || lastSlashIndex+1 >= len(fullSessionID) {
		fmt.Println("Invalid session ID format")
		return
	}
	sessionID := fullSessionID[lastSlashIndex+1:]
	fmt.Printf("Session_Id: %s\n", sessionID)

	userMessage := dfRequest.Text
	fmt.Printf("User message: %s\n", userMessage)

	assistantMessage := converse(userMessage, sessionID)

	dfResponse := DialogflowResponse{
		FulfillmentResponse: FulfillmentResponse{
			Messages: []ResponseMessage{
				{
					Text: Text{
						Text: []string{assistantMessage},
					},
				},
			},
		},
		SessionInfo: dfRequest.SessionInfo,
	}

	if err = json.NewEncoder(w).Encode(&dfResponse); err != nil {
		fmt.Fprint(w, "Error sending response")
		return
	}
}
