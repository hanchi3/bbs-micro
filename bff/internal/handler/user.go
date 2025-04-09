package handler

import (
	"bluebell_microservices/common/pkg/logger"
	pb "bluebell_microservices/proto/user"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UserServiceClient 是 gRPC 客户端的接口，封装了与 gRPC 服务端通信的逻辑 conn
func SignUpHandler(client pb.UserServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {

		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 构造前端请求结构体，利用gin提供的标签约束
		var req struct {
			Username        string `json:"username" binding:"required"`
			Email           string `json:"email" binding:"required,email"`
			Gender          int    `json:"gender" binding:"oneof=0 1 2"` // 性别 0:未知 1:男 2:女
			Password        string `json:"password" binding:"required"`
			ConfirmPassword string `json:"confirm_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Warn("Invalid request", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		genderStr := strconv.Itoa(req.Gender)

		// 构造 gRPC 请求
		grpcReq := &pb.SignUpRequest{
			Username:        req.Username,
			Email:           req.Email,
			Gender:          genderStr,
			Password:        req.Password,
			ConfirmPassword: req.ConfirmPassword,
		}
		logger.Info("Calling user-service SignUp", zap.String("trace_id", traceID), zap.String("username", req.Username))

		// 调用 gRPC 服务
		resp, err := client.SignUp(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call user-service", zap.String("trace_id", traceID), zap.String("username", req.Username), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 根据业务码处理响应
		switch resp.Code {
		case 0:
			logger.Info("SignUp successful", zap.String("trace_id", traceID), zap.String("username", req.Username))
			c.JSON(http.StatusOK, gin.H{"msg": resp.Msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}
	}
}

func LoginHandler(client pb.UserServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {

		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 前端请求结构体
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Warn("Invalid request", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 构造 gRPC 请求
		grpcReq := &pb.LoginRequest{
			Username: req.Username,
			Password: req.Password,
		}
		logger.Info("Calling user-service Login", zap.String("trace_id", traceID), zap.String("username", req.Username))

		// 调用 gRPC 服务
		resp, err := client.Login(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call user-service", zap.String("trace_id", traceID), zap.String("username", req.Username), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 根据业务码处理响应
		switch resp.Code {
		case 0:
			logger.Info("Login successful", zap.String("trace_id", traceID), zap.String("username", req.Username))
			c.JSON(http.StatusOK, gin.H{
				"code":          resp.Code,
				"msg":           resp.Msg,
				"user_id":       fmt.Sprintf("%d", resp.UserId),
				"user_name":     resp.Username,
				"access_token":  resp.AccessToken,
				"refresh_token": resp.RefreshToken,
			})
		default:
			logger.Warn("Login failed", zap.String("trace_id", traceID), zap.String("username", req.Username), zap.String("msg", resp.Msg))
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}
	}
}

func RefreshTokenHandler(client pb.UserServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {

		// 获取 trace_id（与 LoggerMiddleware 配合）
		traceID := c.GetString("trace_id")

		// 前端请求结构体
		refreshToken := c.Query("refresh_token")
		if refreshToken == "" {
			logger.Warn("Missing refresh_token", zap.String("trace_id", traceID))
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少 refresh_token 参数"})
			c.Abort()
			return
		}

		// 从 Header 获取 access_token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("Missing Authorization header", zap.String("trace_id", traceID))
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "请求头缺少 Authorization"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			logger.Warn("Invalid Authorization format", zap.String("trace_id", traceID), zap.String("header", authHeader))
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Authorization 格式错误，应为 Bearer <token>"})
			c.Abort()
			return
		}
		accessToken := parts[1]

		// 构造 rpc 请求结构体
		grpcReq := &pb.RefreshTokenRequest{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}
		logger.Info("Calling user-service RefreshToken", zap.String("trace_id", traceID))

		// 调用 gRPC 服务
		ctx := context.WithValue(c.Request.Context(), "trace_id", traceID)
		resp, err := client.RefreshToken(ctx, grpcReq)
		if err != nil {
			logger.Error("Failed to call RefreshToken", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}

		// 处理响应
		if resp.Code != 0 {
			logger.Warn("Refresh token failed", zap.String("trace_id", traceID), zap.String("msg", resp.Msg))
			c.JSON(http.StatusUnauthorized, gin.H{"code": int(resp.Code), "msg": resp.Msg})
			return
		}

		logger.Info("Token refreshed successfully", zap.String("trace_id", traceID))
		c.JSON(http.StatusOK, gin.H{
			"access_token":  resp.AccessToken,
			"refresh_token": resp.RefreshToken,
		})

	}
}
