package kafka

import (
	"bluebell_microservices/common/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers []string
	Topic   string
}

// VoteMessage 投票消息结构
type VoteMessage struct {
	PostID    int64 `json:"post_id"`
	UserID    int64 `json:"user_id"`
	Direction int64 `json:"direction"`
	Timestamp int64 `json:"timestamp"`
}

// Producer Kafka生产者
type Producer struct {
	producer sarama.SyncProducer
	topic    string
}

// Consumer Kafka消费者
type Consumer struct {
	group   sarama.ConsumerGroup
	topic   string
	handler func(message VoteMessage) error
	ready   chan bool
	topics  []string
}

// NewProducer 创建Kafka生产者
func NewProducer(config KafkaConfig) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 5
	saramaConfig.Producer.Retry.Backoff = 100 * time.Millisecond

	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		logger.Error("Failed to create Kafka producer", zap.Error(err))
		return nil, err
	}

	return &Producer{
		producer: producer,
		topic:    config.Topic,
	}, nil
}

// SendVoteMessage 发送投票消息
func (p *Producer) SendVoteMessage(message VoteMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		logger.Error("Failed to marshal vote message", zap.Error(err))
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(jsonData),
		Key:   sarama.StringEncoder(fmt.Sprintf("%d-%d", message.PostID, message.UserID)),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		logger.Error("Failed to send message to Kafka", zap.Error(err))
		return err
	}

	logger.Info("Message sent to Kafka",
		zap.String("topic", p.topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.Int64("post_id", message.PostID),
		zap.Int64("user_id", message.UserID),
		zap.Int64("direction", message.Direction))

	return nil
}

// Close 关闭生产者
func (p *Producer) Close() error {
	return p.producer.Close()
}

// NewConsumer 创建Kafka消费者
func NewConsumer(config KafkaConfig) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	// 设置消费者组重平衡策略
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	// 设置消费者组会话超时时间
	saramaConfig.Consumer.Group.Session.Timeout = 20 * time.Second
	// 设置消费者组心跳间隔
	saramaConfig.Consumer.Group.Heartbeat.Interval = 6 * time.Second

	// 创建消费者组
	group, err := sarama.NewConsumerGroup(config.Brokers, "post-service-group", saramaConfig)
	if err != nil {
		logger.Error("Failed to create consumer group", zap.Error(err))
		return nil, err
	}

	return &Consumer{
		group:  group,
		topic:  config.Topic,
		ready:  make(chan bool),
		topics: []string{config.Topic},
	}, nil
}

// ConsumeMessages 消费消息
func (c *Consumer) ConsumeMessages(handler func(message VoteMessage) error) error {
	c.handler = handler

	// 启动消费者组
	go func() {
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err := c.group.Consume(context.Background(), c.topics, c); err != nil {
				logger.Error("Error from consumer", zap.Error(err))
			}
			// check if context was cancelled, signaling that the consumer should stop
			if context.Background().Err() != nil {
				return
			}
			c.ready = make(chan bool)
		}
	}()

	<-c.ready // Await till the consumer has been set up
	logger.Info("Consumer up and running!...")

	return nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(c.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var voteMsg VoteMessage
		if err := json.Unmarshal(message.Value, &voteMsg); err != nil {
			logger.Error("Failed to unmarshal vote message", zap.Error(err))
			continue
		}

		if err := c.handler(voteMsg); err != nil {
			logger.Error("Failed to process vote message", zap.Error(err))
		}

		session.MarkMessage(message, "")
	}

	return nil
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	return c.group.Close()
}
