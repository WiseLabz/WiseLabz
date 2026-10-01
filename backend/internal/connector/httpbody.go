package connector

import (
	"io"

	"github.com/WiseLabz/wiselabz/internal/httpx"
)

// MaxResponseBytes caps how much of an upstream HTTP response body is read.
const MaxResponseBytes = httpx.MaxResponseBytes

// ReadBody reads r up to MaxResponseBytes and errors if the body is larger.
func ReadBody(r io.Reader) ([]byte, error) { return httpx.ReadBody(r) }

// LimitedBody wraps r so reads stop after MaxResponseBytes (for streaming decoders).
func LimitedBody(r io.Reader) io.Reader { return httpx.LimitedBody(r) }
