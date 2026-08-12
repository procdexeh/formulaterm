package livetiming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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
	// TYPE = RACE, QUALI, SPRINT, SPRINT QUALI, PRACTICE
	Type      string `json:"Type"`
	// 1 ,2, 3 i.e ^ Practice 1
	Number    int    `json:"Number"`
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

