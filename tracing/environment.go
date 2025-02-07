package tracing

import (
	"os"
	"strconv"

	"github.com/datatrails/go-datatrails-common/logger"
)

const (
	commaSeparator = ","
)

func getOrFatal(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		logger.Sugar.Panicf("required environment variable is not defined: %s", key)
	}
	return value
}

func getTruthyOrFatal(key string) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		logger.Sugar.Panicf("environment variable %s not found", key)
	}
	// t,true,True,1 are all examples of 'truthy' values understood by ParseBool
	b, err := strconv.ParseBool(value)
	if err != nil {
		logger.Sugar.Panicf("environment variable %s not valid truthy value: %v", key, err)
	}
	return b
}
