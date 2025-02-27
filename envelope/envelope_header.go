package envelope

import (
	"encoding/json"
	"fmt"
)

type EnvelopeHeader struct {
	DSN      string `json:"dsn,omitempty"`
	RawBytes []byte `json:"-"`
}

func (e *EnvelopeHeader) OmitDsnFromHeader() {
	e.DSN = ""
}

func (e *EnvelopeHeader) Bytes() []byte {
	return e.RawBytes
}

func parseEnvelopeHeader(bytes []byte) (*EnvelopeHeader, error) {
	envelopeHeader := &EnvelopeHeader{
		RawBytes: bytes,
	}

	err := json.Unmarshal(bytes, envelopeHeader)
	if err != nil {
		return nil, fmt.Errorf("error parsing envelope header")
	}

	return envelopeHeader, nil
}
