package envelope

import (
	"encoding/json"
	"fmt"
)

type EnvelopeHeader struct {
	DSN     string
	Payload []byte
}

func (e *EnvelopeHeader) Bytes() []byte {
	return e.Payload
}

func parseEnvelopeHeader(bytes []byte) (*EnvelopeHeader, error) {
	envelopeHeader := &EnvelopeHeader{
		Payload: bytes,
	}

	err := json.Unmarshal(bytes, envelopeHeader)
	if err != nil {
		return nil, fmt.Errorf("error parsing envelope header")
	}

	return envelopeHeader, nil
}
