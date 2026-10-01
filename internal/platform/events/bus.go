package events

import "context"

type Publisher interface {
	PublishOrderStatus(ctx context.Context, event OrderStatus) error
}

type JobSink interface {
	EnqueueOrderStatus(ctx context.Context, event OrderStatus) error
}

type Direct struct {
	jobs JobSink
}

func NewDirect(jobs JobSink) *Direct {
	return &Direct{jobs: jobs}
}

func (d *Direct) PublishOrderStatus(ctx context.Context, event OrderStatus) error {
	return d.jobs.EnqueueOrderStatus(ctx, event)
}
