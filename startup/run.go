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

type Runner func(string, logger.Logger) error

// defers do not work in main() because of the os.Exit(
func Run(serviceName string, run Runner) {
	logger.New(environment.GetLogLevel())
	log := logger.Sugar.WithServiceName(serviceName)

	// ensure we configure go max procs and memlimit
	//  for kubernetes.
	k8Config, err := k8sworker.NewK8Config(k8sworker.WithLogger(log.Infof))
	if err != nil {
		log.Infof("Error configuring go for kubernetes: %v", err)
		os.Exit(1)
	}

	// log the useful kubernetes go configuration
	log.Infof("Go Configuration: %+v", k8Config)

	exitCode := func() int {
		var exitCode int
		closer := tracing.NewTracer()
		if closer != nil {
			defer closer.Close()
		}
		err := run(serviceName, log)
		if err != nil {
			log.Infof("Error at startup: %v", err)
			exitCode = 1
		}
		return exitCode
	}()

	log.Infof("Shutting down gracefully")
	logger.OnExit()

	// ensure we reset go configuration back to normal
	k8sworker.Close()

	os.Exit(exitCode)

}
