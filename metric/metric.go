package metric

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/scout-inc/scout-go"
	"go.opentelemetry.io/otel/attribute"
)

func shouldRecordMetric(rate float64) bool {
	return rand.Float64() <= math.Min(rate, scout.GetMetricSamplingRate())
}

// Histogram tracks the statistical distribution of a set of values for an event.
//
// Example:
//
// metric.Histogram(ctx, "logins.successful", 1.0, []{Key: "email", Value: "x@example.com"}, 1)
func Histogram(ctx context.Context, name string, value float64, tags []attribute.KeyValue, rate float64) {
	if !shouldRecordMetric(rate) {
		return
	}
	scout.RecordMetric(ctx, name, value, tags...)
}

// Timing records duration information for an event (in seconds).
//
// Example:
//
// duration := time.Millisecond * 30
//
//	tags = []attribute.KeyValue{
//		{
//			Key:   attribute.Key("table"),
//			Value: attribute.StringValue("users"),
//		},
//	}
//	metric.Timing(ctx, "queries.select", duration, tags, 1)
func Timing(ctx context.Context, name string, value time.Duration, tags []attribute.KeyValue, rate float64) {
	if !shouldRecordMetric(rate) {
		return
	}
	scout.RecordMetric(ctx, name, value.Seconds(), tags...)
}

// Increment records a new metric instance with a value of 1.
// Example (to increment the new_users counter -- i.e to record a new instance of new_user):
//
// metric.Increment(ctx, "new_users", nil, 1)
func Increment(ctx context.Context, name string, tags []attribute.KeyValue, rate float64) {
	if !shouldRecordMetric(rate) {
		return
	}
	scout.RecordMetric(ctx, name, 1, tags...)
}
