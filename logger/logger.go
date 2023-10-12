package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	_ "github.com/KimMachineGun/automemlimit"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	opentracing "github.com/opentracing/opentracing-go"
)

var (
	Plain        *zap.Logger
	Sugar        *WrappedLogger
	undoLogger   func()
	undoMaxProcs func()
	Recorded     *observer.ObservedLogs
)

const (
	serviceNameKey = "servicename"
	// We repeat this constant here as we don't want the circular dependency
	// of importint our tracing package
	TraceIDKey = "x-b3-traceid"
)

// so we dont have to import zap everywhere
type Option = zap.Option

type WrappedLogger struct {
	*zap.SugaredLogger
}

func (l *WrappedLogger) ErrorR(msg string, args ...any) {
	keyVals := []any{}

	for i, v := range args {
		keyVals = append(keyVals, fmt.Sprintf("arg%d", i))
		keyVals = append(keyVals, v)
	}

	l.WithOptions(zap.AddCallerSkip(1)).Errorw(msg, keyVals...)
}

func (l *WrappedLogger) InfoR(msg string, args ...any) {
	keyVals := []any{}

	for i, v := range args {
		keyVals = append(keyVals, fmt.Sprintf("arg%d", i))
		keyVals = append(keyVals, v)
	}

	l.WithOptions(zap.AddCallerSkip(1)).Infow(msg, keyVals...)
}

func (l *WrappedLogger) DebugR(msg string, args ...any) {
	keyVals := []any{}

	for i, v := range args {
		keyVals = append(keyVals, fmt.Sprintf("arg%d", i))
		keyVals = append(keyVals, v)
	}

	l.WithOptions(zap.AddCallerSkip(1)).Debugw(msg, keyVals...)
}

// OnExit should be deferred immediately after calling the
// New() method.
func OnExit() {
	_ = Sugar.Sync()
	_ = Plain.Sync()
	undoMaxProcs()
	undoLogger()
	Recorded = nil
}

// Resource - the counter is initialised with a zero value which indicates that
// the uber correction is made (default).
type Resource struct {
	console  bool
	filename string
}

type ResourceOption func(*Resource)

func WithFile(filename string) ResourceOption {
	return func(r *Resource) {
		r.filename = filename
	}
}

func WithConsole() ResourceOption {
	return func(r *Resource) {
		r.console = true
	}
}

type Syncer func() error

// New creates 2 loggers (plain and sugared) as global variables according
// to the desired loglevel ("DEBUG", "NOOP", "TEST", default is "INFO").
// Additionally log output from other loggers in 3rd-party packages
// is redirected to the INFO label of these loggers.
// Both ResourceOption and zap.Option types are supported option types. The
// zap.Options are passed on the to zap logger.
func New(level string, opts ...any) {
	r := &Resource{}

	for _, iopt := range opts {
		if opt, ok := iopt.(ResourceOption); ok {
			opt(r)
		}
	}

	var zopts []zap.Option
	for _, opt := range opts {
		if opt, ok := opt.(zap.Option); ok {
			zopts = append(zopts, opt)
		}
	}

	var err error
	// Use opinionated presets for now.
	switch level {
	case "DEBUG":
		cfg := zap.NewDevelopmentConfig()
		if r.filename != "" {
			cfg.OutputPaths = []string{r.filename}
		}
		if r.console {
			cfg.Encoding = "console"
			cfg.EncoderConfig = zapcore.EncoderConfig{
				MessageKey: "message",
			}
		}
		Plain, err = cfg.Build(zopts...)
		if err != nil {
			log.Panicf("cannot initialise zap logger: %v", err)
		}

	case "NOOP":
		Plain = zap.NewNop()

	case "TEST":
		core, recorded := observer.New(zapcore.DebugLevel)

		ram := zap.WrapCore(
			func(zapcore.Core) zapcore.Core {
				return core
			},
		)

		cfg := zap.NewDevelopmentConfig()
		if r.filename != "" {
			cfg.OutputPaths = []string{r.filename}
		}
		if r.console {
			cfg.Encoding = "console"
			cfg.EncoderConfig = zapcore.EncoderConfig{
				MessageKey: "message",
			}
		}
		var plain *zap.Logger
		plain, err = cfg.Build(zopts...)
		if err != nil {
			log.Panicf("cannot initialise zap logger: %v", err)
		}
		Plain = plain.WithOptions(ram)
		Recorded = recorded

	default:
		cfg := zap.NewProductionConfig()
		if r.filename != "" {
			cfg.OutputPaths = []string{r.filename}
		}
		if r.console {
			cfg.Encoding = "console"
			cfg.EncoderConfig = zapcore.EncoderConfig{
				MessageKey: "message",
			}
		}
		Plain, err = cfg.Build(zopts...)
		if err != nil {
			log.Panicf("cannot initialise zap logger: %v", err)
		}
	}
	undoLogger = zap.RedirectStdLog(Plain)
	Sugar = &WrappedLogger{
		Plain.Sugar(),
	}

	Sugar.Debugf("Go version %s", runtime.Version())
	// Set CPU quota correctly so that stalls on non-existent cores do not occur.
	// This must be done as early as possible on task startup - this way all services
	// will have this behaviour as this method is called by everyone..
	//
	// Refs: https://groups.google.com/forum/#%21topic/prometheus-users/QPQ-UbtvS44
	//       https://github.com/golang/go/issues/19378
	//
	// To summarise golang applications in kubernetes suffer from intermittent gc
	// pauses when the golang application thinks it has access to more cores than
	// really available. This results in intermittently high latency when the the
	// gc thread stalls when it cannot access the cores it thinks it has.. Both
	// Uber and Google noticed this and the solution is to set GOMAXPROCS to the
	// number of cores allocated by Kubernetes. This is obtained from the cgroups
	// setting and the logic is encapsulated in the automaxprocs package.
	//
	// At time of writing. archivist has GOMAXPROCS set to 4 but the kubernetes
	// setting is 1. This could result in 75% of CPU cycles being lost when the gc
	// thread is stalled.
	//
	// When load testing is implemented, benchmarks should be run and this code
	// modified/removed and/or kubernetes limits set more cleverly.
	//
	// Please note that the runtime.GOMAXPROCS setting will be removed at some
	// future date.
	//
	// See https://github.com/golang/go/issues/33803 for proposal to make this
	// go away so that automaxprocs is no longer reqyuired.
	var GOMAXPROCS int = runtime.GOMAXPROCS(-1)
	Sugar.Debugf("Cores allocation GOMAXPROCS %v", GOMAXPROCS)
	// modified/removed and/or kubernetes limits set more cleverly.
	undoMaxProcs, err = maxprocs.Set(maxprocs.Logger(Sugar.Infof))
	if err != nil {
		Sugar.Infof("Error for automaxprocs: %v", err)
	}
	Sugar.Debugf("Cores allocation GOMAXPROCS %v", runtime.GOMAXPROCS(-1))

	// If AUTOMEMLIMIT is not set, it defaults to 0.9. (10% is the headroom for memory sources the Go runtime is unaware of.)
	// If GOMEMLIMIT is already set or AUTOMEMLIMIT=off, automatic setting og GMEMLIMIT is disabled.
	var GOMEMLIMIT int64 = debug.SetMemoryLimit(-1)
	Sugar.Debugf("Memory Limit GOMEMLIMIT %v", GOMEMLIMIT)
}

func valueFromCarrier(carrier opentracing.TextMapCarrier, key string) string {
	value, found := carrier[key]
	if !found || value == "" {
		Sugar.Debugf("%s not found", key)
		return ""
	}
	return value
}

// FromContext takes the trace ID from the current span and adds it to a child wrapped logger:
//
// returns:
//   - the new wrapped logger with a context metadata value for traceID
//
// This will be called on entry to a method or a function that has a context.Context.
func (wl *WrappedLogger) FromContext(ctx context.Context) *WrappedLogger {

	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		Sugar.Debugf("FromContext: span is nil")
		return wl
	}
	carrier := opentracing.TextMapCarrier{}
	err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, carrier)
	if err != nil {
		Sugar.Debugf("FromContext: can't inject span: %v", err)
		return wl
	}

	fields := []any{}
	traceID := valueFromCarrier(carrier, TraceIDKey)
	if traceID != "" {
		fields = append(fields, zap.String(TraceIDKey, traceID))
	}

	if len(fields) == 0 {
		return wl
	}
	// add the fields to the logger
	sugaredLogger := wl.With(fields...)

	return &WrappedLogger{
		SugaredLogger: sugaredLogger,
	}
}

func (wl *WrappedLogger) WithServiceName(servicename string) *WrappedLogger {
	return wl.WithIndex(serviceNameKey, servicename)
}

func (wl *WrappedLogger) WithIndex(key, value string) *WrappedLogger {
	return &WrappedLogger{
		SugaredLogger: wl.With(zap.String(key, strings.ToLower(value))),
	}
}

func (wl *WrappedLogger) WithOptions(opts ...Option) *WrappedLogger {
	s := &WrappedLogger{
		Plain.WithOptions(opts...).Sugar(),
	}
	return s
}

// Close attempts to flush any buffered log entries.
func (wl *WrappedLogger) Close() {
	err := wl.Sync()

	// not alot we can do other than log that we couldn't flush the log
	// This is usually an error 'sync /dev/stderr invalid argument'
	// which is pointless
	if err != nil && !errors.Is(err, syscall.EINVAL) {
		wl.Debugf("Close: Failed to flush log: %v", err)
	}
}
