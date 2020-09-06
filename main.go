package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type event struct {
	Type     string
	Text     string
	Channel  string
	ThreadTS string `json:"thread_ts"`
}

type eventMessage struct {
	Challenge   string
	Event       event
	AuthedUsers []string `json:"authed_users"`
}

type chatMessage struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

type bot struct {
	token  string
	secret string
}

func newBot(token string, secret string) (b *bot) {
	if len(secret) == 0 {
		log.Fatal("Missing secret. Unable to authenticate requests.")
	}

	return &bot{token, secret}
}

func (b *bot) eventHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)

	// Check for a recent timestamp to avoid replay attacks.
	t, _ := strconv.Atoi(r.Header.Get("X-Slack-Request-Timestamp"))
	if math.Abs(float64(time.Now().Unix()-int64(t))) > 300 {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, "Timestamp differs by more than 5 minutes.")
		return
	}
	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(b.secret))
	fmt.Fprintf(mac, "v0:%d:%s", t, body)
	digest := mac.Sum(nil)
	// Reject invalid signatures
	if r.Header.Get("X-Slack-Signature") != "v0="+hex.EncodeToString(digest) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, "Invalid signature.")
		return
	}

	var msg eventMessage
	json.Unmarshal(body, &msg)

	// For challenge requests, just echo the challenge field
	if len(msg.Challenge) > 0 {
		fmt.Fprintln(w, msg.Challenge)
		return
	}

	// When the bot is mentioned, respond with a funny message
	if msg.Event.Type == "app_mention" {
		// Remove the bot's user ID from the message.
		// E.g. "<@U123abc> how do you make scrambled eggs?" -->
		//      "how do you make scrambled eggs?"
		user := msg.AuthedUsers[0]
		text := strings.TrimSpace(
			strings.ReplaceAll(msg.Event.Text, "<@"+user+">", ""))

		// Make a response
		response := getInstructions(text)

		// Send the message to the right channel/thread
		c := chatMessage{msg.Event.Channel, response, msg.Event.ThreadTS}
		j, _ := json.Marshal(c)
		req, _ := http.NewRequest(http.MethodPost,
			"https://slack.com/api/chat.postMessage", bytes.NewBuffer(j))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.token) // OAuth token
		http.DefaultClient.Do(req)
	}
}

func main() {
	bot := newBot(os.Getenv("TOKEN"), os.Getenv("SECRET"))
	http.HandleFunc("/", bot.eventHandler)

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "5000"
	}
	fmt.Printf("Listening on port %s\n", port)
	panic(http.ListenAndServe(":"+port, nil))
}
