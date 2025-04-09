package mysql

import (
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/post-service/internal/model"
	"context"
	"database/sql"
	"errors"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// PostDAO 帖子数据访问对象
type PostDAO struct {
	db *sqlx.DB
}

// NewPostDAO 创建新的 PostDAO 实例
func NewPostDAO() *PostDAO {
	return &PostDAO{
		db: db,
	}
}

func (p *PostDAO) CreatePost(ctx context.Context, post *model.Post) error {
	sqlStr := `
		INSERT INTO post (post_id, title, content, author_id, community_id, create_time, update_time)
		VALUES (:post_id, :title, :content, :author_id, :community_id, :create_time, :update_time)
	`
	_, err := db.NamedExec(sqlStr, post)
	return err
}

func (p *PostDAO) GetCommunityNameByID(param any) (any, error) {
	community := new(model.CommunityDetailRes)
	sqlStr := `select community_id, community_name
	from community
	where community_id = ?`
	err := db.Get(community, sqlStr, param)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("无效的ID")

		}
		zap.L().Error("query community failed", zap.String("sql", sqlStr), zap.Error(err))
		return nil, errors.New("查询失败")
	}
	return community, nil
}

func GetPostTotalCount(search string, communityID int64) (int64, error) {
	var sqlStr string
	var args []interface{}

	if search != "" {
		if communityID > 0 {
			sqlStr = `
				SELECT COUNT(*) FROM post 
				WHERE (title LIKE ? OR content LIKE ?)
				AND community_id = ?
			`
			searchPattern := "%" + search + "%"
			args = []interface{}{searchPattern, searchPattern, communityID}
		} else {
			sqlStr = `
				SELECT COUNT(*) FROM post 
				WHERE title LIKE ? OR content LIKE ?
			`
			searchPattern := "%" + search + "%"
			args = []interface{}{searchPattern, searchPattern}
		}
	} else {
		if communityID > 0 {
			sqlStr = `
				SELECT COUNT(*) FROM post 
				WHERE community_id = ?
			`
			args = []interface{}{communityID}
		} else {
			sqlStr = `
				SELECT COUNT(*) FROM post
			`
			args = nil
		}
	}

	var count int64
	var err error
	if args != nil {
		err = db.QueryRow(sqlStr, args...).Scan(&count)
	} else {
		err = db.QueryRow(sqlStr).Scan(&count)
	}
	return count, err
}

// GetPostListByIDs 根据给定的id列表查询帖子数据
func GetPostListByIDs(ids []string) (postList []*model.Post, err error) {
	sqlStr := `select post_id, title, content, author_id, community_id, create_time, update_time
	from post
	where post_id in (?)
	order by FIND_IN_SET(post_id, ?)`
	// 动态填充id
	query, args, err := sqlx.In(sqlStr, ids, strings.Join(ids, ","))
	if err != nil {
		return
	}
	// sqlx.In 返回带 `?` bindvar的查询语句, 我们使用Rebind()重新绑定它
	query = db.Rebind(query)
	err = db.Select(&postList, query, args...)
	return
}

// GetUserByID 根据ID查询作者信息
func GetUserByID(id uint64) (user *model.User, err error) {
	user = new(model.User)
	sqlStr := `select user_id, username from user where user_id = ?`
	err = db.Get(user, sqlStr, id)
	return
}

// GetCommunityByID 根据ID查询分类社区详情
func GetCommunityByID(id uint64) (*model.CommunityDetailRes, error) {
	community := new(model.CommunityDetailRes)
	sqlStr := `select community_id, community_name, introduction, create_time
	from community
	where community_id = ?`
	err := db.Get(community, sqlStr, id)
	if err != nil {
		if err == sql.ErrNoRows { // 查询为空
			return nil, errors.New("无效的ID") // 无效的ID return
		}
		zap.L().Error("query community failed", zap.String("sql", sqlStr), zap.Error(err))
		return nil, errors.New("查询失败")
	}
	return &model.CommunityDetailRes{
		CommunityID:   community.CommunityID,
		CommunityName: community.CommunityName,
		Introduction:  community.Introduction,
		CreateTime:    community.CreateTime,
	}, err
}

// GetCommunityPostTotalCount 根据社区id查询数据库帖子总数
func GetCommunityPostTotalCount(communityID uint64) (count int64, err error) {
	sqlStr := `select count(post_id) from post where community_id = ?`
	err = db.Get(&count, sqlStr, communityID)
	if err != nil {
		zap.L().Error("db.Get(&count, sqlStr) failed", zap.Error(err))
		return 0, err
	}
	return
}

// GetPostByID 根据帖子id查询帖子信息
func (p *PostDAO) GetPostByID(id int64) (*model.Post, error) {
	post := new(model.Post)
	sqlStr := `select post_id, title, content, author_id, community_id, create_time, update_time
	from post
	where post_id = ?`
	err := db.Get(post, sqlStr, id)
	return post, err
}

// GetPostIDsBySearch 根据搜索关键词获取匹配的帖子ID列表
func GetPostIDsBySearch(search string, page, size int64, communityID int64) ([]string, error) {
	var sqlStr string
	var args []interface{}

	if communityID > 0 {
		sqlStr = `
			SELECT post_id FROM post 
			WHERE (title LIKE ? OR content LIKE ?)
			AND community_id = ?
			ORDER BY create_time DESC
			LIMIT ? OFFSET ?
		`
		searchPattern := "%" + search + "%"
		offset := (page - 1) * size
		args = []interface{}{searchPattern, searchPattern, communityID, size, offset}
	} else {
		sqlStr = `
			SELECT post_id FROM post 
			WHERE title LIKE ? OR content LIKE ?
			ORDER BY create_time DESC
			LIMIT ? OFFSET ?
		`
		searchPattern := "%" + search + "%"
		offset := (page - 1) * size
		args = []interface{}{searchPattern, searchPattern, size, offset}
	}

	var ids []string
	err := db.Select(&ids, sqlStr, args...)
	if err != nil {
		logger.Error("Failed to get post IDs by search",
			zap.String("search", search),
			zap.Int64("page", page),
			zap.Int64("size", size),
			zap.Int64("community_id", communityID),
			zap.Error(err))
		return nil, err
	}

	logger.Info("Got post IDs by search",
		zap.String("search", search),
		zap.Int64("page", page),
		zap.Int64("size", size),
		zap.Int64("community_id", communityID),
		zap.Int("count", len(ids)),
		zap.Strings("ids", ids))

	return ids, nil
}
