// Package startup is intended as a helper package to
// run services in go routines in main
package startup

import (
	"os"

	"github.com/datatrails/go-datatrails-common/environment"
	"github.com/datatrails/go-datatrails-common/k8sworker"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-common/tracing"
)

const (
	restProxyPort = "RESTPROXY_PORT"
	port          = "PORT"
)

type runOptions struct {
	portName string
}

type RunOption func(*runOptions)

func UseRestproxyTracingPort() RunOption {
	return func(o *runOptions) {
		o.portName = restProxyPort
	}
}

func NoTracingPort() RunOption {
	return func(o *runOptions) {
		o.portName = ""
	}
}

type Runner func(logger.Logger) error

// defers do not work in main() because of the os.Exit(
func Run(serviceName string, run Runner, opts ...RunOption) {

	logger.New(environment.GetLogLevel())
	log := logger.Sugar.WithServiceName(serviceName)

	o := runOptions{portName: port}

	for _, opt := range opts {
		opt(&o)
	}

	exitCode := func() int {
		var exitCode int
		var err error
		// ensure we configure go max procs and memlimit
		//  for kubernetes.
		k8Config, err := k8sworker.NewK8Config(k8sworker.WithLogger(log.Infof))
		if err != nil {
			log.Infof("Error configuring go for kubernetes: %v", err)
			return 1
		}
		defer k8sworker.Close()

		// log the useful kubernetes go configuration
		log.Infof("Go Configuration: %+v", k8Config)

		if o.portName != "" {
			closer := tracing.NewTracer(log, o.portName)
			if closer != nil {
				defer closer.Close()
			}
		}
		err = run(log)
		if err != nil {
			log.Infof("Error at startup: %v", err)
			exitCode = 1
		}
		return exitCode
	}()

	log.Infof("Shutting down")
	logger.OnExit()

	os.Exit(exitCode)

}
