// Package startup is intended as a helper package to
// run services in go routines in main
package startup

import (
	"github.com/datatrails/go-datatrails-common/logger"
)

// RoutineFatalOnError runs a function in go routine and fatals when the function errors
func RoutineFatalOnError(serviceStart func() error) {
	go func() {
		err := serviceStart()
		if err != nil {
			logger.Sugar.Panicf("service failed with an error: %v", err)
		}
		logger.Sugar.Info("service terminated")
	}()
}
