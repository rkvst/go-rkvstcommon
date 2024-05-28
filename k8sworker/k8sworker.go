package k8sworker

import (
	"log/slog"
	"runtime"
	"runtime/debug"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"go.uber.org/automaxprocs/maxprocs"
)

var (
	undoMaxProcs func()
)

// K8sConfig sets the cpu and memory
//
//	go configuration for kubernetes.
type K8sConfig struct {
	GoMaxProcs int

	GoMemLimit int64

	GoVersion string
}

func NewK8Config(opts ...K8sOption) (*K8sConfig, error) {

	options := ParseOptions(opts...)

	k8Config := K8sConfig{
		GoVersion: runtime.Version(),
	}

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
	k8Config.GoMaxProcs = runtime.GOMAXPROCS(-1)

	// modified/removed and/or kubernetes limits set more cleverly.
	var err error
	if options.logger != nil {

		memlimit.SetGoMemLimitWithOpts(
			memlimit.WithRatio(0.9),
			memlimit.WithProvider(memlimit.FromCgroup),
			memlimit.WithLogger(slog.Default()),
		)

		undoMaxProcs, err = maxprocs.Set(maxprocs.Logger(options.logger))
	} else {

		memlimit.SetGoMemLimitWithOpts(
			memlimit.WithRatio(0.9),
			memlimit.WithProvider(memlimit.FromCgroup),
		)

		undoMaxProcs, err = maxprocs.Set()
	}

	if err != nil {
		return nil, err
	}

	// If AUTOMEMLIMIT is not set, it defaults to 0.9. (10% is the headroom for memory sources the Go runtime is unaware of.)
	// If GOMEMLIMIT is already set or AUTOMEMLIMIT=off, automatic setting og GMEMLIMIT is disabled.
	k8Config.GoMemLimit = debug.SetMemoryLimit(-1)

	return &k8Config, nil
}

// Close undoes any changes to GoMaxProcs.
func Close() {
	undoMaxProcs()
}
