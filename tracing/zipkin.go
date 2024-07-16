package tracing

import (
	"log"
	"os"
)

func newZipkinLogger() *log.Logger {
	return log.New(os.Stdout, "zipkin", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
}
