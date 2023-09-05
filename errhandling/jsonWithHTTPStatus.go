package errhandling

import (
	"fmt"
)

func JSONWithHTTPStatus(statusCode int, message string) string {
	return fmt.Sprintf(`{"code": "%d", "message": "%s", "details": []}`, statusCode, message)
}
