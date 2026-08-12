package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	ERROR_LIVETIMING_MESSAGE_WRITE          = "Error writing message to livetiming service socket"
	ERROR_LIVETIMING_MESSAGE_BAD_SOCKET     = "Error tried interacting with a socket that is nil"
	ERROR_LIVETIMING_MESSAGE_BUFFER_WRITE   = "Error writing read message into buffer from livetiming service socket"
	ERROR_LIVETIMING_MESSAGE_INVALID_TYPE   = "Error reading message type, received invalid type"
	ERROR_LIVETIMING_MESSAGE_INVALID_TARGET = "Error reading message type, received invalid type"
	ERROR_LIVETIMING_MESSAGE_INVALID_RESULT = "Error reading message result, expected it to be populated"
	ERROR_LIVETIMING_MESSAGE_INVALID_TOPIC  = "Error parsing topic from message, received invalid topic"
	ERROR_LIVETIMING_MESSAGE_TOPIC_PARSE    = "Error parsing topic"

	ERROR_LIVETIMING_DEBUG_FILE_OPEN        = "Error opening debug log file"
	ERROR_LIVETIMING_DEBUG_FILE_WRITE       = "Error writing to debug log file"
	ERROR_LIVETIMING_DEBUG_FILE_READ        = "Error failed to read debug file"
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
	DEBUG_LOG_NAME = "F1TV_LTS_DBG_RAW.json"
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
	"CarData.z",
}

// Rename this stuff better
type LiveTimingTopic int64

const (
	TimingDataTopic LiveTimingTopic = iota
	TimingAppDataTopic
	TimingStatsTopic
	DriverListTopic
	RaceControlMessagesTopic
	TrackStatusTopic
	WeatherDataTopic
	LapCountTopic
	ExtrapolatedClockTopic
	SessionInfoTopic
	SessionStatusTopic
	SessionDataTopic
	PositionZTopic
)

// --- Live Timing Messsage

type LiveTimingMessage struct {
	Type      int                        `json:"type"`
	Target    string                     `json:"target,omitempty"`
	Error     string                     `json:"error,omitempty"`
	Result    map[string]json.RawMessage `json:"result,omitempty"`
	Arguments []json.RawMessage          `json:"arguments,omitempty"`
}

// --- Weather Data ---

type WeatherData struct {
	AirTemp       string `json:"AirTemp"`
	Humidity      string `json:"Humidity"`
	Pressure      string `json:"Pressure"`
	Rainfall      string `json:"Rainfall"`
	TrackTemp     string `json:"TrackTemp"`
	WindDirection string `json:"WindDirection"`
	WindSpeed     string `json:"WindSpeed"`
}

// IndexMap accepts JSON arrays (snapshots) or objects (sparse feed deltas).
// Arrays are normalized to string keys "0", "1", ...
type IndexMap[T any] map[string]T

func (m *IndexMap[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)

	if bytes.Equal(data, []byte("null")) {
		*m = nil
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("unexpected empty JSON value")
	}

	switch data[0] {
	case '[':
		var list []T
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		out := make(IndexMap[T], len(list))
		for i, v := range list {
			out[strconv.Itoa(i)] = v
		}
		*m = out
		return nil
	case '{':
		var obj map[string]T
		if err := json.Unmarshal(data, &obj); err != nil {
			return err
		}
		*m = IndexMap[T](obj)
		return nil
	default:
		return fmt.Errorf("unexpected JSON format: %s", data)
	}
}

// FlexBool accepts JSON bool or string forms ("true"/"false") used by F1 feeds.
type FlexBool bool

func (b *FlexBool) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.Equal(data, []byte("null")):
		*b = false
		return nil
	case bytes.Equal(data, []byte("true")), bytes.Equal(data, []byte(`"true"`)), bytes.Equal(data, []byte(`"True"`)):
		*b = true
		return nil
	case bytes.Equal(data, []byte("false")), bytes.Equal(data, []byte(`"false"`)), bytes.Equal(data, []byte(`"False"`)):
		*b = false
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		switch s {
		case "1", "true", "True":
			*b = true
			return nil
		case "0", "false", "False", "":
			*b = false
			return nil
		}
	}

	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("FlexBool: %s", data)
	}
	*b = FlexBool(v)
	return nil
}

func (b FlexBool) Bool() bool { return bool(b) }

// --- Timing App Data ---

type TimingAppDataStint struct {
	LapFlags        int      `json:"LapFlags,omitempty"`
	Compound        string   `json:"Compound,omitempty"`
	New             FlexBool `json:"New,omitempty"`
	TyresNotChanged string   `json:"TyresNotChanged,omitempty"`
	TotalLaps       int      `json:"TotalLaps,omitempty"`
	StartLaps       int      `json:"StartLaps,omitempty"`

	LapTime   string `json:"LapTime,omitempty"`
	LapNumber int    `json:"LapNumber,omitempty"`
}

type TimingAppDataLine struct {
	RacingNumber string                       `json:"RacingNumber"`
	Stints       IndexMap[TimingAppDataStint] `json:"Stints"`
	Line         int                          `json:"Line"`
	GridPosition string                       `json:"GridPos"`
}

type TimingAppData struct {
	Lines map[string]TimingAppDataLine `json:"Lines"`
}

// --- Timing Data ---
// Live feed fields are often split across messages (partial updates).
// Sectors arrive as [] in snapshots and {} in deltas.

type IntervalToPositionAhead struct {
	Value    string `json:"Value"`
	Catching bool   `json:"Catching"`
}

type TimingDataSectors struct {
	Stopped         bool   `json:"Stopped"`
	Value           string `json:"Value"`
	Status          int    `json:"Status"`
	OverallFastest  bool   `json:"OverallFastest"`
	PersonalFastest bool   `json:"PersonalFastest"`
	// Don't really care about segments for now because I don't even know what they're used for.
	// During a whole race session they were empty essentially.
	// Segments      map[string]string `json:"Segments"`
	PreviousValue string `json:"PreviousValue,omitempty"`
}

type TimingDataSpeeds struct {
	Value           string `json:"Value"`
	Status          int    `json:"Status"`
	OverallFastest  bool   `json:"OverallFastest"`
	PersonalFastest bool   `json:"PersonalFastest"`
}

type TimingDataBestLapTime struct {
	Value string `json:"Value"`
	Lap   int    `json:"Lap,omitempty"`
}

type TimingDataLastLapTime struct {
	Value           string `json:"Value"`
	Status          int    `json:"Status"`
	OverallFastest  bool   `json:"OverallFastest"`
	PersonalFastest bool   `json:"PersonalFastest"`
}

type TimingDataSectorsMap = IndexMap[TimingDataSectors]

type TimingDataLine struct {
	GapToLeader             string                      `json:"GapToLeader"`
	IntervalToPositionAhead IntervalToPositionAhead     `json:"IntervalToPositionAhead"`
	Line                    int                         `json:"Line"`
	Position                string                      `json:"Position"`
	ShowPosition            bool                        `json:"ShowPosition"`
	RacingNumber            string                      `json:"RacingNumber"`
	Retired                 bool                        `json:"Retired"`
	InPit                   bool                        `json:"InPit"`
	PitOut                  bool                        `json:"PitOut"`
	Stopped                 bool                        `json:"Stopped"`
	Status                  int                         `json:"Status"`
	NumberOfLaps            int                         `json:"NumberOfLaps"`
	NumberOfPitStops        int                         `json:"NumberOfPitStops"`
	BestLapTime             TimingDataBestLapTime       `json:"BestLapTime"`
	LastLapTime             TimingDataLastLapTime       `json:"LastLapTime"`
	Sectors                 TimingDataSectorsMap        `json:"Sectors"`
	Speeds                  map[string]TimingDataSpeeds `json:"Speeds"`
}

type TimingData struct {
	Lines    map[string]TimingDataLine `json:"Lines"`
	Withheld bool                      `json:"Withheld"`
}

// --- Timing Stats ---

type TimingStatsPersonalBestLapTime struct {
	Value    string `json:"Value"`
	Lap      int    `json:"Lap"`
	Position int    `json:"Position"`
}

type TimingStatsBestSectors struct {
	Value    string `json:"Value"`
	Position int    `json:"Position"`
}

type TimingStatsBestSpeed struct {
	Value    string `json:"Value"`
	Position int    `json:"Position"`
}

type TimingStatsLine struct {
	Line                int                              `json:"Line"`
	RacingNumber        string                           `json:"RacingNumber"`
	PersonalBestLapTime TimingStatsPersonalBestLapTime   `json:"PersonalBestLapTime"`
	BestSectors         IndexMap[TimingStatsBestSectors] `json:"BestSectors"`
	BestSpeeds          map[string]TimingStatsBestSpeed  `json:"BestSpeeds"`
}

type TimingStats struct {
	Withheld bool                       `json:"Withheld"`
	Lines    map[string]TimingStatsLine `json:"Lines"`
}

// --- Driver List ---

type DriverList map[string]Driver

// For some reason f1 decided to embed _kf: bool into a map[driver_number]Driver
func (dl *DriverList) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := make(DriverList)
	for key, value := range raw {
		if key == "_kf" {
			continue
		}

		var d Driver
		if err := json.Unmarshal(value, &d); err != nil {
			return err
		}

		out[key] = d
	}

	*dl = out
	return nil
}

type Driver struct {
	RacingNumber  string `json:"RacingNumber"`
	BroadcastName string `json:"BroadcastName"`
	FullName      string `json:"FullName"`
	Tla           string `json:"Tla"`
	Line          int    `json:"Line"`
	TeamName      string `json:"TeamName"`
	TeamColour    string `json:"TeamColour"`
	FirstName     string `json:"FirstName"`
	LastName      string `json:"LastName"`
	Reference     string `json:"Reference"`
	HeadshotUrl   string `json:"HeadshotUrl"`
	PublicIdRight string `json:"PublicIdRight"`
}

// --- Race Control Messages ---

type RaceControlMessages struct {
	Messages IndexMap[RaceControlMessage] `json:"Messages"`
}

type RaceControlMessage struct {
	Utc string `json:"Utc"`
	Lap int    `json:"Lap"`
	// Category = Flag, SafetyCar, Driver, Other
	Category string `json:"Category"`
	// If category is flag, these 2 fields should exist
	// Flag = GREEN, YELLOW, DOUBLE YELLOW, RED, CLEAR
	Flag string `json:"Flag,omitempty"`
	// Scope = Track, Sector, Driver, ENDING
	Scope        string `json:"Scope,omitempty"`
	Sector       int    `json:"Sector,omitempty"`
	RacingNumber string `json:"RacingNumber,omitempty"`
	// I believe Status exists if Mode is either VSC or SC, Status = ENDING, DEPLOYED, IN THIS LAP
	Status string `json:"Status,omitempty"`
	// Mode = VSC, SAFETY CAR
	Mode string `json:"Mode,omitempty"`

	Message string `json:"Message"`
}

// --- Track Status ---

type LapCount struct {
	CurrentLap int `json:"CurrentLap"`
	TotalLaps  int `json:"TotalLaps"`
}

// --- Extrapolated Clock ---

type ExtrapolatedClock struct {
	Utc           string `json:"Utc"`
	Remaining     string `json:"Remaining"`
	Extrapolating bool   `json:"Extrapolating"`
}

// --- Session Info ---

type SessionInfoCountry struct {
	Key  int    `json:"Key"`
	Code string `json:"Code"`
	Name string `json:"Name"`
}

type SessionInfoCircuit struct {
	Key       int    `json:"Key"`
	ShortName string `json:"ShortName"`
}

type SessionInfoMeeting struct {
	Key          int                `json:"Key"`
	Name         string             `json:"Name"`
	OfficialName string             `json:"OfficialName"`
	Location     string             `json:"Location"`
	Number       int                `json:"Number"`
	Country      SessionInfoCountry `json:"Country"`
	Circuit      SessionInfoCircuit `json:"Circuit"`
}

type SessionInfoArchiveStatus struct {
	// STATUS = GENERATING, ???
	Status string `json:"Status"`
}

type SessionInfo struct {
	Meeting SessionInfoMeeting `json:"Meeting"`
	// SessionStatus = INACTIVE, ACTIVE, ???
	SessionStatus string                   `json:"SessionStatus"`
	ArchiveStatus SessionInfoArchiveStatus `json:"ArchiveStatus"`
	Key           int                      `json:"Key"`
	// TYPE = RACE, QUALI, SPRINT, SPRINT QUALI, FP1, FP2, FP3,
	Type      string `json:"Type"`
	Name      string `json:"Name"`
	StartDate string `json:"StartDate"`
	EndDate   string `json:"EndDate"`
	GmtOffset string `json:"GmtOffset"`
	Path      string `json:"Path"`
}

// --- Session Status

type SessionStatus struct {
	Status  string `json:"Status"`
	Started string `json:"Started"`
}

// --- Session Data ---

type SessionDataSeries struct {
	Utc string `json:"Utc"`
	Lap int    `json:"Lap"`
}

type SessionDataStatusSeries struct {
	Utc           string `json:"Utc"`
	TrackStatus   string `json:"TrackStatus,omitempty"`
	SessionStatus string `json:"SessionStatus,omitempty"`
}

type SessionData struct {
	Series       IndexMap[SessionDataSeries]       `json:"Series"`
	StatusSeries IndexMap[SessionDataStatusSeries] `json:"StatusSeries"`
}

// --- Track Status ---

type TrackStatus struct {
	Status  string `json:"Status"`
	Message string `json:"Message"`
}

type ParseTopicResult struct {
	result any
	kind   LiveTimingTopic
}

func ParseTopic(topic string, message json.RawMessage) (any, error) {
	var (
		out any
		err error
	)
	switch topic {
	case "TimingData":
		parsedMessage := &TimingData{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "TimingAppData":
		parsedMessage := &TimingAppData{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "TimingStats":
		parsedMessage := &TimingStats{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "DriverList":
		parsedMessage := &DriverList{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "RaceControlMessages":
		parsedMessage := &RaceControlMessages{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "TrackStatus":
		parsedMessage := &TrackStatus{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "WeatherData":
		parsedMessage := &WeatherData{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "LapCount":
		parsedMessage := &LapCount{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "ExtrapolatedClock":
		parsedMessage := &ExtrapolatedClock{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "SessionInfo":
		parsedMessage := &SessionInfo{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "SessionStatus":
		parsedMessage := &SessionStatus{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	case "SessionData":
		parsedMessage := &SessionData{}
		err = json.Unmarshal(message, parsedMessage)
		out = parsedMessage
	// case "Position.z":
	default:
		return nil, fmt.Errorf("%s: '%s'", ERROR_LIVETIMING_MESSAGE_INVALID_TOPIC, topic)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", ERROR_LIVETIMING_MESSAGE_INVALID_TOPIC, err)
	}

	return out, nil
}

func (m LiveTimingMessage) String() string {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

type LiveTimingDebugWriter struct {
	file    *os.File
	scanner *bufio.Scanner
}

func (ltdw *LiveTimingDebugWriter) OpenFile() error {
	file, err := os.OpenFile(DEBUG_LOG_NAME, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.FileMode(0600))
	if err != nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	ltdw.file = file

	return nil
}

func (ltdw *LiveTimingDebugWriter) OpenReadFile() error {
	file, err := os.Open(DEBUG_LOG_NAME)
	if err != nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	ltdw.file = file
	ltdw.scanner = bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	ltdw.scanner.Buffer(buf, 4<<20)

	return nil
}

func (ltdw *LiveTimingDebugWriter) Read() ([]byte, error) {
	if ltdw.file == nil || ltdw.scanner == nil {
		return nil, errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	if ltdw.scanner.Scan() {
		line := append([]byte(nil), ltdw.scanner.Bytes()...)
		return line, nil
	}

	if err := ltdw.scanner.Err(); err != nil {
		fmt.Printf("%s: ERROR: %s", LIVETIMING_LOG_PREFIX, err)
		return nil, errors.New(ERROR_LIVETIMING_DEBUG_FILE_READ)
	}

	return nil, io.EOF
}

func (ltdw *LiveTimingDebugWriter) Write(message []byte, pretty bool) error {
	if ltdw.file == nil {
		return errors.New(ERROR_LIVETIMING_DEBUG_FILE_OPEN)
	}

	var prettyBuff bytes.Buffer
	if pretty {
		if err := json.Indent(&prettyBuff, message, "", "  "); err != nil {
			return errors.New(ERROR_LIVETIMING_DEBUG_MESSAGE_MARSHALL)
		}
	} else {
		prettyBuff.Write(message)
	}

	prettyBuff.WriteByte('\n')

	if _, err := ltdw.file.Write(prettyBuff.Bytes()); err != nil {
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

func (ltc *LiveTimingClient) handleMessage(message LiveTimingMessage) error {
	switch message.Type {
	case SIGNARLR_MSG_COMPLETION:
		{
			if len(message.Result) == 0 {
				// TODO: fix this error.
				return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_RESULT)
			}

			for key, value := range message.Result {
				data, err := ParseTopic(key, value)
				if err != nil {
					fmt.Printf("%s: ERROR=%s\n", LIVETIMING_LOG_PREFIX, err)
				}
				fmt.Printf("%s: PARSED_TOPIC=%s VALUE=%v\n", LIVETIMING_LOG_PREFIX, key, data)
			}

			return nil
		}
	case SIGNARLR_MSG_INVOCATION:
		{
			if message.Target == "" {
				return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_TARGET)
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
	case SIGNARLR_MSG_PING:
		{
			ltc.Write(1, fmt.Appendf(nil, `{"type": %v}`, SIGNARLR_MSG_PING))
			fmt.Printf("%s: RESPONDED TO PING MESSAGE WITH '%v'\n", LIVETIMING_LOG_PREFIX, SIGNARLR_MSG_PING)
			return nil
		}
	case SIGNARLR_MSG_CLOSE:
		{
			if message.Error != "" {
				return fmt.Errorf("%s: %s", ERROR_LIVETIMING_CONNECTION_CLOSED, message.Error)
			} else {
				ltc.Stopped = true
				return nil
			}
		}
	default:
		return errors.New(ERROR_LIVETIMING_MESSAGE_INVALID_TYPE)
	}
}

func (ltc *LiveTimingClient) Close() {
	if ltc != nil {
		ltc.Ws.Close()
		ltc.Ws = nil
	}
}

func (ltc *LiveTimingClient) Write(messageType int, b []byte) error {
	if ltc.Ws == nil {
		return errors.New(ERROR_LIVETIMING_MESSAGE_BAD_SOCKET)
	}

	if err := ltc.Ws.WriteMessage(messageType, b); err != nil {
		return errors.New(ERROR_LIVETIMING_MESSAGE_WRITE)
	}

	return nil
}

func (ltc *LiveTimingClient) parseRawMessage(data []byte) error {
	separatedMessagesSlice := bytes.SplitSeq(data, []byte(SIGNALR_SEPERATOR))
	for messageBytes := range separatedMessagesSlice {
		if len(messageBytes) == 0 {
			continue
		}

		var message LiveTimingMessage
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			fmt.Printf("%s: MESSAGE=%v\n", LIVETIMING_LOG_PREFIX, messageBytes)
			continue
		}

		ltc.handleMessage(message)
	}
	return nil
}

func (ltc *LiveTimingClient) Replay() error {
	reader := NewLiveTimingDebugWriter()

	if err := reader.OpenReadFile(); err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	for {
		data, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		err = ltc.parseRawMessage(data)
		if err != nil {
			fmt.Printf("%s: ERROR: %s\n", LIVETIMING_LOG_PREFIX, err)
		}
		break
	}
	return nil
}

func (ltc *LiveTimingClient) Connect() error {
	// -- NEGOTIATE ---
	err := ltc.negotiate()
	if err != nil {
		return nil
	}

	// The messages can be quite big due to zipped telemetry being sent, 4MiB *should* suffice
	d := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		ReadBufferSize:   4 << 20,
	}

	const (
		maxRetries     = 3
		initialBackoff = 1 * time.Second
	)

	// Eventually refactor out this goto, i don't like them generally but I'd rather make quick forward progress.
connect:
	backoff := initialBackoff
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ltc.Ws != nil {
			ltc.Close()
		}
		
		// -- DIAL WITH CREDS ---
		ws, _, err := d.Dial(
			LIVETIMING_WS_URL+ltc.NegotiatedTokens.ConnectionToken+"&access_token=eyJraWQiOiIxIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYifQ.eyJFeHRlcm5hbEF1dGhvcml6YXRpb25zQ29udGV4dERhdGEiOiJDQU4iLCJTdWJzY3JpcHRpb25TdGF0dXMiOiJhY3RpdmUiLCJTdWJzY3JpYmVySWQiOiIxODIyNzA0MTIiLCJGaXJzdE5hbWUiOiJKZWFuLVNlYmFzdGllbiIsImVudHMiOlt7ImNvdW50cnkiOiJDQU4iLCJlbnQiOiJSRUcifSx7ImNvdW50cnkiOiJDQU4iLCJlbnQiOiJQUk8ifV0sIkxhc3ROYW1lIjoiR2lyb3V4IiwiZXhwIjoxNzg1MjM4MTYwLCJTZXNzaW9uSWQiOiJleUpoYkdjaU9pSm9kSFJ3T2k4dmQzZDNMbmN6TG05eVp5OHlNREF4THpBMEwzaHRiR1J6YVdjdGJXOXlaU05vYldGakxYTm9ZVEkxTmlJc0luUjVjQ0k2SWtwWFZDSjkuZXlKaWRTSTZJakV3TURFeElpd2ljMmtpT2lJMk1HRTVZV1E0TkMxbE9UTmtMVFE0TUdZdE9EQmtOaTFoWmpNM05EazBaakpsTWpJaUxDSm9kSFJ3T2k4dmMyTm9aVzFoY3k1NGJXeHpiMkZ3TG05eVp5OTNjeTh5TURBMUx6QTFMMmxrWlc1MGFYUjVMMk5zWVdsdGN5OXVZVzFsYVdSbGJuUnBabWxsY2lJNklqRTRNakkzTURReE1pSXNJbWxrSWpvaVlUUmhZV05rWmpFdE5tRTJZeTAwTkRJekxXRXlZVEV0WVdOak0ySm1aRGswTnprMUlpd2lkQ0k2SWpFaUxDSnNJam9pWlc0dFIwSWlMQ0prWXlJNklqTTJORFFpTENKaFpXUWlPaUl5TURJMkxUQTRMVEEzVkRFeE9qSTVPakl3TGpRM05Gb2lMQ0prZENJNklqRWlMQ0psWkNJNklqSXdNall0TURndE1qTlVNVEU2TWprNk1qQXVORGMwV2lJc0ltTmxaQ0k2SWpJd01qWXRNRGN0TWpWVU1URTZNams2TWpBdU5EYzBXaUlzSW1sd0lqb2lNbUV3TURveFpXSTRPbU13TlRrNllqZG1Oem8zWlRnek9qbG1Oelk2T1RBM09Eb3lZVGxpSWl3aVl5STZJa1JQVFVWSlMwRldRU0lzSW5OMElqb2lTMVVpTENKd1l5STZJalUwTXpRd0lpd2lZMjhpT2lKTVZGVWlMQ0p1WW1ZaU9qRTNPRFE0T1RJMU5qQXNJbVY0Y0NJNk1UYzROelE0TkRVMk1Dd2lhWE56SWpvaVlYTmpaVzVrYjI0dWRIWWlMQ0poZFdRaU9pSmhjMk5sYm1SdmJpNTBkaUo5LkZ5WGx2U0NfeEwxb3ptQkpPdGYtNy0tTmlIbWtoVzdjRlFxa2xCM1VPM00iLCJpYXQiOjE3ODQ4OTI1NjAsIlN1YnNjcmliZWRQcm9kdWN0IjoiRjEgVFYgUHJvIEFubnVhbCIsImp0aSI6IjdkODcxYTgwLTFmZTEtNGM0Zi1iMzVjLTk5ZDdiZjJmZTJlNyIsImhhc2hlZFN1YnNjcmliZXJJZCI6IkdIVGZuenVYSUpadnhQSHZsOXlBNEF6RVFoR2NtYVZmVnB3cExDRnZ1eHM9In0.Rd4PjCEVQvp1JpOS1KmEfe7vy5fa_eoyr0LzUP9len0Wjoyn7DC_5AGB35sQ5BAuIo8L-9sceQFO5tKPhQBrHZnKQzn26RWkqo3xcU9aLa_5M5KDKaX24Xlfsx5jLRqzcgR-wHp_aOLSFXxlUY-htFynYFGfw2SBYo5iCucX-AabNXxDioS0BRXcYziPJvPAcBDI4zLNAR2VUgYi8qAxhBIzmDJUyJuCtM5uez4MzqCbTYLIFRu5AU6nxiTgGUOAyUE3RUDNWnQMTJRnIu655kOpOslN3cU0rPbXOjwaTB_cxsaV-s70viFm8gQPAFLSV9kVFW3vUXkk0OuV3QbHng",
			nil,
		)
		if err == nil {
			ltc.Ws = ws
			fmt.Printf("%s: Connected.\n", LIVETIMING_LOG_PREFIX)
			break
		}

		if attempt == maxRetries {
			return errors.New(ERROR_LIVETIMING_CONNECTION_DIAL)
		}

		fmt.Printf(
			"%s: Dial failed (attempt %d/%d): %v. Retrying in %v...\n",
			LIVETIMING_LOG_PREFIX,
			attempt,
			maxRetries,
			err,
			backoff,
		)

		time.Sleep(backoff)
		backoff *= 4
	}

	// --- HANDSHAKE ---
	handshake := `{"protocol":"json","version":1}` + SIGNALR_SEPERATOR
	err = ltc.Write(
		websocket.TextMessage,
		[]byte(handshake),
	)

	if err != nil {
		log.Fatal(err)
	}
	_, msg, err := ltc.Ws.ReadMessage()
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

	err = ltc.Write(
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
	for ltc.Stopped != true {
		_, data, err := ltc.Ws.ReadMessage()
		if err != nil {
			return errors.New(ERROR_LIVETIMING_MESSAGE_READ)
		}

		err = debugWriter.Write(data, false)
		if err != nil {
			fmt.Printf("%s: DEBUG_WRITE_ERROR=%v\n", LIVETIMING_LOG_PREFIX, err)
		}

		err = ltc.parseRawMessage(data)
		if err != nil {
			if err.Error() == ERROR_LIVETIMING_CONNECTION_CLOSED {
				goto connect
			} else {
				fmt.Printf("%s: HANDLE_MESSAGE_ERROR=%v\n", LIVETIMING_LOG_PREFIX, err)
			}
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
