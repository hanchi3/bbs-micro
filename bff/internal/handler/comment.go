package handler

import (
	"bluebell_microservices/bff/internal/middleware"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/proto/comment"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CommentHandler(client comment.CommentServiceClient) gin.HandlerFunc {
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
		AuthorID, ok := userIDInterface.(uint64)
		if !ok {
			logger.Error("Invalid author_id type", zap.String("trace_id", traceID))
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "author_id 类型错误",
			})
			return
		}

		// 定义请求参数结构体
		var req struct {
			PostID   uint64 `json:"post_id" binding:"required"`
			ParentID uint64 `json:"parent_id"`
			Content  string `json:"content" binding:"required"`
		}

		// 获取并验证请求参数
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error("Invalid request parameters",
				zap.String("trace_id", traceID),
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "请求参数错误",
				"data": err.Error(),
			})
			return
		}

		// 参数验证
		if req.PostID == 0 {
			logger.Error("PostID is required", zap.String("trace_id", traceID))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "帖子ID不能为空",
			})
			return
		}

		if req.Content == "" {
			logger.Error("Content is required", zap.String("trace_id", traceID))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "评论内容不能为空",
			})
			return
		}

		// 组合创建评论请求
		createReq := &comment.CreateCommentRequest{
			PostId:   req.PostID,
			ParentId: req.ParentID,
			Content:  req.Content,
			AuthorId: AuthorID,
		}

		// 调用评论服务创建评论
		resp, err := client.CreateComment(c.Request.Context(), createReq)
		if err != nil {
			logger.Error("Failed to create comment",
				zap.String("trace_id", traceID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "创建评论失败",
				"data": err.Error(),
			})
			return
		}

		// 返回成功响应
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "创建评论成功",
			"data": resp,
		})
	}
}

func CommentListHandler(client comment.CommentServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetString("trace_id") // 从上下文获取 trace_id

		// 获取请求参数
		postIDStr := c.Query("post_id")
		if postIDStr == "" {
			logger.Error("PostID is required", zap.String("trace_id", traceID))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "帖子ID不能为空",
			})
			return
		}

		// 将 postIDStr 转换为 uint64
		postID, err := strconv.ParseUint(postIDStr, 10, 64)
		if err != nil {
			logger.Error("Invalid post_id", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "帖子ID格式错误",
			})
			return
		}

		// 调用评论服务获取评论列表
		resp, err := client.GetCommentList(c.Request.Context(), &comment.GetCommentListRequest{
			PostId: postID,
		})
		if err != nil {
			logger.Error("Failed to get comment list", zap.String("trace_id", traceID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "获取评论列表失败",
				"data": err.Error(),
			})
			return
		}

		// 返回成功响应
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "获取评论列表成功",
			"data": resp,
		})
	}
}
