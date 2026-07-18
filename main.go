package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

// --- Error Consts ---
const (
	ERROR_LIVETIMING_CONNECTION_DIAL      = "Error dialing livetiming service"
	ERROR_LIVETIMING_CONNECTION_HANDSHAKE = "Error handshaking with livetiming service"
	ERROR_LIVETIMING_CONNECTION_CLOSED    = "Error received connection close message"

	ERROR_LIVETIMING_TOPIC_SUBSCRIPTION = "Error subscribing to topics on livetiming service"

	ERROR_LIVETIMING_MESSAGE_READ           = "Error reading message from livetiming service socket"
	ERROR_LIVETIMING_MESSAGE_BUFFER_WRITE   = "Error writing read message into buffer from livetiming service socket"
	ERROR_LIVETIMING_MESSAGE_INVALID_TYPE   = "Error reading message type, received invalid type"
	ERROR_LIVETIMING_MESSAGE_INVALID_TARGET = "Error reading message type, received invalid type"

	ERROR_LIVETIMING_DEBUG_FILE_OPEN        = "Error opening debug log file"
	ERROR_LIVETIMING_DEBUG_FILE_WRITE       = "Error writing to debug log file"
	ERROR_LIVETIMING_DEBUG_MESSAGE_MARSHALL = "Error marshalling debug message"

	ERROR_LIVETIMING_TARGET_NOT_FOUND = "Error finding provided target in targets"
)

// --- LTS Consts ---
const (
	LIVETIMING_LOG_PREFIX    = "[F1TV_LTS]"
	LIVETIMING_WS_URL        = "wss://livetiming.formula1.com/signalrcore?connectionToken="
	LIVETIMING_NEGOTIATE_URL = "https://livetiming.formula1.com/signalrcore/negotiate?negotiateVersion=1"

	SIGNALR_SEPERATOR = "\x1e"

	SIGNARLR_MSG_INVOCATION  = 1
	SIGNARLR_MSG_STREAM_ITEM = 2
	SIGNARLR_MSG_COMPLETION  = 3
	SIGNARLR_MSG_PING        = 6
	SIGNARLR_MSG_CLOSE       = 7
)

// --- Debug Consts ---
const (
	DEBUG_LOG_NAME = "F1TV_LTS_DBG_DUMP.json"
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

type SignalRMessage struct {
	Type      int               `json:"type"`
	Target    string            `json:"target,omitempty"`
	Error     string            `json:"error,omitempty"`
	Arguments []json.RawMessage `json:"arguments,omitempty"`
	Raw       map[string]any    `json:"-"`
}

func (m SignalRMessage) String() string {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

type LiveTimingDebugWriter struct {
	file *os.File
}

func (ltdw *LiveTimingDebugWriter) OpenFile() error {
	file, err := os.OpenFile(DEBUG_LOG_NAME, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.FileMode(0600))
	if err != nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	ltdw.file = file

	return nil
}

func (ltdw *LiveTimingDebugWriter) Write(message []byte) error {
	if ltdw.file == nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, message, "", "  "); err != nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_MESSAGE_MARSHALL)
	}

	pretty.WriteByte('\n')

	if _, err := ltdw.file.Write(pretty.Bytes()); err != nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_WRITE)
	}

	return nil
}

func (ltdw *LiveTimingDebugWriter) Close() error {
	if ltdw.file != nil {
		return ltdw.file.Close()
	}

	return nil
}

func NewLiveTimingDebugWriter() *LiveTimingDebugWriter {
	return &LiveTimingDebugWriter{}
}

type LiveTimingNegotiatedTokens struct {
	ConnectionID    string `json:"connectionId"`
	ConnectionToken string `json:"connectionToken"`
}

type LiveTimingClient struct {
	Targets          []string
	Connected        bool
	Stopped          bool
	Ws               *websocket.Conn
	NegotiatedTokens LiveTimingNegotiatedTokens
}

func (ltc *LiveTimingClient) FindTarget(target string) (string, error) {
	for i := 0; i < len(ltc.Targets); i++ {
		if ltc.Targets[i] == target {
			return target, nil
		}
	}
	return "", errors.New(ERROR_LIVETIMING_TARGET_NOT_FOUND)
}

func (ltc *LiveTimingClient) AddTarget(target string) {
	ltc.Targets = append(ltc.Targets, target)
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

func (ltc *LiveTimingClient) handleMessage(message SignalRMessage) error {

	switch message.Type {
	case SIGNARLR_MSG_PING:
		{
			fmt.Printf("%s: RESPONDED TO PING MESSAGE WITH '%v'\n", LIVETIMING_LOG_PREFIX, SIGNARLR_MSG_PING)
			ltc.Ws.WriteMessage(1, fmt.Appendf(nil, `{"type": %v}`, SIGNARLR_MSG_PING))
			return nil
		}
	case SIGNARLR_MSG_CLOSE:
		{
			if message.Error == "" {
				return fmt.Errorf("%s: %s", ERROR_LIVETIMING_CONNECTION_CLOSED, message.Error)
			} else {
				return errors.New(ERROR_LIVETIMING_CONNECTION_CLOSED)
			}
		}
	case SIGNARLR_MSG_INVOCATION:
		{
			if message.Target == "" {
				return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_TARGET)
			}

			_, err := ltc.FindTarget(message.Target)
			if err != nil {
				ltc.AddTarget(message.Target)
			}

			if message.Target == "feed" {
				if message.Arguments == nil {

					return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_TARGET)
				}
				topicPreview := string(message.Arguments[0])
				if len(topicPreview) > 80 {
					topicPreview = topicPreview[:80]
				}

				if topicPreview != "TimingData" {
					fmt.Printf(
						"Feed msg: %d args, topics=%s\n",
						len(message.Arguments),
						topicPreview,
					)
				}
			}

			return nil
		}
	default:
		return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_TYPE)
	}
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
	if err != nil {
		return errors.New(ERROR_LIVETIMING_CONNECTION_DIAL)
	}
	ltc.Ws = ws

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
		return errors.New(ERROR_LIVETIMING_CONNECTION_HANDSHAKE)
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
	debugWriter := NewLiveTimingDebugWriter()
	err = debugWriter.OpenFile()
	if err != nil {
		return err
	}

	defer debugWriter.Close()
	DEBUG_MESSAGE_COUNT := 0
	for ltc.Stopped != true {
		_, data, err := ws.ReadMessage()

		if err != nil {
			return errors.New(ERROR_LIVETIMING_MESSAGE_READ)
		}

		separatedMessagesSlice := bytes.SplitSeq(data, []byte(SIGNALR_SEPERATOR))
		for messageBytes := range separatedMessagesSlice {
			DEBUG_MESSAGE_COUNT++
			if len(messageBytes) == 0 {
				continue
			}
			var message SignalRMessage
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				fmt.Printf("%s: MESSAGE=%v\n", LIVETIMING_LOG_PREFIX, messageBytes)
				continue
			}

			// write the first 50 messages for debug purposes
			if DEBUG_MESSAGE_COUNT <= 500 {
				err = debugWriter.Write(messageBytes)
				fmt.Printf("ERROR: %s\n", err)
			}

			fmt.Printf("%s: MESSAGE=%v\n", LIVETIMING_LOG_PREFIX, message)
			ltc.handleMessage(message)
		}
	}

	return nil
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
