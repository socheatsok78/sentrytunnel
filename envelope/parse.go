package envelope

func Parse(bytes []byte) (*Envelope, error) {
	envelope := &Envelope{}
	err := Unmarshal(bytes, envelope)
	if err != nil {
		return nil, err
	}

	return envelope, nil
}
