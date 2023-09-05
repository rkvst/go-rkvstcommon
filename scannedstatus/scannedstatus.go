package scannedstatus

import (
	"strconv"
)

type ScannedStatus int

// there is a fundamental error here in that the default zero value
// is used for something meaningful. Better would be to name the zero
// value to UNSPECIFIED and NotScanned to 1...
// difficult to change now...
const (
	NotScanned ScannedStatus = iota
	ScannedOK
	ScannedBad
)

func (s ScannedStatus) String() string {
	switch s {
	case NotScanned:
		return "NOT_SCANNED"
	case ScannedOK:
		return "SCANNED_OK"
	case ScannedBad:
		return "SCANNED_BAD"
	}
	return "NOT_SCANNED"
}

func Value(s string) ScannedStatus {
	switch s {
	case "NOT_SCANNED":
		return NotScanned
	case "SCANNED_OK":
		return ScannedOK
	case "SCANNED_BAD":
		return ScannedBad
	}
	return NotScanned
}

func FromString(stringVal string) ScannedStatus {
	val, err := strconv.Atoi(stringVal)
	if err != nil {
		return NotScanned
	}
	return ScannedStatus(val)
}
