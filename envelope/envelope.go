package envelope

import (
	"fmt"
	"strings"
)

type Envelope struct {
	Header EnvelopeHeader
	Items  []byte
}

func (e *Envelope) String() string {
	bytes := []byte{}
	bytes = append(bytes, []byte(e.Header.String())...)
	bytes = append(bytes, e.Items...)
	return string(bytes)
}

func Parse(bytes []byte) (*Envelope, error) {
	envelope := &Envelope{}
	err := Unmarshal(bytes, envelope)
	if err != nil {
		return nil, err
	}

	return envelope, nil
}

func Unmarshal(bytes []byte, envelope *Envelope) error {
	lines := strings.SplitN(string(bytes), "\n", 2)

	// Verify that the envelope has at least two lines
	if len(lines) < 2 {
		return fmt.Errorf("error parsing envelope")
	}

	// Parse the envelope header
	envelopeHeader, err := parseEnvelopeHeader([]byte(lines[0]))
	if err != nil {
		return err
	}

	envelope.Header = *envelopeHeader
	envelope.Items = []byte(lines[1])

	return nil
}
