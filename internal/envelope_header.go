package internal

import (
	"encoding/json"
	"fmt"
)

type EnvelopeHeader struct {
	DSN string
}

func ParseEnvelopeHeader(bytes []byte) (*EnvelopeHeader, error) {
	envelopeHeader := &EnvelopeHeader{}

	err := json.Unmarshal(bytes, envelopeHeader)
	if err != nil {
		return nil, fmt.Errorf("error parsing envelope header")
	}

	return envelopeHeader, nil
}
