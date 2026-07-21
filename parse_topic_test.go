package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopic_UnknownTopic(t *testing.T) {
	_, err := ParseTopic("NotARealTopic", json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestParseTopic_TimingData_SectorsArrayAndMap(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantKey string
		wantVal string
	}{
		{
			name: "array snapshot shape",
			payload: `{
				"Lines": {
					"1": {
						"RacingNumber": "1",
						"Position": "1",
						"Sectors": [
							{"Value": "28.100", "Status": 2048, "OverallFastest": false, "PersonalFastest": true},
							{"Value": "31.200", "Status": 2048},
							{"Value": "25.300", "Status": 2048}
						]
					}
				}
			}`,
			wantKey: "0",
			wantVal: "28.100",
		},
		{
			name: "map feed shape",
			payload: `{
				"Lines": {
					"1": {
						"Sectors": {
							"0": {"Value": "28.050", "Status": 2048, "PersonalFastest": true},
							"1": {"Segments": {"0": {"Status": 2048}}}
						}
					}
				}
			}`,
			wantKey: "0",
			wantVal: "28.050",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseTopic("TimingData", json.RawMessage(tc.payload))
			require.NoError(t, err)
			td := out.(*TimingData)
			line, ok := td.Lines["1"]
			require.True(t, ok, "missing line 1")
			sector, ok := line.Sectors[tc.wantKey]
			require.True(t, ok, "missing sector %s in %#v", tc.wantKey, line.Sectors)
			assert.Equal(t, tc.wantVal, sector.Value)
		})
	}
}

func TestParseTopic_TimingData_LapTimesAndCounts(t *testing.T) {
	payload := json.RawMessage(`{
		"Lines": {
			"81": {
				"RacingNumber": "81",
				"Position": "2",
				"NumberOfLaps": 26,
				"NumberOfPitStops": 1,
				"BestLapTime": {"Value": "1:49.333", "Lap": 26},
				"LastLapTime": {
					"Value": "1:49.784",
					"Status": 0,
					"OverallFastest": false,
					"PersonalFastest": true
				},
				"Speeds": {
					"ST": {"Value": "312", "Status": 2048, "OverallFastest": false, "PersonalFastest": true}
				},
				"Sectors": []
			}
		}
	}`)

	out, err := ParseTopic("TimingData", payload)
	require.NoError(t, err)
	td := out.(*TimingData)
	line, ok := td.Lines["81"]
	require.True(t, ok)

	assert.Equal(t, 26, line.NumberOfLaps)
	assert.Equal(t, 1, line.NumberOfPitStops)
	assert.Equal(t, "1:49.333", line.BestLapTime.Value)
	assert.Equal(t, 26, line.BestLapTime.Lap)
	assert.Equal(t, "1:49.784", line.LastLapTime.Value)
	assert.False(t, line.LastLapTime.OverallFastest)
	assert.True(t, line.LastLapTime.PersonalFastest)
	require.Contains(t, line.Speeds, "ST")
	assert.Equal(t, "312", line.Speeds["ST"].Value)
}

func TestParseTopic_TimingAppData_StintsArrayAndMap(t *testing.T) {
	cases := []struct {
		name     string
		payload  string
		compound string
	}{
		{
			name: "array snapshot uses Stints key",
			payload: `{
				"Lines": {
					"1": {
						"RacingNumber": "1",
						"Line": 1,
						"GridPos": "1",
						"Stints": [
							{"Compound": "MEDIUM", "New": true, "TotalLaps": 12, "StartLaps": 0}
						]
					}
				}
			}`,
			compound: "MEDIUM",
		},
		{
			name: "map feed sparse stint update",
			payload: `{
				"Lines": {
					"1": {
						"Stints": {
							"0": {"Compound": "HARD", "TotalLaps": 5}
						}
					}
				}
			}`,
			compound: "HARD",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseTopic("TimingAppData", json.RawMessage(tc.payload))
			require.NoError(t, err)
			app := out.(*TimingAppData)
			line, ok := app.Lines["1"]
			require.True(t, ok)
			require.NotEmpty(t, line.Stints)
			stint, ok := line.Stints["0"]
			require.True(t, ok)
			assert.Equal(t, tc.compound, stint.Compound)
		})
	}
}

func TestParseTopic_TimingStats_BestSectorsArrayAndMap(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantKey string
		wantVal string
	}{
		{
			name: "array snapshot",
			payload: `{
				"Lines": {
					"4": {
						"RacingNumber": "4",
						"Line": 1,
						"PersonalBestLapTime": {"Value": "1:48.900", "Lap": 10, "Position": 1},
						"BestSectors": [
							{"Value": "27.900", "Position": 1},
							{"Value": "30.100", "Position": 2},
							{"Value": "24.800", "Position": 1}
						],
						"BestSpeeds": {"ST": {"Value": "320", "Position": 3}}
					}
				}
			}`,
			wantKey: "0",
			wantVal: "27.900",
		},
		{
			name: "map feed",
			payload: `{
				"Lines": {
					"4": {
						"BestSectors": {
							"1": {"Value": "30.050", "Position": 1}
						}
					}
				}
			}`,
			wantKey: "1",
			wantVal: "30.050",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseTopic("TimingStats", json.RawMessage(tc.payload))
			require.NoError(t, err)
			stats := out.(*TimingStats)
			line, ok := stats.Lines["4"]
			require.True(t, ok)
			sec, ok := line.BestSectors[tc.wantKey]
			require.True(t, ok)
			assert.Equal(t, tc.wantVal, sec.Value)
		})
	}
}

func TestParseTopic_DriverList_StripsKFAndTeamName(t *testing.T) {
	payload := json.RawMessage(`{
		"_kf": true,
		"1": {
			"RacingNumber": "1",
			"BroadcastName": "M VERSTAPPEN",
			"FullName": "Max VERSTAPPEN",
			"Tla": "VER",
			"Line": 1,
			"TeamName": "Red Bull Racing",
			"TeamColour": "3671C6",
			"FirstName": "Max",
			"LastName": "Verstappen",
			"Reference": "VERSTAPPEN",
			"HeadshotUrl": "https://example.invalid/ver.png"
		}
	}`)

	out, err := ParseTopic("DriverList", payload)
	require.NoError(t, err)
	dl := out.(*DriverList)

	_, hasKF := (*dl)["_kf"]
	assert.False(t, hasKF)

	d, ok := (*dl)["1"]
	require.True(t, ok)
	assert.Equal(t, "VER", d.Tla)
	assert.Equal(t, "Red Bull Racing", d.TeamName)
}

func TestParseTopic_RaceControlMessages_ArrayAndMap(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantKey string
		wantMsg string
	}{
		{
			name: "array completion shape",
			payload: `{
				"Messages": [
					{
						"Utc": "2026-07-19T13:03:52",
						"Lap": 1,
						"Category": "Flag",
						"Flag": "GREEN",
						"Scope": "Track",
						"Message": "GREEN LIGHT - PIT EXIT OPEN"
					}
				]
			}`,
			wantKey: "0",
			wantMsg: "GREEN LIGHT - PIT EXIT OPEN",
		},
		{
			name: "map feed shape",
			payload: `{
				"Messages": {
					"49": {
						"Utc": "2026-07-19T13:03:52",
						"Lap": 1,
						"Category": "Flag",
						"Flag": "GREEN",
						"Scope": "Track",
						"Message": "GREEN LIGHT - PIT EXIT OPEN"
					}
				}
			}`,
			wantKey: "49",
			wantMsg: "GREEN LIGHT - PIT EXIT OPEN",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseTopic("RaceControlMessages", json.RawMessage(tc.payload))
			require.NoError(t, err)
			rcm := out.(*RaceControlMessages)
			msg, ok := rcm.Messages[tc.wantKey]
			require.True(t, ok)
			assert.Equal(t, tc.wantMsg, msg.Message)
		})
	}
}

func TestParseTopic_SessionData_SeriesArrayAndMap(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		check   func(t *testing.T, sd *SessionData)
	}{
		{
			name: "array completion",
			payload: `{
				"Series": [{"Utc": "2026-07-19T13:00:00Z", "Lap": 1}],
				"StatusSeries": [{"Utc": "2026-07-19T13:00:00Z", "TrackStatus": "1"}]
			}`,
			check: func(t *testing.T, sd *SessionData) {
				require.Contains(t, sd.Series, "0")
				assert.Equal(t, 1, sd.Series["0"].Lap)
				require.Contains(t, sd.StatusSeries, "0")
				assert.Equal(t, "1", sd.StatusSeries["0"].TrackStatus)
			},
		},
		{
			name: "map feed with SessionStatus",
			payload: `{
				"StatusSeries": {
					"16": {"Utc": "2026-07-19T13:03:52.073Z", "SessionStatus": "Started"}
				}
			}`,
			check: func(t *testing.T, sd *SessionData) {
				require.Contains(t, sd.StatusSeries, "16")
				assert.Equal(t, "Started", sd.StatusSeries["16"].SessionStatus)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseTopic("SessionData", json.RawMessage(tc.payload))
			require.NoError(t, err)
			tc.check(t, out.(*SessionData))
		})
	}
}

func TestParseTopic_SimpleTopics(t *testing.T) {
	cases := []struct {
		topic   string
		payload string
		check   func(t *testing.T, v any)
	}{
		{
			topic:   "WeatherData",
			payload: `{"AirTemp":"24.1","Humidity":"55","Pressure":"1012.1","Rainfall":"0","TrackTemp":"34.8","WindDirection":"210","WindSpeed":"1.2"}`,
			check: func(t *testing.T, v any) {
				w := v.(*WeatherData)
				assert.Equal(t, "24.1", w.AirTemp)
				assert.Equal(t, "34.8", w.TrackTemp)
			},
		},
		{
			topic:   "TrackStatus",
			payload: `{"Status":"1","Message":"AllClear"}`,
			check: func(t *testing.T, v any) {
				ts := v.(*TrackStatus)
				assert.Equal(t, "1", ts.Status)
				assert.Equal(t, "AllClear", ts.Message)
			},
		},
		{
			topic:   "LapCount",
			payload: `{"CurrentLap":12,"TotalLaps":44}`,
			check: func(t *testing.T, v any) {
				lc := v.(*LapCount)
				assert.Equal(t, 12, lc.CurrentLap)
				assert.Equal(t, 44, lc.TotalLaps)
			},
		},
		{
			topic:   "ExtrapolatedClock",
			payload: `{"Utc":"2026-07-19T13:00:00.000Z","Remaining":"01:23:45","Extrapolating":true}`,
			check: func(t *testing.T, v any) {
				ec := v.(*ExtrapolatedClock)
				assert.Equal(t, "01:23:45", ec.Remaining)
				assert.True(t, ec.Extrapolating)
			},
		},
		{
			topic:   "SessionStatus",
			payload: `{"Status":"Started","Started":"Started"}`,
			check: func(t *testing.T, v any) {
				ss := v.(*SessionStatus)
				assert.Equal(t, "Started", ss.Status)
			},
		},
		{
			topic: "SessionInfo",
			payload: `{
				"Meeting": {
					"Key": 1255,
					"Name": "Belgian Grand Prix",
					"OfficialName": "FORMULA 1 ...",
					"Location": "Spa-Francorchamps",
					"Number": 14,
					"Country": {"Key": 1, "Code": "BEL", "Name": "Belgium"},
					"Circuit": {"Key": 7, "ShortName": "Spa-Francorchamps"}
				},
				"ArchiveStatus": {"Status": "Generating"},
				"Key": 1,
				"Type": "Race",
				"Name": "Race",
				"StartDate": "2026-07-19T15:00:00",
				"EndDate": "2026-07-19T17:00:00",
				"GmtOffset": "02:00:00",
				"Path": "2026/2026-07-19_Belgian_Grand_Prix/2026-07-19_Race/"
			}`,
			check: func(t *testing.T, v any) {
				si := v.(*SessionInfo)
				assert.Equal(t, "Race", si.Type)
				assert.Equal(t, "BEL", si.Meeting.Country.Code)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.topic, func(t *testing.T) {
			out, err := ParseTopic(tc.topic, json.RawMessage(tc.payload))
			require.NoError(t, err)
			tc.check(t, out)
		})
	}
}
