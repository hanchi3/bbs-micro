package kafka

import (
	"bluebell_microservices/common/pkg/logger"
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
	consumer sarama.Consumer
	topic    string
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

	consumer, err := sarama.NewConsumer(config.Brokers, saramaConfig)
	if err != nil {
		logger.Error("Failed to create Kafka consumer", zap.Error(err))
		return nil, err
	}

	return &Consumer{
		consumer: consumer,
		topic:    config.Topic,
	}, nil
}

// ConsumeMessages 消费消息
func (c *Consumer) ConsumeMessages(handler func(message VoteMessage) error) error {
	partitions, err := c.consumer.Partitions(c.topic)
	if err != nil {
		logger.Error("Failed to get partitions", zap.Error(err))
		return err
	}

	for _, partition := range partitions {
		pc, err := c.consumer.ConsumePartition(c.topic, partition, sarama.OffsetNewest)
		if err != nil {
			logger.Error("Failed to create partition consumer", zap.Error(err))
			continue
		}

		go func(pc sarama.PartitionConsumer) {
			defer pc.Close()

			for msg := range pc.Messages() {
				var voteMsg VoteMessage
				if err := json.Unmarshal(msg.Value, &voteMsg); err != nil {
					logger.Error("Failed to unmarshal vote message", zap.Error(err))
					continue
				}

				if err := handler(voteMsg); err != nil {
					logger.Error("Failed to process vote message", zap.Error(err))
				}

				pc.MarkOffset(msg.Offset, "")
			}
		}(pc)
	}

	return nil
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	return c.consumer.Close()
}
