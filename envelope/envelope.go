package envelope

type Envelope struct {
	Header  EnvelopeHeader
	Type    EnvelopeType
	Payload []byte
}

func (e *Envelope) Bytes() []byte {
	bytes := []byte{}
	bytes = append(bytes, e.Header.Bytes()...)
	bytes = append(bytes, e.Type.Bytes()...)
	bytes = append(bytes, e.Payload...)
	return bytes
}
