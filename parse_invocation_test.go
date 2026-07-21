package main

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feed (invocation) payloads are partial updates. Tests only require that each
// line unmarshals; they must not assume full snapshots or complete field sets.

func TestInvocationFixtures_ParseAllLines(t *testing.T) {
	files := globNDJSON(t, filepath.Join("testdata", "invocation", "*.json"))

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			lines := readNDJSONLines(t, path)
			for i, line := range lines {
				t.Run("line_"+strconv.Itoa(i), func(t *testing.T) {
					topic, parsed := parseFeedLine(t, line)
					assert.NotEmpty(t, topic)
					assert.NotNil(t, parsed)
				})
			}
		})
	}
}

func TestInvocation_TopicTypes(t *testing.T) {
	cases := []struct {
		file  string
		topic string
		check func(t *testing.T, v any)
	}{
		{
			file:  "invocation_message_timing_data.json",
			topic: "TimingData",
			check: func(t *testing.T, v any) {
				require.IsType(t, &TimingData{}, v)
			},
		},
		{
			file:  "invocation_message_timing_app_data.json",
			topic: "TimingAppData",
			check: func(t *testing.T, v any) {
				require.IsType(t, &TimingAppData{}, v)
			},
		},
		{
			file:  "invocation_message_timing_stats.json",
			topic: "TimingStats",
			check: func(t *testing.T, v any) {
				require.IsType(t, &TimingStats{}, v)
			},
		},
		{
			file:  "invocation_message_driver_list.json",
			topic: "DriverList",
			check: func(t *testing.T, v any) {
				dl := v.(*DriverList)
				_, hasKF := (*dl)["_kf"]
				assert.False(t, hasKF)
			},
		},
		{
			file:  "invocation_message_race_control.json",
			topic: "RaceControlMessages",
			check: func(t *testing.T, v any) {
				require.IsType(t, &RaceControlMessages{}, v)
			},
		},
		{
			file:  "invocation_message_session_data.json",
			topic: "SessionData",
			check: func(t *testing.T, v any) {
				require.IsType(t, &SessionData{}, v)
			},
		},
		{
			file:  "invocation_message_weather_data.json",
			topic: "WeatherData",
			check: func(t *testing.T, v any) {
				require.IsType(t, &WeatherData{}, v)
			},
		},
		{
			file:  "invocation_message_track_status.json",
			topic: "TrackStatus",
			check: func(t *testing.T, v any) {
				require.IsType(t, &TrackStatus{}, v)
			},
		},
		{
			file:  "invocation_message_lap_count.json",
			topic: "LapCount",
			check: func(t *testing.T, v any) {
				require.IsType(t, &LapCount{}, v)
			},
		},
		{
			file:  "invocation_message_extrapolated_clock.json",
			topic: "ExtrapolatedClock",
			check: func(t *testing.T, v any) {
				require.IsType(t, &ExtrapolatedClock{}, v)
			},
		},
		{
			file:  "invocation_message_session_info.json",
			topic: "SessionInfo",
			check: func(t *testing.T, v any) {
				require.IsType(t, &SessionInfo{}, v)
			},
		},
		{
			file:  "invocation_message_session_status.json",
			topic: "SessionStatus",
			check: func(t *testing.T, v any) {
				require.IsType(t, &SessionStatus{}, v)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.topic, func(t *testing.T) {
			path := filepath.Join("testdata", "invocation", tc.file)
			lines := readNDJSONLines(t, path)
			for i, line := range lines {
				t.Run("line_"+strconv.Itoa(i), func(t *testing.T) {
					topic, parsed := parseFeedLine(t, line)
					assert.Equal(t, tc.topic, topic)
					tc.check(t, parsed)
				})
			}
		})
	}
}
