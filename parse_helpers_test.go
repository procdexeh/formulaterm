package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	maxScanLine = 4 << 20
	rawDumpPath = "./testdata/spa_raw_race_dump.json"
)

func openScanner(t *testing.T, path string) (*bufio.Scanner, func()) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err, "open %s", path)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxScanLine)

	return sc, func() { _ = f.Close() }
}

func readNDJSONLines(t *testing.T, path string) [][]byte {
	t.Helper()

	sc, close := openScanner(t, path)
	defer close()

	var lines [][]byte
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, cp)
	}
	require.NoError(t, sc.Err(), "scan %s", path)
	require.NotEmpty(t, lines, "no lines in %s", path)
	return lines
}

func globNDJSON(t *testing.T, pattern string) []string {
	t.Helper()

	matches, err := filepath.Glob(pattern)
	require.NoError(t, err, "glob %s", pattern)
	require.NotEmpty(t, matches, "no files match %s", pattern)
	return matches
}

func mustUnmarshalMessage(t *testing.T, raw []byte) LiveTimingMessage {
	t.Helper()

	var msg LiveTimingMessage
	require.NoError(t, json.Unmarshal(raw, &msg), "envelope: %s", snippet(raw))
	return msg
}

func snippet(raw []byte) string {
	const n = 180
	raw = bytes.TrimSpace(raw)
	if len(raw) <= n {
		return string(raw)
	}
	return string(raw[:n]) + "…"
}

// feedArgs extracts topic + payload from a type-1 "feed" invocation.
func feedArgs(t *testing.T, msg LiveTimingMessage) (topic string, payload json.RawMessage) {
	t.Helper()

	require.Equal(t, SIGNARLR_MSG_INVOCATION, msg.Type, "expected invocation message")
	require.Equal(t, "feed", msg.Target, "expected feed target")
	require.GreaterOrEqual(t, len(msg.Arguments), 2, "feed arguments: want >= 2, got %d", len(msg.Arguments))

	require.NoError(t, json.Unmarshal(msg.Arguments[0], &topic), "feed topic arg")
	require.NotEmpty(t, topic, "empty feed topic")
	payload = msg.Arguments[1]
	require.NotEmpty(t, payload, "empty feed payload for topic %s", topic)
	return topic, payload
}

func completionTopics(t *testing.T, msg LiveTimingMessage) map[string]json.RawMessage {
	t.Helper()

	require.Equal(t, SIGNARLR_MSG_COMPLETION, msg.Type, "expected completion message")
	require.NotEmpty(t, msg.Result, "completion result empty")
	return msg.Result
}

func parseFeedLine(t *testing.T, raw []byte) (topic string, parsed any) {
	t.Helper()

	msg := mustUnmarshalMessage(t, raw)
	topic, payload := feedArgs(t, msg)
	parsed, err := ParseTopic(topic, payload)
	require.NoError(t, err, "ParseTopic(%q) payload=%s", topic, snippet(payload))
	require.NotNil(t, parsed, "ParseTopic(%q) returned nil", topic)
	return topic, parsed
}

func parseCompletionLine(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	msg := mustUnmarshalMessage(t, raw)
	topics := completionTopics(t, msg)
	out := make(map[string]any, len(topics))
	for name, payload := range topics {
		parsed, err := ParseTopic(name, payload)
		require.NoError(t, err, "ParseTopic(%q) payload=%s", name, snippet(payload))
		require.NotNil(t, parsed, "ParseTopic(%q) returned nil", name)
		out[name] = parsed
	}
	return out
}
