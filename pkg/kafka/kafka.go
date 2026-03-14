package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
)

type Config struct {
	Brokers []string
}

func NewSyncProducer(cfg Config) (sarama.SyncProducer, error) {
	producerCfg := sarama.NewConfig()
	producerCfg.Producer.RequiredAcks = sarama.WaitForAll
	producerCfg.Producer.Return.Successes = true
	producerCfg.Producer.Retry.Max = 5
	producerCfg.Version = sarama.V2_8_0_0

	return sarama.NewSyncProducer(cfg.Brokers, producerCfg)
}

func PublishJSON(producer sarama.SyncProducer, topic, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(body),
	}

	_, _, err = producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

type HandlerFunc func(ctx context.Context, msg *sarama.ConsumerMessage) error

type consumerGroupHandler struct {
	ctx     context.Context
	handler HandlerFunc
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-h.ctx.Done():
			return h.ctx.Err()
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if err := h.handler(h.ctx, msg); err == nil {
				session.MarkMessage(msg, "")
			}
		}
	}
}

func Consume(ctx context.Context, cfg Config, groupID string, topics []string, handler HandlerFunc) error {
	consumerCfg := sarama.NewConfig()
	consumerCfg.Version = sarama.V2_8_0_0
	consumerCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	consumerCfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	group, err := sarama.NewConsumerGroup(cfg.Brokers, groupID, consumerCfg)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer func() { _ = group.Close() }()

	h := &consumerGroupHandler{ctx: ctx, handler: handler}
	for {
		if err := group.Consume(ctx, topics, h); err != nil {
			return fmt.Errorf("consume: %w", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}
