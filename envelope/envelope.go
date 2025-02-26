package envelope

type Envelope struct {
	Header  EnvelopeHeader
	Payload []byte
}

func (e *Envelope) Bytes() []byte {
	bytes := []byte{}
	bytes = append(bytes, e.Header.Bytes()...)
	bytes = append(bytes, e.Payload...)
	return bytes
}
