package envelope

type EnvelopePayload struct {
	Payload []byte
}

func (e *EnvelopePayload) Bytes() []byte {
	return e.Payload
}

func parseEnvelopePayload(bytes []byte) (*EnvelopePayload, error) {
	envelopeHeader := &EnvelopePayload{
		Payload: bytes,
	}

	return envelopeHeader, nil
}
