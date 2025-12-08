package envelope

import (
	"bytes"
	"fmt"
)

func Unmarshal(data []byte, envelope *Envelope) error {
	lines := bytes.SplitN(data, []byte("\n"), 2)

	// Verify that the envelope has at least two items
	if len(lines) < 2 {
		return fmt.Errorf("error parsing envelope")
	}

	// Parse the envelope header
	if envelopeHeader, err := ParseEnvelopeHeader(lines[0]); err == nil {
		envelope.Header = *envelopeHeader
	} else {
		return err
	}

	// Parse the envelope payload
	if envelopePayload, err := parseEnvelopePayload(lines[1]); err == nil {
		envelope.Payload = *envelopePayload
	} else {
		return err
	}

	// Return nil to indicate success
	return nil
}
