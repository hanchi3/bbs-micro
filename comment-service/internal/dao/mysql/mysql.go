package mysql

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"bluebell_microservices/common/config"
	"bluebell_microservices/common/pkg/logger"
)

// 定义一个全局对象db
var db *sqlx.DB

// Init 初始化数据库连接
func Init(cfg *config.MySQL) (err error) {
	logger.Info("Initializing MySQL connection",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
		zap.String("username", cfg.Username))

	// 构造 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)
	// 建立数据库连接
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		logger.Error("Failed to connect to database", zap.Error(err))
		return fmt.Errorf("open mysql failed, err: %v", err)
	}

	logger.Info("Successfully connected to MySQL")

	// 设置最大连接数和最大空闲连接数
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Second * 30)

	// 测试数据库连接
	err = db.Ping()
	if err != nil {
		logger.Error("Failed to ping database", zap.Error(err))
		return fmt.Errorf("connect mysql failed, err: %v", err)
	}

	logger.Info("Successfully pinged MySQL database")
	return nil
}

// Close 关闭数据库连接
func Close() {
	if db != nil {
		_ = db.Close()
	}
}

// DB 获取数据库连接
func DB() *sqlx.DB {
	return db
}
