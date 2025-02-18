package servicebus

import (
	"context"
	"go.uber.org/zap"
	"sync"
)

type (
	client interface {
		createQueue(context.Context, string) error
		deleteQueue(context.Context, string) error
		sendMessage(context.Context, string, []byte) error
		// GetMessage(context.Context, int, string)
	}

	servicebus struct {
		// waitgroup for dispatch
		wg *sync.WaitGroup

		// Read & write locking
		l *sync.RWMutex

		// client for service bus
		client client
	}
)

func Service(logger *zap.Logger, connStr string) (sb *servicebus, err error) {
	sb = &servicebus{
		wg: &sync.WaitGroup{},
		l:  &sync.RWMutex{},
	}

	sb.client, err = newClient(logger, connStr)
	if err != nil {
		return
	}

	return
}

func (sb *servicebus) CreateQueue(ctx context.Context, q string) (err error) {
	err = sb.client.createQueue(ctx, q)
	if err != nil {
	}
	return
}

func (sb *servicebus) SendMessage(ctx context.Context, q string, payload []byte) (err error) {
	err = sb.client.sendMessage(ctx, q, payload)
	if err != nil {
		return
	}
	return
}

func (sb *servicebus) DeleteQueue(ctx context.Context, q string) (err error) {
	err = sb.client.deleteQueue(ctx, q)
	if err != nil {
		return
	}
	return
}
