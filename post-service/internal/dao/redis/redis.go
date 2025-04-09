package redis

import (
	"fmt"

	"bluebell_microservices/common/config"
	"bluebell_microservices/common/pkg/logger"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

var client *redis.Client

// Init 初始化 Redis 连接
func Init(cfg *config.Redis) error {
	client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// 测试连接
	_, err := client.Ping().Result() // 旧版 Ping 不接受 context
	if err != nil {
		logger.Error("Failed to connect to redis", zap.Error(err))
		return fmt.Errorf("connect redis failed, err: %v", err)
	}
	logger.Info("Redis connected successfully", zap.String("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)))
	return nil
}

// Close 关闭 Redis 连接
func Close() {
	if client != nil {
		if err := client.Close(); err != nil {
			logger.Error("Failed to close redis", zap.Error(err))
		}
	}
}

// Client 获取 Redis 客户端
func Client() *redis.Client {
	return client
}
