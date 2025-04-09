package model

import "time"

const (
	OrderTime  = "time"
	OrderScore = "score"
)

// ParamPostList 获取帖子列表query 参数
type ParamPostList struct {
	Search      string `json:"search" form:"search"`               // 关键字搜索
	CommunityID int64  `json:"community_id" form:"community_id"`   // 可以为空
	Page        int64  `json:"page" form:"page"`                   // 页码
	Size        int64  `json:"size" form:"size"`                   // 每页数量
	Order       string `json:"order" form:"order" example:"score"` // 排序依据
}

// ParamGithubTrending 获取Github热榜项目query 参数
type ParamGithubTrending struct {
	Language int   `json:"language" form:"language"` // 语言 0：All 1：Go 2：Python 3：JavaScript 4：Java
	Page     int64 `json:"page" form:"page"`         // 页码
	Size     int64 `json:"size" form:"size"`         // 每页数量
}

// Post 帖子Post结构体 内存对齐概念 字段类型相同的对齐 缩小变量所占内存大小
type Post struct {
	PostID      uint64    `json:"post_id,string" db:"post_id"`
	AuthorId    uint64    `json:"author_id" db:"author_id"`
	CommunityID uint64    `json:"community_id" db:"community_id" binding:"required"`
	Status      int32     `json:"status" db:"status"`
	Title       string    `json:"title" db:"title" binding:"required"`
	Content     string    `json:"content" db:"content" binding:"required"`
	CreateTime  time.Time `json:"-" db:"create_time"`
	UpdateTime  time.Time `json:"-" db:"update_time"`
}

// CommunityDetailRes 社区详情model
type CommunityDetailRes struct {
	CommunityID   uint64 `json:"community_id" db:"community_id"`
	CommunityName string `json:"community_name" db:"community_name"`
	Introduction  string `json:"introduction,omitempty" db:"introduction"` // omitempty 当Introduction为空时不展示
	CreateTime    string `json:"create_time" db:"create_time"`
}

type Page struct {
	Total int64 `json:"total"`
	Page  int64 `json:"page"`
	Size  int64 `json:"size"`
}

// ApiPostDetail 帖子返回的详情结构体
type ApiPostDetail struct {
	*Post                                  // 嵌入帖子结构体
	*CommunityDetailRes `json:"community"` // 嵌入社区信息
	AuthorName          string             `json:"author_name"`
	VoteNum             int64              `json:"vote_num"` // 投票数量
	//CommunityName string `json:"community_name"`
}

// ApiPostDetail 帖子返回的详情结构体
type ApiPostDetailRes struct {
	Page Page             `json:"page"`
	List []*ApiPostDetail `json:"list"`
}

// User 定义请求参数结构体
type User struct {
	UserID       uint64 `json:"user_id,string" db:"user_id"` // 指定json序列化/反序列化时使用小写user_id
	UserName     string `json:"username" db:"username"`
	Password     string `json:"password" db:"password"`
	Email        string `json:"email" db:"gender"`  // 邮箱
	Gender       int    `json:"gender" db:"gender"` // 性别
	AccessToken  string
	RefreshToken string
}
