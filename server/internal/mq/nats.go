package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"mysecondwebgame/server/internal/reqmetrics"
)

type Publisher interface {
	PublishJSON(ctx context.Context, topic string, payload any) error
	Close()
}

type NatsPublisher struct {
	conn   *nats.Conn
	prefix string
}

func NewNatsPublisher(url, topicPrefix string) (*NatsPublisher, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NatsPublisher{conn: conn, prefix: topicPrefix}, nil
}

func (p *NatsPublisher) PublishJSON(ctx context.Context, topic string, payload any) error {
	startAt := time.Now()
	defer func() {
		if collector := reqmetrics.FromContext(ctx); collector != nil {
			collector.AddNATS(time.Since(startAt))
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	subject := topic
	if p.prefix != "" {
		subject = fmt.Sprintf("%s.%s", p.prefix, topic)
	}
	return p.conn.Publish(subject, body)
}

func (p *NatsPublisher) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}

// Each runtime receives invalidations; this must not be a queue subscription.
func (p *NatsPublisher) SubscribeBytes(topic string, handle func([]byte)) error {
	subject := topic
	if p.prefix != "" {
		subject = p.prefix + "." + topic
	}
	_, err := p.conn.Subscribe(subject, func(msg *nats.Msg) { handle(msg.Data) })
	return err
}
