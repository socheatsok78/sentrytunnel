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
	envelopeHeader, err := parseEnvelopeHeader(lines[0])
	if err != nil {
		return err
	}

	// Set the header
	envelope.Header = *envelopeHeader

	// Set the payload
	envelope.Payload = lines[1]

	// Return nil to indicate success
	return nil
}
