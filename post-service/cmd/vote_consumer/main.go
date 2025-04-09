package main

import (
	"bluebell_microservices/common/pkg/kafka"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/post-service/internal/dao/redis"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

var (
	kafkaBrokers = flag.String("kafka-brokers", "localhost:9092", "Kafka brokers, comma separated")
	kafkaTopic   = flag.String("kafka-topic", "post-votes", "Kafka topic for vote messages")
)

func main() {
	flag.Parse()

	// 初始化Kafka消费者
	kafkaConfig := kafka.KafkaConfig{
		Brokers: []string{*kafkaBrokers},
		Topic:   *kafkaTopic,
	}

	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		logger.Error("Failed to create Kafka consumer", zap.Error(err))
		os.Exit(1)
	}
	defer consumer.Close()

	// 处理投票消息
	err = consumer.ConsumeMessages(func(message kafka.VoteMessage) error {
		logger.Info("Processing vote message",
			zap.Int64("post_id", message.PostID),
			zap.Int64("user_id", message.UserID),
			zap.Int64("direction", message.Direction),
			zap.Int64("timestamp", message.Timestamp))

		// 调用Redis处理投票
		err := redis.CreatePostVote(message.PostID, message.UserID, message.Direction)
		if err != nil {
			logger.Error("Failed to process vote",
				zap.Int64("post_id", message.PostID),
				zap.Int64("user_id", message.UserID),
				zap.Error(err))
			return err
		}

		logger.Info("Vote processed successfully",
			zap.Int64("post_id", message.PostID),
			zap.Int64("user_id", message.UserID),
			zap.Int64("direction", message.Direction))

		return nil
	})

	if err != nil {
		logger.Error("Failed to consume messages", zap.Error(err))
		os.Exit(1)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down vote consumer")
}
