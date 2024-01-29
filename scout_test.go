package scout

import (
	"context"
	"testing"

	"github.com/aws/smithy-go/ptr"
	"github.com/pkg/errors"
)

// TestConsumeError tests every case for RecordMetric
func TestRecordMetric(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeys.SessionSecureID, "0")
	ctx = context.WithValue(ctx, ContextKeys.RequestID, "0")
	tests := map[string]struct {
		metricInput struct {
			name  string
			value float64
		}
		contextInput      context.Context
		expectedFlushSize int
	}{
		"test": {expectedFlushSize: 1, contextInput: ctx, metricInput: struct {
			name  string
			value float64
		}{name: "myMetric", value: 123.456}},
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			Init()
			RecordMetric(input.contextInput, input.metricInput.name, input.metricInput.value)
		})
	}
	Stop()
}

func TestScopedKey(t *testing.T) {
	tests := []struct {
		key       string
		expected  string
		separator *string
	}{
		{"a", "scout.a", nil},
		{"chi", "scout.chi", nil},
		{"Tracer", "scout.Tracer", nil},
		{"a", "scout-a", ptr.String("-")},
		{"chi", "scout-chi", ptr.String("-")},
		{"Tracer", "scout-Tracer", ptr.String("-")},
	}

	for _, tt := range tests {
		result := ScopedKey(tt.key, tt.separator)
		if result != tt.expected {
			t.Fatalf("[ScopedKey] expected ScopedKey(%s, %s) to be %s, got %s", tt.key, *tt.separator, tt.expected, result)
		}
	}
}

func TestExtractIdsFromRequest(t *testing.T) {
	tests := []struct {
		requestDetails  string
		sessionSecureId string
		requestId       string
		err             error
	}{
		{"abc/def", "abc", "def", nil},
		{"1/2", "1", "2", nil},
		{"qwerty/uiop", "qwerty", "uiop", nil},
		{"qwerty", "", "", errors.New("request does not contain tracer IDs")},
	}

	for _, tt := range tests {
		sessionSecureId, requestId, err := ExtractIdsFromRequest(tt.requestDetails)

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		ttErrMsg := ""
		if err != nil {
			ttErrMsg = tt.err.Error()
		}

		if errMsg != ttErrMsg {
			t.Fatalf("[ExtractIdsFromRequest] expected ExtractIdsFromRequest(%s) to be (%s, %s, %v), got (%s, %s, %v)", tt.requestDetails, tt.sessionSecureId, tt.requestId, tt.err, sessionSecureId, requestId, err)
		}

	}
}
