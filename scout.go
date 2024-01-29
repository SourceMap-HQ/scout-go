package scout

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"go.opentelemetry.io/otel/trace"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type config struct {
	otelEndpoint       string
	projectID          string
	resourceAttributes []attribute.KeyValue
	metricSamplingRate float64
	samplingRateMap    map[trace.SpanKind]float64
}

var (
	interruptChan chan bool
	signalChan    chan os.Signal
	conf          = &config{
		otelEndpoint:       OTLPDefaultEndpoint,
		metricSamplingRate: 1.,
		samplingRateMap: map[trace.SpanKind]float64{
			trace.SpanKindUnspecified: 1.,
		},
	}
)

type Option interface {
	apply(conf *config)
}

type option func(conf *config)

func (fn option) apply(conf *config) {
	fn(conf)
}

func WithProjectID(projectID string) Option {
	return option(func(conf *config) {
		conf.projectID = projectID
	})
}

func WithMetricSamplingRate(samplingRate float64) Option {
	return option(func(conf *config) {
		conf.metricSamplingRate = samplingRate
	})
}

func WithSamplingRate(samplingRate float64) Option {
	return option(func(conf *config) {
		conf.samplingRateMap = map[trace.SpanKind]float64{
			trace.SpanKindUnspecified: samplingRate,
		}
	})
}

func WithSamplingRateMap(rates map[trace.SpanKind]float64) Option {
	return option(func(conf *config) {
		conf.samplingRateMap = rates
	})
}

func WithServiceName(serviceName string) Option {
	return option(func(conf *config) {
		attr := semconv.ServiceNameKey.String(serviceName)
		conf.resourceAttributes = append(conf.resourceAttributes, attr)
	})
}

func WithServiceVersion(serviceVersion string) Option {
	return option(func(conf *config) {
		attr := semconv.ServiceVersionKey.String(serviceVersion)
		conf.resourceAttributes = append(conf.resourceAttributes, attr)
	})
}

func WithEnvironment(environment string) Option {
	return option(func(conf *config) {
		attr := semconv.DeploymentEnvironmentKey.String(environment)
		conf.resourceAttributes = append(conf.resourceAttributes, attr)
	})
}

// type contextKey refers to attribute keys that Scout stores in the tracker's context
type contextKey string

const (
	Scout           contextKey = "scout"
	RequestID                  = Scout + "RequestID"
	SessionSecureID            = Scout + "SessionSecureID"
)

func ScopedKey(key string, separator *string) string {
	sep := "."
	if separator != nil {
		sep = *separator
	}
	return fmt.Sprintf("%s%s%s", Scout, sep, key)
}

var (
	ContextKeys = struct {
		RequestID       contextKey
		SessionSecureID contextKey
	}{
		RequestID:       RequestID,
		SessionSecureID: SessionSecureID,
	}
)

// appState is used for keeping track of the current state of the app
// it's used to determine whether to accept new errors
// appState tracks whether the app is running, stopped or idle
type appState byte

const (
	idle appState = iota
	started
	stopped
)

var (
	state      appState
	stateMutex sync.RWMutex
	otlp       *OTLP
)

const (
	consumeErrorWorkerStopped = "scout worker stopped"
)

// Logger is an interface that implements Log and Logf
type Logger interface {
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

var logger struct {
	Logger
}

// noop default logger
type deadLog struct{}

func (d deadLog) Error(_ ...interface{})            {}
func (d deadLog) Errorf(_ string, _ ...interface{}) {}

func init() {
	interruptChan = make(chan bool, 1)
	signalChan = make(chan os.Signal, 1)
	conf = &config{}

	signal.Notify(signalChan, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGINT)
	SetOtelEndpoint(OTLPDefaultEndpoint)
	SetDebugMode(deadLog{})
}

// Initialise telemetry collector
func Init(opts ...Option) {
	StartWithContext(context.Background(), opts...)
}

// StartWithContext is used to start Scout's telemetry collection service, but allows the user to pass in their own context.Context.
// This allows the user kill the Scout worker by invoking context.CancelFunc.
func StartWithContext(ctx context.Context, opts ...Option) {
	for _, opt := range opts {
		opt.apply(conf)
	}
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if state == started {
		return
	}
	var err error
	otlp, err = StartOTLP()
	if err != nil {
		logger.Errorf("failed to start opentelemetry exporter: %s", err)
	}
	state = started
	go func() {
		for {
			select {
			case <-interruptChan:
				return
			case <-signalChan:
				shutdown()
				return
			case <-ctx.Done():
				shutdown()
				return
			}
		}
	}()
}

// Start readies Scout to start collecting telemetry.
func Start(opts ...Option) {
	StartWithContext(context.Background(), opts...)
}

// Flush buffers and stop collecting telemetry
func Stop() {
	interruptChan <- true
	shutdown()
}

func IsRunning() bool {
	return state == started
}

// SetOtelEndpoint allows you to override the otlp address used for sending errors and traces.
// Use the root http url. Eg: https://otel.scout.us:4318
func SetOtelEndpoint(newotelEndpoint string) {
	conf.otelEndpoint = newotelEndpoint
}

func SetDebugMode(l Logger) {
	logger.Logger = l
}

func SetProjectID(id string) {
	conf.projectID = id
}

func GetProjectID() string {
	return conf.projectID
}

func GetMetricSamplingRate() float64 {
	return conf.metricSamplingRate
}

// InterceptRequest calls InterceptRequestWithContext using the request object's context
func InterceptRequest(r *http.Request) context.Context {
	return InterceptRequestWithContext(r.Context(), r)
}

// InterceptRequestWithContext captures the and request ID
// for a particular request from the request headers, adding the values to the provided context.
func InterceptRequestWithContext(ctx context.Context, r *http.Request) context.Context {
	header := r.Header.Get("X-Scout-Request")
	ids := strings.Split(header, "/")
	if len(ids) < 2 {
		return ctx
	}
	ctx = context.WithValue(ctx, ContextKeys.SessionSecureID, ids[0])
	ctx = context.WithValue(ctx, ContextKeys.RequestID, ids[1])
	return ctx
}

func validateRequest(ctx context.Context) (sessionSecureID string, requestID string, err error) {
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	if state == stopped {
		err = errors.New(consumeErrorWorkerStopped)
		return
	}
	if v := ctx.Value(string(ContextKeys.SessionSecureID)); v != nil {
		sessionSecureID = v.(string)
	}
	if v := ctx.Value(ContextKeys.SessionSecureID); v != nil {
		sessionSecureID = v.(string)
	}
	if v := ctx.Value(string(ContextKeys.RequestID)); v != nil {
		requestID = v.(string)
	}
	if v := ctx.Value(ContextKeys.RequestID); v != nil {
		requestID = v.(string)
	}
	return
}

func shutdown() {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	if !IsRunning() {
		return
	}
	if otlp != nil {
		otlp.shutdown()
	}
	state = stopped
}
