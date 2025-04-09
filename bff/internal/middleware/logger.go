package middleware

import (
	"bluebell_microservices/common/pkg/logger"
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

/*
1.自定义中间件可以直接复用：在 common/pkg/logger 中定义的 zap 的全局实例（logger.Logger）
2.添加 trace_id 是分布式系统的常见需求。
*/

/*
API层和rpc层选择了日志的不同方法：
user-service：
1. 作为一个 gRPC 微服务，没有为它添加中间件，而是直接在代码中（例如 main.go 和 logic 层）使用 zap 记录日志。
2. user-service 是一个独立的服务，直接处理 Login 和 SignUp 等 RPC 调用，日志需求更倾向于业务逻辑的细节。
3. user-service 使用 gRPC 协议，而不是 HTTP，gRPC 有自己的拦截器（Interceptor）机制，没有为 user-service 添加 gRPC 拦截器，因为：当前需求简单，直接在业务代码中记录日志已足够。
BFF（API 服务）：
1. 为 BFF（基于 Gin 的 API 服务）添加了一个 Gin 中间件 LoggerMiddleware，用于记录 HTTP 请求的日志。
2. Gin 框架处理 HTTP 请求，HTTP 请求有明确的生命周期，中间件可以轻松捕获请求的开始和结束，适合记录整个请求的概况。
3. LoggerMiddleware() gin.HandlerFunc 统一记录所有 HTTP 请求的日志，Handler 中只记录特定业务逻辑。
*/

// LoggerMiddleware 创建日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()

		// 获取请求信息
		method := c.Request.Method
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// 生成或获取 trace_id
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = fmt.Sprintf("%d", time.Now().UnixNano()) // 简单生成，生产中可用 UUID
		}
		c.Set("trace_id", traceID) // 存入上下文，供 Handler 使用

		// 读取请求体（可选）
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			// 恢复请求体，供后续 Handler 使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// 处理请求
		c.Next()

		// 计算耗时
		latency := time.Since(start)

		// 获取响应状态
		status := c.Writer.Status()

		// 构造日志字段
		fields := []zap.Field{
			zap.String("trace_id", traceID),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", userAgent),
			zap.Int("status", status),
			zap.Duration("latency", latency),
		}

		// 可选：记录请求体
		if len(bodyBytes) > 0 {
			fields = append(fields, zap.String("body", string(bodyBytes)))
		}

		// 根据状态码记录不同级别的日志
		switch {
		case status >= 500:
			logger.Error("HTTP request failed", fields...)
		case status >= 400:
			logger.Warn("HTTP request client error", fields...)
		default:
			logger.Info("HTTP request completed", fields...)
		}
	}
}
