package main

import (
	"bluebell_microservices/bff/internal/grpc_client"
	"bluebell_microservices/bff/internal/handler"
	"bluebell_microservices/bff/internal/middleware"
	"bluebell_microservices/common/pkg/logger"
	"flag"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupRouter() *gin.Engine {
	flag.Parse()

	// 初始化日志
	if err := logger.Init("info", "bff.log"); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer logger.Logger.Sync()

	// 初始化 gRPC 客户端
	clients, err := grpc_client.NewClients()
	if err != nil {
		logger.Error("Failed to connect to micro-service", zap.Error(err))
		log.Fatalf("Failed to initialize gRPC clients: %v", err)
	}

	// 设置 Gin
	r := gin.Default()
	r.Use(middleware.LoggerMiddleware()) // 使用日志中间件

	// 设置路由
	v1 := r.Group("/api/v1")
	//
	v1.POST("/signup", handler.SignUpHandler(clients.User))
	v1.POST("/login", handler.LoginHandler(clients.User))
	v1.GET("/refresh_token", handler.RefreshTokenHandler(clients.User))

	v1.GET("/posts2", handler.GetPostListHandler(clients.Post))
	v1.GET("/post/:id", handler.PostDetailHandler(clients.Post)) // 查询帖子详情
	v1.GET("/search", handler.PostSearchHandler(clients.Post))   // 搜索业务-搜索帖子

	// 中间件
	v1.Use(middleware.JWTAuthMiddleware()) // 应用JWT认证中间件
	{
		v1.POST("/post", handler.CreatePostHandler(clients.Post)) // 创建帖子
		v1.POST("/vote", handler.VoteHandler(clients.Post))       // 投票

		v1.POST("/comment", handler.CommentHandler(clients.Comment))    // 评论
		v1.GET("/comment", handler.CommentListHandler(clients.Comment)) // 评论列表

		v1.GET("/ping", func(c *gin.Context) {
			userID, exists := c.Get(middleware.ContextUserIDKey)
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code": 401,
					"msg":  "未登录",
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"code": 200,
				"msg":  "pong",
				"data": gin.H{
					"user_id": userID,
				},
			})
		})
	}

	return r
}

func main() {
	r := SetupRouter()
	if err := r.Run(":8080"); err != nil {
		logger.Error("Failed to run BFF", zap.Error(err))
		log.Fatalf("Failed to run server: %v", err)
	}
}
