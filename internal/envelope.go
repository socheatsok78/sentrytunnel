package internal

import (
	"bytes"
	"fmt"
)

func Parse(data []byte) (*EnvelopeHeader, error) {
	lines := bytes.SplitN(data, []byte("\n"), 2)
	if len(lines) < 2 {
		return nil, fmt.Errorf("error parsing envelope")
	}
	return ParseEnvelopeHeader(lines[0])
}
