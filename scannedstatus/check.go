package scannedstatus

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/datatrails/go-datatrails-common/errhandling"
	"github.com/datatrails/go-datatrails-common/logger"
)

const (
	Key       = "scanned_status"
	BadReason = "scanned_bad_reason"
	Timestamp = "scanned_status_timestamp"

	allowedBadQueryParamName  = "allow_insecure"
	propmtForPendingParamName = "strict"
	allowedQueryParamValue    = "true"
)

// common code to construct a returned error
func newError(msg string, args ...any) error {
	return errors.New(
		errhandling.JSONWithHTTPStatus(
			http.StatusBadRequest,
			fmt.Sprintf(msg, args...),
		),
	)
}

func Check(scannedStatus string, query url.Values) error {

	logger.Sugar.Debugf("Scanned status Check %s", scannedStatus)

	var err error
	strictValidation := false
	allowInsecure := false
	if badQuery := query.Get(allowedBadQueryParamName); len(badQuery) > 0 {
		logger.Sugar.Debugf("Scanned status BadQuery %v", badQuery)
		allowInsecure, err = strconv.ParseBool(badQuery)
		if err != nil {
			return newError(
				"illegal value for '%s' must be truthy value: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False",
				allowedBadQueryParamName,
			)
		}
	}

	if strict := query.Get(propmtForPendingParamName); len(strict) > 0 {
		logger.Sugar.Debugf("Scanned status strict %v", strict)
		strictValidation, err = strconv.ParseBool(strict)
		if err != nil {
			return newError(
				"illegal value for '%s' must be truthy value: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False",
				propmtForPendingParamName,
			)
		}
	}

	if scannedStatus == ScannedBad.String() && !allowInsecure {
		logger.Sugar.Debugf("Scanned bad disallowed")
		return newError(
			"this attachment is not safe - if you wish to download it add %s=%s",
			allowedBadQueryParamName,
			allowedQueryParamValue,
		)
	}

	if scannedStatus == NotScanned.String() && strictValidation && !allowInsecure {
		logger.Sugar.Debugf("Scanned not scanned disallowed")
		return newError(
			"this attachment is potentially not safe - if you wish to download it add %s=%s",
			allowedBadQueryParamName,
			allowedQueryParamValue,
		)
	}

	return nil
}
