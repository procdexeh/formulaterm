package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ERROR_LIVETIMING_DIAL                 = "Error dialing livetiming service"
	ERROR_LIVETIMING_HANDSHAKE            = "Error handshaking with livetiming service"
	ERROR_LIVETIMING_TOPIC_SUBSCRIPTION   = "Error subscribing to topics on livetiming service"
	ERROR_LIVETIMING_MESSAGE_READ         = "Error reading message from livetiming service socket"
	ERROR_LIVETIMING_MESSAGE_BUFFER_WRITE = "Error writing read message into buffer from livetiming service socket"
)

const (
	LIVETIMING_LOG_PREFIX    = "[F1TV_LTS]"
	LIVETIMING_WS_URL        = "wss://livetiming.formula1.com/signalrcore?connectionToken="
	LIVETIMING_NEGOTIATE_URL = "https://livetiming.formula1.com/signalrcore/negotiate?negotiateVersion=1"

	SIGNALR_SEPERATOR       = "\x1e"
	SIGNARLR_MSG_INVOCATION = 1
	SIGNARLR_MSG_PING       = 6
	SIGNARLR_MSG_CLOSE      = 7
)

var LIVETIMING_TOPICS = []string{
	"TimingData",
	"TimingAppData",
	"TimingStats",
	"DriverList",
	"RaceControlMessages",
	"TrackStatus",
	"WeatherData",
	"LapCount",
	"ExtrapolatedClock",
	"SessionInfo",
	"SessionStatus",
	"SessionData",
	"Position.z",
}

type LiveTimingNegotiatedTokens struct {
	ConnectionID    string `json:"connectionId"`
	ConnectionToken string `json:"connectionToken"`
}

type LiveTimingClient struct {
	Connected        bool
	exit             bool
	Ws               *websocket.Conn
	NegotiatedTokens LiveTimingNegotiatedTokens
}

func (ltc *LiveTimingClient) negotiate() error {
	req, err := http.NewRequest(
		"POST",
		LIVETIMING_NEGOTIATE_URL,
		bytes.NewBuffer([]byte{}),
	)

	if err != nil {
		return err
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var ltt LiveTimingNegotiatedTokens
	err = json.Unmarshal(body, &ltt)

	ltc.NegotiatedTokens = ltt

	return nil
}

func (ltc *LiveTimingClient) Connect() error {
	// -- NEGOTIATE ---
	err := ltc.negotiate()
	if err != nil {
		return nil
	}

	// -- DIAL WITH CREDS ---
	ws, _, err := websocket.DefaultDialer.Dial(
		LIVETIMING_WS_URL+ltc.NegotiatedTokens.ConnectionToken,
		nil,
	)
	defer ws.Close()

	if err != nil {
		return errors.New(ERROR_LIVETIMING_DIAL)
	}
	fmt.Printf("%s: Connected.\n", LIVETIMING_LOG_PREFIX)

	// --- HANDSHAKE ---
	handshake := `{"protocol":"json","version":1}` + SIGNALR_SEPERATOR
	err = ws.WriteMessage(
		websocket.TextMessage,
		[]byte(handshake),
	)

	if err != nil {
		log.Fatal(err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Fatal(err)
	}

	if len(msg) != len([]byte(`{}`+SIGNALR_SEPERATOR)) {
		return errors.New(ERROR_LIVETIMING_HANDSHAKE)
	}

	fmt.Printf("%s: Handshake successful.\n", LIVETIMING_LOG_PREFIX)

	// --- SUBSCRIBE TO TOPICS ---
	sub := map[string]any{
		"type":         SIGNARLR_MSG_INVOCATION,
		"invocationId": "1",
		"target":       "Subscribe",
		"arguments":    [][]string{LIVETIMING_TOPICS},
	}

	b, _ := json.Marshal(sub)

	err = ws.WriteMessage(
		websocket.TextMessage,
		append(b, SIGNALR_SEPERATOR...),
	)

	if err != nil {
		fmt.Printf("%s: ERROR=%v\n", LIVETIMING_LOG_PREFIX, err)
		return errors.New(ERROR_LIVETIMING_TOPIC_SUBSCRIPTION)
	}

	// --- Message Reading ---
	for {
		_, data, err := ws.ReadMessage()

		if err != nil {
			return errors.New(ERROR_LIVETIMING_MESSAGE_READ)
		}

		separatedMessagesSlice := bytes.SplitSeq(data, []byte(SIGNALR_SEPERATOR))
		for messageBytes := range separatedMessagesSlice {
			if len(messageBytes) == 0 {
				continue
			}
			var message map[string]any
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				fmt.Printf("%s: MESSAGE=%v\n", LIVETIMING_LOG_PREFIX, messageBytes)
				continue
			}

			fmt.Printf("%s: MESSAGE=%s\n", LIVETIMING_LOG_PREFIX, message)
		}
	}
}

func NewLiveTimingClient() *LiveTimingClient {
	return &LiveTimingClient{}
}

func main() {
	client := NewLiveTimingClient()
	err := client.Connect()
	if err != nil {
		log.Fatal(err)
	}
}






