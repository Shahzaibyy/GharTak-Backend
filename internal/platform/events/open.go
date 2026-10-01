package events

func Open(natsURL string, jobs JobSink) (Publisher, *NATS, func(), error) {
	if natsURL == "" {
		return NewDirect(jobs), nil, func() {}, nil
	}
	bus, err := OpenNATS(natsURL)
	if err != nil {
		return nil, nil, nil, err
	}
	return bus, bus, bus.Close, nil
}
