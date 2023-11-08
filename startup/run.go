// Package startup is intended as a helper package to
// run services in go routines in main
package startup

import (
	"os"

	"github.com/datatrails/go-datatrails-common/environment"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-common/tracing"
)

type Runner func(string, *logger.WrappedLogger) error

// defers do not work in main() because of the os.Exit(
func Run(serviceName string, run Runner) {
	logger.New(environment.GetLogLevel())
	log := logger.Sugar.WithServiceName(serviceName)

	exitCode := func() int {
		var exitCode int
		closer := tracing.NewTracer()
		if closer != nil {
			defer closer.Close()
		}
		err := run(serviceName, log)
		if err != nil {
			log.Infof("Error terminating: %v", err)
			exitCode = 1
		}
		return exitCode
	}()

	log.Infof("Shutting down gracefully")
	logger.OnExit()
	os.Exit(exitCode)

}
