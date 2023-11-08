package environment

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
)

const (
	commaSeparator = ","
)

// GetLogLevel returns the loglevet or panics. This is called before any logger
// is available. i.e. don't use a logger here.
func GetLogLevel() string {
	value, ok := os.LookupEnv("LOGLEVEL")
	if !ok {
		panic(errors.New("No loglevel specified"))
	}
	return value
}

// GetOrFatal returns the key's value or logs a Fatal error (and exits)
func GetOrFatal(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		logger.Sugar.Panicf("required environment variable is not defined: %s", key)
	}
	return value
}

// GetIntOrFatal returns value of environment variable that is
// expected to be an int, otherwise logs a Fatal error (and exits)
func GetIntOrFatal(key string) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		logger.Sugar.Panicf("required environment variable is not defined or: %s", key)
	}
	value, err := strconv.Atoi(val)
	if err != nil {
		logger.Sugar.Panicf("unable to convert %s value to int: %v", key, err)
	}
	return value
}

// GetRequired gets the value for the key, or an error if it is not set.
func GetRequired(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("required environment variable '%s' is not defined", key)
	}
	return value, nil
}

// GetTruthyOrFatal returns true if key is set to a value that is truthy. Returns
// false otherwise.
func GetTruthyOrFatal(key string) bool {
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

// GetListOrFatal returns the key's value as a list or logs a Fatal error (and exits)
//
//	The value is expected to be a csv
//
// NOTE: if the value is not csv, it is returned as is in a list with the original string
//
//	as the only element in the list
func GetListOrFatal(key string) []string {
	const commaSeparator = ","
	if value, ok := os.LookupEnv(key); ok {
		values := strings.Split(value, commaSeparator)
		return values
	}
	logger.Sugar.Panicf("required environment variable is not defined: %s", key)
	return []string{} // never reaches here
}

// ReadIndirectOrFatal reads filename and uses it to read a value from the file.
// Any error is Fatal.
func ReadIndirectOrFatal(varname string) string {
	filename, ok := os.LookupEnv(varname)
	if !ok {
		logger.Sugar.Panicf("environment variable `%s' not present in env", varname)
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		logger.Sugar.Panicf("error reading file `%s': %s", filename, err)
	}
	return string(b)
}
