package kafka

import (
	"bluebell_microservices/common/config"
	"bluebell_microservices/common/pkg/kafka"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/common/pkg/snowflake"
	"bluebell_microservices/post-service/internal/dao/mysql"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Consumer Kafka消费者
type Consumer struct {
	consumer        *kafka.Consumer
	batchSize       int
	voteCounts      map[int64]int64 // 帖子投票计数
	voteCountsMutex sync.Mutex
	voteCountsFile  string
	ctx             context.Context
	cancel          context.CancelFunc
}

// VoteRecord 投票记录结构（用于 MySQL）
type VoteRecord struct {
	VoteID    int64
	PostID    int64
	UserID    int64
	Direction int64
	CreatedAt time.Time
}

var (
	consumer *Consumer
	once     sync.Once
)

// Init 初始化Kafka消费者
func Init(config *config.Kafka) error {
	var initErr error
	once.Do(func() {
		// 设置默认值
		if config.BatchSize == 0 {
			config.BatchSize = 1
		}
		if config.VoteCountsFile == "" {
			config.VoteCountsFile = filepath.Join("data", "vote_count.json")
		}
		if len(config.Brokers) == 0 {
			config.Brokers = []string{"kafka:9092"}
		}
		if config.Topic == "" {
			config.Topic = "post-votes"
		}

		// 确保目录存在
		os.MkdirAll(filepath.Dir(config.VoteCountsFile), 0755)

		// 初始化Kafka消费者
		kafkaConfig := kafka.KafkaConfig{
			Brokers: config.Brokers,
			Topic:   config.Topic,
		}

		kafkaConsumer, err := kafka.NewConsumer(kafkaConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to create Kafka consumer: %v", err)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		consumer = &Consumer{
			consumer:       kafkaConsumer,
			batchSize:      config.BatchSize,
			voteCounts:     make(map[int64]int64),
			voteCountsFile: config.VoteCountsFile,
			ctx:            ctx,
			cancel:         cancel,
		}

		// 加载现有的 vote_counts.json
		if err := consumer.loadVoteCounts(); err != nil {
			logger.Warn("Failed to load vote counts", zap.Error(err))
		}
	})

	return initErr
}

// GetConsumer 获取Kafka消费者实例
func GetConsumer() *Consumer {
	return consumer
}

// loadVoteCounts 从文件加载 vote_counts
func (c *Consumer) loadVoteCounts() error {
	c.voteCountsMutex.Lock()
	defer c.voteCountsMutex.Unlock()

	data, err := os.ReadFile(c.voteCountsFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		return json.Unmarshal(data, &c.voteCounts)
	}
	return nil
}

// processVote 处理单条投票消息
func (c *Consumer) processVote(msg kafka.VoteMessage) error {
	// 生成雪花算法ID
	voteID, err := snowflake.GetID()
	if err != nil {
		logger.Error("Failed to genID",
			zap.Int64("post_id", msg.PostID),
			zap.Int64("user_id", msg.UserID),
			zap.Int64("direction", msg.Direction),
			zap.Error(err))
		return err
	}

	// 1. 写入 MySQL
	db := mysql.DB()
	_, err = db.Exec(`
		INSERT INTO vote (vote_id, post_id, user_id, vote_type, created_at)
		VALUES (?, ?, ?, ?, NOW())`,
		voteID, msg.PostID, msg.UserID, msg.Direction)
	if err != nil {
		logger.Error("Failed to insert vote into MySQL",
			zap.Uint64("vote_id", voteID),
			zap.Int64("post_id", msg.PostID),
			zap.Int64("user_id", msg.UserID),
			zap.Int64("direction", msg.Direction),
			zap.Error(err))
		return err
	}

	// 2. 更新 voteCounts 并写入 JSON 文件
	c.voteCountsMutex.Lock()
	c.voteCounts[msg.PostID] += msg.Direction
	c.voteCountsMutex.Unlock()

	// 保存到文件（每次投票都保存，生产中可优化为定期保存）
	c.saveVoteCounts()

	logger.Info("Vote processed",
		zap.Uint64("vote_id", voteID),
		zap.Int64("post_id", msg.PostID),
		zap.Int64("user_id", msg.UserID),
		zap.Int64("direction", msg.Direction))
	return nil
}

// saveVoteCounts 保存 vote_counts 到 JSON 文件
func (c *Consumer) saveVoteCounts() {
	c.voteCountsMutex.Lock()
	defer c.voteCountsMutex.Unlock()

	jsonData, err := json.MarshalIndent(c.voteCounts, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal vote counts", zap.Error(err))
		return
	}

	err = os.WriteFile(c.voteCountsFile, jsonData, 0644)
	if err != nil {
		logger.Error("Failed to write vote counts to file", zap.Error(err))
		return
	}
}

// periodicallySaveVoteCounts 定期保存 vote_counts
func (c *Consumer) periodicallySaveVoteCounts() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.saveVoteCounts() // 退出前保存一次
			return
		case <-ticker.C:
			c.saveVoteCounts()
		}
	}
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	c.cancel() // 取消上下文，停止所有 goroutine
	if c.consumer != nil {
		return c.consumer.Close()
	}
	return nil
}

// Start 启动消费者
func (c *Consumer) Start() error {
	// 启动定期保存 vote_counts
	go c.periodicallySaveVoteCounts()

	// 处理消息
	err := c.consumer.ConsumeMessages(func(msg kafka.VoteMessage) error {
		return c.processVote(msg)
	})
	if err != nil {
		logger.Error("Failed to start consumer", zap.Error(err))
		return err
	}

	return nil
}
