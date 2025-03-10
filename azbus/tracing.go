package azbus

import (
	"github.com/datatrails/go-datatrails-common/tracing"
)

var (
	NewSpan = tracing.NewSpan
)

type NewSpanFunc = tracing.NewSpanFunc
type Spanner = tracing.Spanner
type Span = tracing.Span
