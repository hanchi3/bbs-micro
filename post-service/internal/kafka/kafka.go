package kafka

import (
	"bluebell_microservices/common/pkg/kafka"
	"bluebell_microservices/common/pkg/logger"
	"fmt"

	"go.uber.org/zap"
)

// Producer Kafka生产者
type Producer struct {
	producer *kafka.Producer
}

// NewProducer 创建Kafka生产者
func NewProducer() *Producer {
	// 初始化Kafka生产者
	kafkaConfig := kafka.KafkaConfig{
		Brokers: []string{"kafka:9092"}, // 默认配置，实际应从配置文件读取
		Topic:   "post-votes",
	}

	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		logger.Error("Failed to create Kafka producer", zap.Error(err))
		return nil
	}

	return &Producer{
		producer: producer,
	}
}

// SendVoteMessage 发送投票消息
func (p *Producer) SendVoteMessage(message kafka.VoteMessage) error {
	if p.producer == nil {
		return fmt.Errorf("kafka producer is not initialized")
	}
	return p.producer.SendVoteMessage(message)
}

// Close 关闭生产者
func (p *Producer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}
