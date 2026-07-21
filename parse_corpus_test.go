package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorpusRawDump_ParseAllFeedAndCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("full session dump; run without -short")
	}
	if _, err := os.Stat(rawDumpPath); err != nil {
		t.Skipf("dump %s not present: %v", rawDumpPath, err)
	}

	sc, close := openScanner(t, rawDumpPath)
	defer close()

	var (
		lineNo      int
		nCompletion int
		nFeed       int
		nPing       int
		nOther      int
		nSkippedPos int
		byTopic     = map[string]int{}
	)

	for sc.Scan() {
		lineNo++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}

		var msg LiveTimingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("line %d: envelope unmarshal: %v\n%s", lineNo, err, snippet(raw))
		}

		switch msg.Type {
		case SIGNARLR_MSG_PING, SIGNARLR_MSG_CLOSE:
			nPing++
			continue

		case SIGNARLR_MSG_COMPLETION:
			nCompletion++
			if len(msg.Result) == 0 {
				// Empty completion is a known edge; count but do not fail corpus.
				continue
			}
			for topic, payload := range msg.Result {
				if topic == "Position.z" {
					nSkippedPos++
					continue
				}
				if _, err := ParseTopic(topic, payload); err != nil {
					t.Fatalf("line %d completion ParseTopic(%q): %v\npayload=%s", lineNo, topic, err, snippet(payload))
				}
				byTopic["completion:"+topic]++
			}

		case SIGNARLR_MSG_INVOCATION:
			if msg.Target != "feed" {
				nOther++
				continue
			}
			nFeed++
			if len(msg.Arguments) < 2 {
				t.Fatalf("line %d: feed arguments short: %s", lineNo, snippet(raw))
			}
			var topic string
			if err := json.Unmarshal(msg.Arguments[0], &topic); err != nil {
				t.Fatalf("line %d: feed topic: %v", lineNo, err)
			}
			if topic == "Position.z" {
				nSkippedPos++
				continue
			}
			payload := msg.Arguments[1]
			if _, err := ParseTopic(topic, payload); err != nil {
				t.Fatalf("line %d feed ParseTopic(%q): %v\npayload=%s", lineNo, topic, err, snippet(payload))
			}
			byTopic[topic]++

		default:
			nOther++
		}
	}
	require.NoError(t, sc.Err())
	require.Greater(t, lineNo, 0)

	t.Logf("corpus lines=%d completion=%d feed=%d ping/close=%d other=%d skipped_position_z=%d",
		lineNo, nCompletion, nFeed, nPing, nOther, nSkippedPos)
	t.Logf("topics: %s", formatCounts(byTopic))
}

func formatCounts(m map[string]int) string {
	// stable-ish small summary
	type kv struct {
		k string
		v int
	}
	var items []kv
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	// simple insertion by value desc without importing sort for tiny maps is fine with sort
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].v > items[i].v || (items[j].v == items[i].v && items[j].k < items[i].k) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	out := ""
	for i, it := range items {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s=%d", it.k, it.v)
	}
	return out
}
