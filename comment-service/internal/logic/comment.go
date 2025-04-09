package logic

import (
	"bluebell_microservices/comment-service/internal/dao/mysql"
	"bluebell_microservices/comment-service/internal/model"
	"bluebell_microservices/common/pkg/logger"
	"context"

	"go.uber.org/zap"
)

type CommentLogic struct {
	commentDao *mysql.CommentDAO
}

func NewCommentLogic() *CommentLogic {
	return &CommentLogic{
		commentDao: mysql.NewCommentDAO(),
	}
}

func (l *CommentLogic) CreateComment(ctx context.Context, comment *model.Comment) error {
	logger.Info("CreateComment attempt", zap.Any("comment", comment))

	// 保存到数据库
	if err := l.commentDao.CreateComment(ctx, comment); err != nil {
		logger.Error("Failed to create comment", zap.Error(err))
		return err
	}

	return nil
}

// GetCommentList 获取评论列表
func (l *CommentLogic) GetCommentList(ctx context.Context, postID uint64) ([]*model.Comment, error) {
	logger.Info("GetCommentList attempt", zap.Uint64("post_id", postID))

	comments, err := l.commentDao.GetCommentList(ctx, postID)
	if err != nil {
		logger.Error("Failed to get comment list", zap.Error(err))
		return nil, err
	}

	return comments, nil
}
