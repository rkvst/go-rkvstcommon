package logger

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// It is expected that WithContext will have trace ID set
func BenchmarkWrappedLogger_FromContextTraceID(b *testing.B) {
	tests := []struct {
		name string
	}{
		{
			name: "positive",
		},
	}
	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {

			New("NOOP")

			ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{TraceID: {"foobar"}})
			for n := 0; n < b.N; n++ {
				func(inctx context.Context) {
					log := Sugar.FromContext(inctx)
					defer log.Close()
				}(ctx)
			}
		})
	}
}
