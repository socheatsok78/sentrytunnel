package envelope

import "bytes"

type Envelope struct {
	Header  EnvelopeHeader
	Payload EnvelopePayload
}

func (e *Envelope) Bytes() []byte {
	bytes := []byte{}
	bytes = append(bytes, e.Header.Bytes()...)
	bytes = append(bytes, []byte("\n")...)
	bytes = append(bytes, e.Payload.Bytes()...)
	return bytes
}

func (e *Envelope) NewReader() *bytes.Reader {
	return bytes.NewReader(e.Bytes())
}
