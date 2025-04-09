package handler

import (
	"bluebell_microservices/bff/internal/middleware"
	"bluebell_microservices/common/pkg/kafka"
	"bluebell_microservices/common/pkg/logger"
	pb "bluebell_microservices/proto/post"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CreatePostHandler(client pb.PostServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 从上下文中获取 user_id
		userIDInterface, exists := c.Get(middleware.ContextUserIDKey)
		if !exists {
			logger.Error("User not logged in", zap.String("trace_id", traceID))
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "请先登录",
			})
			return
		}

		// 将 interface{} 转换为 uint64
		userID, ok := userIDInterface.(uint64)
		if !ok {
			logger.Error("Invalid user_id type",
				zap.String("trace_id", traceID),
				zap.Any("user_id", userIDInterface))
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "用户ID类型错误",
			})
			return
		}

		// 1、获取参数及校验参数
		var req struct {
			CommunityID int64  `json:"community_id" binding:"required"`
			Title       string `json:"title" binding:"required"`
			Content     string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error("Invalid request parameters",
				zap.String("trace_id", traceID),
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 2、构造 gRPC 请求
		grpcReq := &pb.CreatePostRequest{
			CommunityId: req.CommunityID,
			Title:       req.Title,
			Content:     req.Content,
			AuthorId:    int64(userID), // 将 uint64 转换为 int64
		}

		logger.Info("Calling post-service CreatePost",
			zap.String("trace_id", traceID),
			zap.Uint64("user_id", userID),
			zap.Int64("community_id", req.CommunityID),
			zap.String("title", req.Title))

		// 3、调用 gRPC 服务
		resp, err := client.CreatePost(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call post-service",
				zap.String("trace_id", traceID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 4、处理响应
		switch resp.Code {
		case 0:
			logger.Info("CreatePost successful",
				zap.String("trace_id", traceID),
				zap.Uint64("user_id", userID))
			c.JSON(http.StatusOK, gin.H{
				"code":    resp.Code,
				"message": resp.Msg,
			})
		default:
			logger.Warn("CreatePost failed",
				zap.String("trace_id", traceID),
				zap.String("msg", resp.Msg))
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}
	}
}

func PostDetailHandler(client pb.PostServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 1、获取参数(从URL中获取帖子的id)
		postIdStr := c.Param("id")
		postId, err := strconv.ParseInt(postIdStr, 10, 64)
		if err != nil {
			logger.Error("Invalid post ID", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "帖子ID格式错误"})
			return
		}

		// 2、调用gRPC服务查询帖子详情
		grpcReq := &pb.GetPostByIdRequest{
			PostId: postId,
		}

		logger.Info("Calling post-service GetPostById",
			zap.String("trace_id", traceID),
			zap.Int64("post_id", postId))

		// 3、调用gRPC服务
		resp, err := client.GetPostById(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call post-service", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 根据业务码处理响应
		switch resp.Code {
		case 0:
			logger.Info("GetPostById successful", zap.String("trace_id", traceID))
			c.JSON(http.StatusOK, gin.H{
				"code":    resp.Code,
				"message": resp.Msg,
				"data":    resp.Post,
			})
		default:
			logger.Warn("GetPostById failed", zap.String("trace_id", traceID), zap.String("msg", resp.Msg))
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}
	}
}

func GetPostListHandler(client pb.PostServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {

		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 前端请求结构体
		var req struct {
			Search      string `form:"search"` // 改用 form tag
			CommunityId int64  `form:"community_id"`
			Page        int64  `form:"page"`
			Size        int64  `form:"size"`
			Order       string `form:"order"`
		}

		// 改用 ShouldBindQuery 来绑定 URL 查询参数
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warn("Invalid request", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 设置默认值
		if req.Page == 0 {
			req.Page = 1
		}
		if req.Size == 0 {
			req.Size = 10
		}
		if req.Order == "" {
			req.Order = "time"
		}

		// 构造 gRPC 请求
		grpcReq := &pb.GetPostListRequest{
			Search:      req.Search,
			CommunityId: req.CommunityId,
			Page:        req.Page,
			Size:        req.Size,
			Order:       req.Order,
		}

		logger.Info("Calling post-service GetPostList",
			zap.String("trace_id", traceID),
			zap.Any("request", grpcReq))

		// 调用 gRPC 服务
		resp, err := client.GetPostList(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call post-service", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 根据业务码处理响应
		switch resp.Code {
		case 0:
			logger.Info("GetPostList successful", zap.String("trace_id", traceID))
			c.JSON(http.StatusOK, gin.H{
				"code":    resp.Code, // 映射为目标 JSON 的成功码
				"message": resp.Msg,  // 直接使用 gRPC 的消息
				"data": gin.H{ // 构造 data 字段
					"page": gin.H{
						"total": resp.Page.Total,
						"page":  resp.Page.Page,
						"size":  resp.Page.Size,
					},
					"list": resp.Posts, // 直接使用 posts，Gin 会自动序列化为 JSON，字段名由 proto 标签决定
				},
			})
		default:
			logger.Warn("GetPostList failed", zap.String("trace_id", traceID), zap.String("msg", resp.Msg))
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}

	}
}

func PostSearchHandler(client pb.PostServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {

		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 前端请求结构体
		var req struct {
			Search      string `form:"search"` // 改用 form tag
			CommunityId int64  `form:"community_id"`
			Page        int64  `form:"page"`
			Size        int64  `form:"size"`
			Order       string `form:"order"`
		}

		// 改用 ShouldBindQuery 来绑定 URL 查询参数
		if err := c.ShouldBindQuery(&req); err != nil {
			logger.Warn("Invalid request", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 设置默认值
		if req.Page == 0 {
			req.Page = 1
		}
		if req.Size == 0 {
			req.Size = 10
		}
		if req.Order == "" {
			req.Order = "time"
		}

		// 构造 gRPC 请求
		grpcReq := &pb.SearchPostsRequest{
			Search:      req.Search,
			CommunityId: req.CommunityId,
			Page:        req.Page,
			Size:        req.Size,
			Order:       req.Order,
		}

		logger.Info("Calling post-service SearchPosts",
			zap.String("trace_id", traceID),
			zap.Any("request", grpcReq))

		// 调用 gRPC 服务
		resp, err := client.SearchPosts(c.Request.Context(), grpcReq)
		if err != nil {
			logger.Error("Failed to call post-service", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 根据业务码处理响应
		switch resp.Code {
		case 0:
			logger.Info("SearchPosts successful", zap.String("trace_id", traceID))
			c.JSON(http.StatusOK, gin.H{
				"code":    resp.Code, // 映射为目标 JSON 的成功码
				"message": resp.Msg,  // 直接使用 gRPC 的消息
				"data": gin.H{ // 构造 data 字段
					"page": gin.H{
						"total": resp.Page.Total,
						"page":  resp.Page.Page,
						"size":  resp.Page.Size,
					},
					"list": resp.Posts, // 直接使用 posts，Gin 会自动序列化为 JSON，字段名由 proto 标签决定
				},
			})
		default:
			logger.Warn("SearchPosts failed", zap.String("trace_id", traceID), zap.String("msg", resp.Msg))
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Msg})
		}
	}
}

func VoteHandler(client pb.PostServiceClient, kafkaProducer *kafka.Producer) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 从上下文中获取 user_id
		userIDInterface, exists := c.Get(middleware.ContextUserIDKey)
		if !exists {
			logger.Error("User not logged in", zap.String("trace_id", traceID))
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "请先登录",
			})
			return
		}

		// 将 interface{} 转换为 uint64
		userID, ok := userIDInterface.(uint64)
		if !ok {
			logger.Error("Invalid user_id type",
				zap.String("trace_id", traceID),
				zap.Any("user_id", userIDInterface))
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "用户ID类型错误",
			})
			return
		}

		// 1、获取参数
		var req struct {
			PostID    int64 `json:"post_id" binding:"required"`
			Direction int64 `json:"direction" binding:"required,oneof=1 0 -1"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error("Invalid request parameters", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 2、构造投票消息
		voteMsg := kafka.VoteMessage{
			PostID:    req.PostID,
			UserID:    int64(userID),
			Direction: req.Direction,
			Timestamp: time.Now().Unix(),
		}

		// 3、发送到Kafka
		err := kafkaProducer.SendVoteMessage(voteMsg)
		if err != nil {
			logger.Error("Failed to send vote message to Kafka",
				zap.String("trace_id", traceID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "投票请求处理失败，请稍后重试"})
			return
		}

		// 4、返回成功响应
		logger.Info("Vote request sent to Kafka",
			zap.String("trace_id", traceID),
			zap.Int64("post_id", req.PostID),
			zap.Int64("direction", req.Direction))
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "投票请求已接收，正在处理中",
		})
	}
}
