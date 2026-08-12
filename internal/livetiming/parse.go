package livetiming

import (
	"encoding/json"
	"errors"
)

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

// --- Errors ---

var ErrParseInvalidTopic = errors.New("parse: tried to parse an invalid topic")
var ErrParseJson = errors.New("parse: error parsing topic")

// --- Parsing ---

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
		return nil, ErrParseInvalidTopic
	}

	if err != nil {
		return nil, ErrParseJson
	}

	return out, nil
}
