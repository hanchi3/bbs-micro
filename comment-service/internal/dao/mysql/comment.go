package mysql

import (
	"bluebell_microservices/comment-service/internal/model"
	"context"

	"github.com/jmoiron/sqlx"
)

type CommentDAO struct {
	db *sqlx.DB
}

func NewCommentDAO() *CommentDAO {
	return &CommentDAO{
		db: db,
	}
}

func (dao *CommentDAO) CreateComment(ctx context.Context, comment *model.Comment) error {
	sqlStr := `insert into comment(comment_id, content, post_id, author_id, parent_id, create_time)
    values(?,?,?,?,?,?)`
	_, err := dao.db.ExecContext(ctx, sqlStr, comment.CommentID, comment.Content, comment.PostID,
		comment.AuthorID, comment.ParentID, comment.CreateTime)
	return err
}

func (dao *CommentDAO) GetCommentList(ctx context.Context, postID uint64) ([]*model.Comment, error) {
	sqlStr := `select comment_id, content, post_id, author_id, parent_id, create_time
	from comment
	where post_id = ?
	order by create_time desc`
	var commentList []*model.Comment
	err := dao.db.SelectContext(ctx, &commentList, sqlStr, postID)
	if err != nil {
		return nil, err
	}
	return commentList, nil
}
