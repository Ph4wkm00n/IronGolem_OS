package signal

import (
	"testing"
	"time"
)

func TestNormalizeLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		wantText string
		wantMeta map[string]string
	}{
		{
			name:     "plain data message",
			line:     `{"envelope":{"source":"+15551234567","sourceNumber":"+15551234567","sourceName":"Alice","timestamp":1712345678901,"dataMessage":{"timestamp":1712345678901,"message":"hello there"}},"account":"+15559876543"}`,
			wantOK:   true,
			wantText: "hello there",
			wantMeta: map[string]string{
				"recipient":   "+15551234567",
				"source":      "+15551234567",
				"source_name": "Alice",
			},
		},
		{
			name:     "group message carries group_id",
			line:     `{"envelope":{"sourceNumber":"+15551234567","timestamp":1712345678901,"dataMessage":{"timestamp":1712345678901,"message":"group hi","groupInfo":{"groupId":"abc123=="}}}}`,
			wantOK:   true,
			wantText: "group hi",
			wantMeta: map[string]string{
				"recipient": "+15551234567",
				"source":    "+15551234567",
				"group_id":  "abc123==",
			},
		},
		{
			name:   "receipt without dataMessage dropped",
			line:   `{"envelope":{"source":"+15551234567","timestamp":1712345678901,"receiptMessage":{"when":1712345678901,"isDelivery":true}}}`,
			wantOK: false,
		},
		{
			name:   "typing indicator dropped",
			line:   `{"envelope":{"source":"+15551234567","timestamp":1712345678901,"typingMessage":{"action":"STARTED"}}}`,
			wantOK: false,
		},
		{
			name:   "empty message body dropped",
			line:   `{"envelope":{"source":"+15551234567","timestamp":1712345678901,"dataMessage":{"timestamp":1712345678901,"message":""}}}`,
			wantOK: false,
		},
		{
			name:   "missing source dropped",
			line:   `{"envelope":{"timestamp":1712345678901,"dataMessage":{"timestamp":1712345678901,"message":"orphan"}}}`,
			wantOK: false,
		},
		{
			name:   "malformed JSON dropped without panic",
			line:   `{"envelope":{"source":"+1555`,
			wantOK: false,
		},
		{
			name:   "non-JSON garbage dropped without panic",
			line:   `signal-cli: WARN some log line that leaked to stdout`,
			wantOK: false,
		},
		{
			name:   "empty line dropped",
			line:   ``,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := normalizeLine([]byte(tt.line))
			if ok != tt.wantOK {
				t.Fatalf("normalizeLine ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if msg.Content != tt.wantText {
				t.Errorf("Content = %q, want %q", msg.Content, tt.wantText)
			}
			if msg.Direction != "inbound" && string(msg.Direction) != "inbound" {
				t.Errorf("Direction = %v, want inbound", msg.Direction)
			}
			for k, want := range tt.wantMeta {
				if got := msg.Metadata[k]; got != want {
					t.Errorf("Metadata[%q] = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestNormalizeLineTimestamp(t *testing.T) {
	line := `{"envelope":{"sourceNumber":"+15551234567","timestamp":1712345678901,"dataMessage":{"timestamp":1712345678901,"message":"ts check"}}}`
	msg, ok := normalizeLine([]byte(line))
	if !ok {
		t.Fatal("expected message")
	}
	want := time.UnixMilli(1712345678901).UTC()
	if !msg.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, want)
	}
}

func TestNormalizeLineZeroTimestampFallsBack(t *testing.T) {
	line := `{"envelope":{"sourceNumber":"+15551234567","dataMessage":{"message":"no ts"}}}`
	before := time.Now().UTC().Add(-time.Minute)
	msg, ok := normalizeLine([]byte(line))
	if !ok {
		t.Fatal("expected message")
	}
	if msg.Timestamp.Before(before) {
		t.Errorf("zero timestamp should fall back to ~now, got %v", msg.Timestamp)
	}
}
