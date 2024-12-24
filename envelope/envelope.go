package envelope

import (
	"bytes"
	"fmt"
)

type Envelope struct {
	Header EnvelopeHeader
	Items  []byte
}

func (e *Envelope) Bytes() []byte {
	bytes := []byte{}
	bytes = append(bytes, e.Header.Bytes()...)
	bytes = append(bytes, e.Items...)
	return bytes
}

func Parse(bytes []byte) (*Envelope, error) {
	envelope := &Envelope{}
	err := Unmarshal(bytes, envelope)
	if err != nil {
		return nil, err
	}

	return envelope, nil
}

func Unmarshal(data []byte, envelope *Envelope) error {
	lines := bytes.SplitN(data, []byte("\n"), 2)

	// Verify that the envelope has at least two lines
	if len(lines) < 2 {
		return fmt.Errorf("error parsing envelope")
	}

	// Parse the envelope header
	envelopeHeader, err := parseEnvelopeHeader(lines[0])
	if err != nil {
		return err
	}

	envelope.Header = *envelopeHeader
	envelope.Items = lines[1]

	return nil
}
