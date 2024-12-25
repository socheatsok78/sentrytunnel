package envelope

import (
	"bytes"
	"fmt"
)

func Unmarshal(data []byte, envelope *Envelope) error {
	lines := bytes.SplitN(data, []byte("\n"), 3)

	// Verify that the envelope has at least two lines
	if len(lines) < 2 {
		return fmt.Errorf("error parsing envelope")
	}

	// Parse the envelope header
	envelopeHeader, err := parseEnvelopeHeader(lines[0])
	if err != nil {
		return err
	}

	envelopeType, err := parseEnvelopeType(lines[1])
	if err != nil {
		return err
	}

	envelope.Header = *envelopeHeader
	envelope.Type = *envelopeType
	envelope.Payload = lines[2]

	return nil
}
