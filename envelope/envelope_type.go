package envelope

import (
	"encoding/json"
	"fmt"
)

type EnvelopeType struct {
	Type string `json:"type"`
}

func (e *EnvelopeType) Bytes() []byte {
	bytes, _ := json.Marshal(e)
	bytes = append(bytes, []byte("\n")...)
	return bytes
}

func parseEnvelopeType(bytes []byte) (*EnvelopeType, error) {
	envelopeHeader := &EnvelopeType{}

	err := json.Unmarshal(bytes, envelopeHeader)
	if err != nil {
		return nil, fmt.Errorf("error parsing envelope header")
	}

	return envelopeHeader, nil
}
