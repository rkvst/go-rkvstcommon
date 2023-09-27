// Package tracing is responsible for forwarding and translating span headers for internal requests
package tracing

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	grpc_otrace "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	otnethttp "github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
)

const (
	requestID         = "x-request-id"
	otSpanContext     = "x-ot-span-context"
	prefixTracerState = "x-b3-"
	TraceID           = prefixTracerState + "traceid"
	spanID            = prefixTracerState + "spanid"
	parentSpanID      = prefixTracerState + "parentspanid"
	sampled           = prefixTracerState + "sampled"
	flags             = prefixTracerState + "flags"
)

var otHeaders = []string{
	requestID,
	otSpanContext,
	prefixTracerState,
	TraceID,
	spanID,
	parentSpanID,
	sampled,
	flags,
}

func HTTPMiddleware(h http.Handler) http.Handler {
	return otnethttp.Middleware(
		opentracing.GlobalTracer(),
		h,
		otnethttp.OperationNameFunc(func(r *http.Request) string {
			return "HTTP " + r.Method + ":" + r.URL.EscapedPath() + " >"
		}),
	)
}

// GRPCDialTracingOptions returns DialOption enabling open tracing for grpc connections
func GRPCDialTracingOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStreamInterceptor(
			grpc_otrace.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(
			grpc_otrace.UnaryClientInterceptor()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
}

// HeaderMatcher ensures that open tracing headers x-b3-* are forwarded to output requests
func HeaderMatcher(key string) (string, bool) {
	key = textproto.CanonicalMIMEHeaderKey(key)
	for _, tracingKey := range otHeaders {
		if strings.ToLower(key) == tracingKey {
			return key, true
		}
	}
	return "", false
}

func trimPodName(p string) string {
	a := strings.Split(p, "-")
	i := len(a)
	if i > 2 {
		return strings.Join(a[:i-2], "-")
	}
	if i > 1 {
		return strings.Join(a[:i-1], "-")
	}
	return p
}

func NewTracer() io.Closer {
	instanceName, _, _ := strings.Cut(environment.GetOrFatal("POD_NAME"), " ") 
	nameSpace := environment.GetOrFatal("POD_NAMESPACE")
	containerName := environment.GetOrFatal("CONTAINER_NAME")
	podName := strings.Join([]string{trimPodName(instanceName), nameSpace, containerName}, ".")
	listenStr := fmt.Sprintf("localhost:%s", environment.GetOrFatal("PORT"))
        return NewFromEnv(strings.TrimSpace(podName), listenStr, "ZIPKIN_ENDPOINT", "DISABLE_ZIPKIN");
}

// NewFromEnv initialises tracing and returns a closer if tracing is
// configured.  If the necessary configuration is not available it is Fatal
// unless disableVar is set and is truthy (strconf.ParseBool -> true). If
// tracing is disabled returns nil
func NewFromEnv(service string, host string, endpointVar, disableVar string) io.Closer {
	ze, ok := os.LookupEnv(endpointVar)
	if !ok {
		if disabled := environment.GetTruthyOrFatal(disableVar); !disabled {
			logger.Sugar.Panicf(
				"'%s' has not been provided and is not disabled by '%s'",
				endpointVar, disableVar)
		}
		logger.Sugar.Infof("zipkin disabled by '%s'", disableVar)
		return nil
	}
	// zipkin conf is available, disable it if disableVar is truthy

	if disabled := environment.GetTruthyOrFatal(disableVar); disabled {
		logger.Sugar.Infof("'%s' set, zipkin disabled", disableVar)
		return nil
	}
	return New(service, host, ze)
}

// New initialises tracing
// uses zipkin client tracer
func New(service string, host string, zipkinEndpoint string) io.Closer {
	// create our local service endpoint
	localEndpoint, err := zipkin.NewEndpoint(service, host)
	if err != nil {
		logger.Sugar.Panicf("unable to create zipkin local endpoint service '%s' - host '%s': %v", service, host, err)

	}

	// set up a span reporter
	zipkinLogger := log.New(os.Stdout, "zipkin", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	reporter := zipkinhttp.NewReporter(zipkinEndpoint, zipkinhttp.Logger(zipkinLogger))

	// initialise our tracer
	nativeTracer, err := zipkin.NewTracer(
		reporter,
		zipkin.WithLocalEndpoint(localEndpoint),
		zipkin.WithSharedSpans(false),
	)
	if err != nil {
		logger.Sugar.Panicf("unable to create zipkin tracer: %v", err)
	}

	// use zipkin-go-opentracing to wrap our tracer
	tracer := zipkinot.Wrap(nativeTracer)
	opentracing.SetGlobalTracer(tracer)

	//	logger.Plain.Core().With(zap.String("service", cfg.ServiceName),)

	return reporter
}
