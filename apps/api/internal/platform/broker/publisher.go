package broker

import "context"

type Publisher interface {
	Publish(context.Context, Envelope) error
	Close() error
}
