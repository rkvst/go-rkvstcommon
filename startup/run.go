// Package startup is intended as a helper package to
// run services in go routines in main
package startup

import (
	"os"

	"github.com/rkvst/go-rkvstcommon/environment"
	"github.com/rkvst/go-rkvstcommon/logger"
)

type Runner func(string, *logger.WrappedLogger) error

// defers do not work in main() because of the os.Exit(
func Run(serviceName string, run Runner) {
	var exitCode int
	logger.New(environment.GetLogLevel())
	log := logger.Sugar.WithServiceName(serviceName)

	err := run(serviceName, log)
	if err != nil {
		log.Infof("Error terminating: %v", err)
		exitCode = 1
	}

	log.Infof("Shutting down gracefully")
	logger.OnExit()
	os.Exit(exitCode)

}
