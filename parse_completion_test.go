package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Topics present in the captured completion snapshots (Position.z absent).
var completionExpectedTopics = []string{
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
}

func TestCompletionFixtures_ParseAllTopics(t *testing.T) {
	files := globNDJSON(t, filepath.Join("testdata", "completion", "*.json"))

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			lines := readNDJSONLines(t, path)
			require.Len(t, lines, 1, "completion fixtures are one envelope per file")

			parsed := parseCompletionLine(t, lines[0])

			for _, topic := range completionExpectedTopics {
				_, ok := parsed[topic]
				assert.Truef(t, ok, "missing topic %s in completion result", topic)
			}

			require.IsType(t, &TimingData{}, parsed["TimingData"])
			require.IsType(t, &TimingAppData{}, parsed["TimingAppData"])
			require.IsType(t, &TimingStats{}, parsed["TimingStats"])
			require.IsType(t, &DriverList{}, parsed["DriverList"])
			require.IsType(t, &RaceControlMessages{}, parsed["RaceControlMessages"])
			require.IsType(t, &TrackStatus{}, parsed["TrackStatus"])
			require.IsType(t, &WeatherData{}, parsed["WeatherData"])
			require.IsType(t, &LapCount{}, parsed["LapCount"])
			require.IsType(t, &ExtrapolatedClock{}, parsed["ExtrapolatedClock"])
			require.IsType(t, &SessionInfo{}, parsed["SessionInfo"])
			require.IsType(t, &SessionStatus{}, parsed["SessionStatus"])
			require.IsType(t, &SessionData{}, parsed["SessionData"])
		})
	}
}

func TestCompletionRaceMid_Structural(t *testing.T) {
	path := filepath.Join("testdata", "completion", "completion_message_race_mid.json")
	lines := readNDJSONLines(t, path)
	parsed := parseCompletionLine(t, lines[0])

	t.Run("DriverList", func(t *testing.T) {
		dl := parsed["DriverList"].(*DriverList)
		require.NotEmpty(t, *dl)
		_, hasKF := (*dl)["_kf"]
		assert.False(t, hasKF)

		var d Driver
		var found bool
		for key, driver := range *dl {
			if key == "_kf" {
				continue
			}
			d = driver
			found = true
			break
		}
		require.True(t, found)
		assert.NotEmpty(t, d.Tla)
		assert.NotEmpty(t, d.RacingNumber)
		assert.NotEmpty(t, d.TeamName)
	})

	t.Run("TimingData", func(t *testing.T) {
		td := parsed["TimingData"].(*TimingData)
		require.NotEmpty(t, td.Lines)

		var line TimingDataLine
		var ok bool
		for _, l := range td.Lines {
			line = l
			ok = true
			break
		}
		require.True(t, ok)
		assert.NotEmpty(t, line.RacingNumber)
		assert.NotEmpty(t, line.Position)
		require.NotEmpty(t, line.Sectors)
		assert.NotEmpty(t, line.BestLapTime.Value)
		assert.Greater(t, line.BestLapTime.Lap, 0)
		assert.NotEmpty(t, line.LastLapTime.Value)
		assert.Greater(t, line.NumberOfLaps, 0)
	})

	t.Run("TimingAppData", func(t *testing.T) {
		app := parsed["TimingAppData"].(*TimingAppData)
		require.NotEmpty(t, app.Lines)

		var line TimingAppDataLine
		for _, l := range app.Lines {
			line = l
			break
		}
		require.NotEmpty(t, line.Stints)
	})

	t.Run("RaceControlMessages", func(t *testing.T) {
		rcm := parsed["RaceControlMessages"].(*RaceControlMessages)
		require.NotEmpty(t, rcm.Messages)
	})

	t.Run("SessionInfo", func(t *testing.T) {
		si := parsed["SessionInfo"].(*SessionInfo)
		assert.NotEmpty(t, si.Type)
		assert.NotEmpty(t, si.Meeting.Name)
	})

	t.Run("WeatherData", func(t *testing.T) {
		w := parsed["WeatherData"].(*WeatherData)
		assert.NotEmpty(t, w.AirTemp)
		assert.NotEmpty(t, w.TrackTemp)
	})
}

func TestCompletionRaceStart_Structural(t *testing.T) {
	path := filepath.Join("testdata", "completion", "completion_message_race_start.json")
	lines := readNDJSONLines(t, path)
	parsed := parseCompletionLine(t, lines[0])

	td := parsed["TimingData"].(*TimingData)
	require.NotEmpty(t, td.Lines)

	lc := parsed["LapCount"].(*LapCount)
	assert.GreaterOrEqual(t, lc.TotalLaps, 1)

	ts := parsed["TrackStatus"].(*TrackStatus)
	assert.NotEmpty(t, ts.Status)
}
