package envelope

type EnvelopePayload struct {
	RawBytes []byte `json:"-"`
}

func (e *EnvelopePayload) Bytes() []byte {
	return e.RawBytes
}
