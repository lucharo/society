package transport

import "context"

type Transport interface {
	Open(ctx context.Context) error
	Send(ctx context.Context, payload []byte) ([]byte, error)
	Close() error
}
