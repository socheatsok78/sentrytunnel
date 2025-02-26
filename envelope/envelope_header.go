package envelope

import (
	"encoding/json"
	"fmt"
)

type EnvelopeHeader struct {
	DSN    string `json:"dsn,omitempty"`
	SDK    sdk    `json:"sdk"`
	SentAt string `json:"sent_at"`
}

func (e *EnvelopeHeader) OmitDsnFromHeader() {
	e.DSN = ""
}

func (e *EnvelopeHeader) Bytes() []byte {
	bytes, _ := json.Marshal(e)
	bytes = append(bytes, []byte("\n")...)
	return bytes
}

func (e *EnvelopeHeader) PrettyPrint() string {
	return fmt.Sprintf("EnvelopeHeader{DSN: %s, SDK: %s, SentAt: %s}", e.DSN, e.SDK, e.SentAt)
}

func parseEnvelopeHeader(bytes []byte) (*EnvelopeHeader, error) {
	envelopeHeader := &EnvelopeHeader{}

	err := json.Unmarshal(bytes, envelopeHeader)
	if err != nil {
		return nil, fmt.Errorf("error parsing envelope header")
	}

	return envelopeHeader, nil
}

type sdk struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
