package log

import (
	"io"

	"github.com/scout-inc/scout-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
)

var (
	LogSeverityKey = attribute.Key(scout.LogSeverityAttribute)
	LogMessageKey  = attribute.Key(scout.LogMessageAttribute)
)

// Init configures logrus to ship logs to Scout
func Init() {
	logrus.SetReportCaller(true)
	logrus.AddHook(NewHook(WithLevels(logrus.AllLevels...)))
}

// DisableOutput turns off stdout / stderr output from logrus, in case another logger is already used.
func DisableOutput() {
	logrus.SetOutput(io.Discard)
}
