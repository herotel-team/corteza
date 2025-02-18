package consumer

import (
	"context"
	"github.com/cortezaproject/corteza/server/pkg/eventbus"
	"github.com/cortezaproject/corteza/server/pkg/servicebus"
	"go.uber.org/zap"
	"time"

	"github.com/cortezaproject/corteza/server/pkg/messagebus/types"
)

type (
	// @fixme: make this common type
	ServicebusDispatcher interface {
		CreateQueue(context.Context, string) error
		// DeleteQueue(context.Context, string) error
		SendMessage(context.Context, string, []byte) error
	}

	QueueEventBuilder interface {
		CreateQueueEvent(string, []byte) eventbus.Event
	}

	ServicebusConsumer struct {
		logger *zap.Logger

		// @fixme:not sure about this
		// Azure service bus connection string
		connStr string

		queue  string
		handle types.ConsumerType
		// explorer servicebus.Client
		poll *time.Ticker

		// @fixme: not sure about this weather to keep it here or not
		servicer QueueEventBuilder

		dispatcher ServicebusDispatcher
	}
)

func NewServicebusConsumer(_ context.Context, logger *zap.Logger, connStr string, q string, servicer types.QueueEventBuilder) (sb *ServicebusConsumer, err error) {
	sb = &ServicebusConsumer{
		logger: logger,

		queue:    q,
		handle:   types.ConsumerServicebus,
		servicer: servicer,
	}

	sb.dispatcher, err = servicebus.Service(logger, connStr)
	if err != nil {
		return
	}

	return
}

func (sb *ServicebusConsumer) Write(ctx context.Context, p []byte) (err error) {
	err = sb.dispatcher.SendMessage(ctx, sb.queue, p)
	if err != nil {
		return
	}

	_ = sb.servicer.CreateQueueEvent(sb.queue, p)

	return
}

func (sb *ServicebusConsumer) GetConsumerType() string {
	if sb.handle == "" {
		return ""
	}

	return string(sb.handle)
}
