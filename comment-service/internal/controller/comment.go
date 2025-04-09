package controller

import (
	"bluebell_microservices/comment-service/internal/logic"
	"bluebell_microservices/comment-service/internal/model"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/common/pkg/snowflake"
	pb "bluebell_microservices/proto/comment"
	"context"
	"time"

	"go.uber.org/zap"
)

type CommentController struct {
	pb.UnimplementedCommentServiceServer
	commentLogic *logic.CommentLogic
}

func NewCommentController() *CommentController {
	return &CommentController{
		commentLogic: logic.NewCommentLogic(),
	}
}

func (c *CommentController) CreateComment(ctx context.Context, req *pb.CreateCommentRequest) (*pb.CreateCommentResponse, error) {
	logger.Info("Received CreateComment request",
		zap.Uint64("post_id", req.PostId),
		zap.Uint64("author_id", req.AuthorId),
		zap.String("content", req.Content))

	var comment model.Comment

	// 生成评论ID
	commentID, err := snowflake.GetID()
	if err != nil {
		logger.Error("snowflake.GetID() failed", zap.Error(err))
		return nil, err
	}
	comment.CommentID = commentID

	// 从请求参数中获取其他字段
	comment.PostID = req.PostId
	comment.ParentID = req.ParentId
	comment.AuthorID = req.AuthorId
	comment.Content = req.Content
	comment.CreateTime = time.Now()

	logger.Info("Creating comment",
		zap.Uint64("comment_id", comment.CommentID),
		zap.Uint64("post_id", comment.PostID),
		zap.Uint64("author_id", comment.AuthorID))

	err = c.commentLogic.CreateComment(ctx, &comment)
	if err != nil {
		logger.Error("Failed to create comment", zap.Error(err))
		return nil, err
	}

	return &pb.CreateCommentResponse{
		Code:    0,
		Message: "创建评论成功",
	}, nil
}

func (c *CommentController) GetCommentList(ctx context.Context, req *pb.GetCommentListRequest) (*pb.GetCommentListResponse, error) {
	logger.Info("Received GetCommentList request",
		zap.Uint64("post_id", req.PostId))

	comments, err := c.commentLogic.GetCommentList(ctx, req.PostId)
	if err != nil {
		logger.Error("Failed to get comment list", zap.Error(err))
		return nil, err
	}

	// 转换评论列表
	pbComments := make([]*pb.Comment, len(comments))
	for i, comment := range comments {
		pbComments[i] = &pb.Comment{
			CommentId:  comment.CommentID,
			PostId:     comment.PostID,
			ParentId:   comment.ParentID,
			AuthorId:   comment.AuthorID,
			Content:    comment.Content,
			CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	return &pb.GetCommentListResponse{
		Code:     0,
		Message:  "获取评论列表成功",
		Comments: pbComments,
	}, nil
}
