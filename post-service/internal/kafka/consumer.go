package kafka

import (
	"bluebell_microservices/common/config"
	"bluebell_microservices/common/pkg/kafka"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/post-service/internal/dao/mysql"
	"bluebell_microservices/post-service/internal/dao/redis"
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
	voteCounts      map[int64]int64
	voteCountsMutex sync.Mutex
	voteCountsFile  string
	ctx             context.Context
	cancel          context.CancelFunc
	batch           []struct {
		PostID    int64
		UserID    int64
		Direction int64
	}
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
			config.BatchSize = 100
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

		consumer = &Consumer{
			consumer:       kafkaConsumer,
			batchSize:      config.BatchSize,
			voteCounts:     make(map[int64]int64),
			voteCountsFile: config.VoteCountsFile,
		}
	})

	return initErr
}

// GetConsumer 获取Kafka消费者实例
func GetConsumer() *Consumer {
	return consumer
}

// processBatch 批量处理消息
func (c *Consumer) processBatch(batch []struct {
	PostID    int64
	UserID    int64
	Direction int64
}) {
	if len(batch) == 0 {
		return
	}

	// 获取数据库连接
	db := mysql.DB()
	redisClient := redis.Client()

	// 开始数据库事务
	tx, err := db.Begin()
	if err != nil {
		logger.Error("Failed to begin transaction", zap.Error(err))
		return
	}

	// 准备SQL语句
	stmt, err := tx.Prepare(`
		INSERT INTO vote (post_id, user_id, vote_type)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE vote_type = VALUES(vote_type)
	`)
	if err != nil {
		logger.Error("Failed to prepare statement", zap.Error(err))
		tx.Rollback()
		return
	}
	defer stmt.Close()

	// 执行批量插入
	for _, vote := range batch {
		_, err := stmt.Exec(vote.PostID, vote.UserID, vote.Direction)
		if err != nil {
			logger.Error("Failed to insert vote",
				zap.Int64("post_id", vote.PostID),
				zap.Int64("user_id", vote.UserID),
				zap.Int64("direction", vote.Direction),
				zap.Error(err))
			tx.Rollback()
			return
		}

		// 更新Redis中的投票状态为已入库(1)
		voteStatusKey := fmt.Sprintf("bluebell-plus:vote:status:%d:%d", vote.PostID, vote.UserID)
		err = redisClient.Set(voteStatusKey, 1, 24*time.Hour).Err()
		if err != nil {
			logger.Error("Failed to update vote status",
				zap.Int64("post_id", vote.PostID),
				zap.Int64("user_id", vote.UserID),
				zap.Error(err))
			// 继续处理，不中断批量处理
		}

		// 更新vote_counts
		c.updateVoteCount(vote.PostID, vote.Direction)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit transaction", zap.Error(err))
		tx.Rollback()
		return
	}

	logger.Info("Successfully processed batch", zap.Int("batch_size", len(batch)))
}

// updateVoteCount 更新投票计数
func (c *Consumer) updateVoteCount(postID int64, direction int64) {
	c.voteCountsMutex.Lock()
	defer c.voteCountsMutex.Unlock()

	// 更新投票计数
	c.voteCounts[postID] += direction
}

// periodicallySaveVoteCounts 定期保存vote_counts.json
func (c *Consumer) periodicallySaveVoteCounts(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 上下文取消，退出
			return
		case <-ticker.C:
			// 定时器触发，保存vote_counts.json
			c.saveVoteCounts()
		}
	}
}

// saveVoteCounts 保存vote_counts.json
func (c *Consumer) saveVoteCounts() {
	c.voteCountsMutex.Lock()
	defer c.voteCountsMutex.Unlock()

	// 将vote_counts转换为JSON
	jsonData, err := json.MarshalIndent(c.voteCounts, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal vote counts", zap.Error(err))
		return
	}

	// 写入文件
	err = os.WriteFile(c.voteCountsFile, jsonData, 0644)
	if err != nil {
		logger.Error("Failed to write vote counts to file", zap.Error(err))
		return
	}

	logger.Info("Successfully saved vote counts",
		zap.String("file", c.voteCountsFile),
		zap.Int("count", len(c.voteCounts)))
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	if c.consumer != nil {
		return c.consumer.Close()
	}
	return nil
}

// processMessages 处理消息
func (c *Consumer) processMessages(ctx context.Context) {
	// 批量处理的计时器
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 处理消息
	for {
		select {
		case <-ctx.Done():
			// 上下文取消，退出
			return
		case <-ticker.C:
			// 定时器触发，处理批量处理队列
			if len(c.batch) > 0 {
				c.processBatch(c.batch)
				c.batch = c.batch[:0] // 清空批量处理队列
			}
		}
	}
}

// Start 启动消费者
func (c *Consumer) Start(ctx context.Context) error {
	// 启动消息处理
	err := c.consumer.ConsumeMessages(func(msg kafka.VoteMessage) error {
		// 添加到批量处理队列
		c.batch = append(c.batch, struct {
			PostID    int64
			UserID    int64
			Direction int64
		}{
			PostID:    msg.PostID,
			UserID:    msg.UserID,
			Direction: msg.Direction,
		})

		// 如果批量处理队列达到大小，处理它
		if len(c.batch) >= c.batchSize {
			c.processBatch(c.batch)
			c.batch = c.batch[:0] // 清空批量处理队列
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to start consumer", zap.Error(err))
		return err
	}

	// 启动一个goroutine定期处理批量消息
	go c.processMessages(ctx)

	return nil
}
