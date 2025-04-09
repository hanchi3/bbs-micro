package logic

import (
	"context"
	"strings"

	"bluebell_microservices/common/pkg/logger" // 导入公共包
	"bluebell_microservices/post-service/internal/dao/mysql"
	"bluebell_microservices/post-service/internal/dao/redis"
	"bluebell_microservices/post-service/internal/model"

	"go.uber.org/zap"
)

/*
依赖注入：
依赖注入的核心思想是将组件的依赖关系外部化（例如通过构造函数注入），而不是在组件内部直接创建依赖的实例。这样做有几个好处：

解耦：PostLogic 不再负责创建 PostDAO 的实例，外部（比如工厂方法 NewPostLogic）提供依赖。这样可以降低模块之间的耦合。

易于测试：通过依赖注入，可以在测试时轻松替换 PostDAO 为一个 mock 或 fake 实现，进行单元测试。

可扩展性：如果将来你想替换 PostDAO 的实现，只需要修改构造函数或 DI 配置，而不需要修改业务逻辑。
*/
type PostLogic struct {
	postDao *mysql.PostDAO
}

func NewPostLogic() *PostLogic {
	return &PostLogic{
		postDao: mysql.NewPostDAO(),
	}
}

func (l *PostLogic) CreatePost(ctx context.Context, post *model.Post) error {
	logger.Info("CreatePost attempt", zap.Any("post", post))

	// 2、创建帖子 保存到数据库
	if err := l.postDao.CreatePost(ctx, post); err != nil {
		zap.L().Error("mysql.CreatePost(&post) failed", zap.Error(err))
		return err
	}

	// 3、redis存储帖子信息
	if err := redis.CreatePost(
		post.PostID,
		post.AuthorId,
		post.Title,
		TruncateByWords(post.Content, 120),
		post.CommunityID); err != nil {
		zap.L().Error("redis.CreatePost failed", zap.Error(err))
		return err
	}
	return nil

}

func (l *PostLogic) GetPostList2(req *model.ParamPostList) (*model.ApiPostDetailRes, error) {
	logger.Info("GetPostList attempt",
		zap.String("Order", req.Order),
		zap.Int64("Page", req.Page),
		zap.Int64("Size", req.Size),
		zap.Int("CommunityID", int(req.CommunityID)),
		zap.String("Search", req.Search))

	// 从mysql获取总页数
	total, err := mysql.GetPostTotalCount(req.Search, req.CommunityID)
	if err != nil {
		logger.Warn("GetPostTotalCount failed", zap.Error(err))
		return nil, err
	}

	var resp model.ApiPostDetailRes
	resp.Page.Total = total
	resp.Page.Page = req.Page
	resp.Page.Size = req.Size
	// 初始化空列表，避免返回null
	resp.List = make([]*model.ApiPostDetail, 0)

	// 1、如果有搜索关键词，直接从MySQL获取匹配的帖子ID
	var ids []string
	if req.Search != "" {
		ids, err = mysql.GetPostIDsBySearch(req.Search, req.Page, req.Size, req.CommunityID)
	} else {
		// 如果没有搜索关键词，从Redis获取排序后的ID
		ids, err = redis.GetPostIDsInOrder(req)
	}

	if err != nil {
		logger.Error("Failed to get post IDs", zap.Error(err))
		return &resp, nil
	}

	if len(ids) == 0 {
		logger.Info("No posts found")
		return &resp, nil
	}

	// 2、提前查询好每篇帖子的投票数
	voteData, err := redis.GetPostVoteData(ids)
	if err != nil {
		logger.Warn("redis.GetPostVoteData(ids) failed", zap.Error(err))
		return nil, err
	}

	// 3、根据id去数据库查询帖子详细信息
	posts, err := mysql.GetPostListByIDs(ids)
	if err != nil {
		logger.Error("Failed to get posts from MySQL", zap.Error(err))
		return nil, err
	}

	// 4、组合数据
	for idx, post := range posts {
		logger.Info("Processing post",
			zap.Uint64("post_id", post.PostID),
			zap.Uint64("author_id", post.AuthorId),
			zap.Uint64("community_id", post.CommunityID))

		// 根据作者id查询作者信息
		user, err := mysql.GetUserByID(post.AuthorId)
		if err != nil {
			logger.Error("mysql.GetUserByID() failed",
				zap.Uint64("author_id", post.AuthorId),
				zap.Error(err))
			continue // 跳过这条数据，继续处理下一条
		}
		// 根据社区id查询社区详细信息
		community, err := mysql.GetCommunityByID(post.CommunityID)
		if err != nil {
			logger.Error("mysql.GetCommunityByID() failed",
				zap.Uint64("community_id", post.CommunityID),
				zap.Error(err))
			continue // 跳过这条数据，继续处理下一条
		}
		// 接口数据拼接
		postDetail := &model.ApiPostDetail{
			VoteNum:            voteData[idx],
			Post:               post,
			CommunityDetailRes: community,
			AuthorName:         user.UserName,
		}
		resp.List = append(resp.List, postDetail)
	}
	return &resp, nil
}

// GetCommunityPostList 根据社区id去查询帖子列表
func (l *PostLogic) GetCommunityPostList(p *model.ParamPostList) (*model.ApiPostDetailRes, error) {
	var res model.ApiPostDetailRes
	// 从mysql获取该社区下帖子列表总数
	total, err := mysql.GetCommunityPostTotalCount(uint64(p.CommunityID))
	if err != nil {
		logger.Error("GetCommunityPostTotalCount failed", zap.Error(err))
		return nil, err
	}
	res.Page.Total = total
	// 1、根据参数中的排序规则去redis查询id列表
	ids, err := redis.GetCommunityPostIDsInOrder(p)
	if err != nil {
		logger.Error("GetCommunityPostIDsInOrder failed", zap.Error(err))
		return nil, err
	}
	if len(ids) == 0 {
		logger.Info("No posts found in Redis")
		return &res, nil
	}
	zap.L().Debug("GetPostList2", zap.Any("ids", ids))
	// 2、提前查询好每篇帖子的投票数
	voteData, err := redis.GetPostVoteData(ids)
	if err != nil {
		logger.Error("GetPostVoteData failed", zap.Error(err))
		return nil, err
	}
	// 3、根据id去数据库查询帖子详细信息
	// 返回的数据还要按照我给定的id的顺序返回  order by FIND_IN_SET(post_id, ?)
	posts, err := mysql.GetPostListByIDs(ids)
	if err != nil {
		logger.Error("GetPostListByIDs failed", zap.Error(err))
		return nil, err
	}
	res.Page.Page = p.Page
	res.Page.Size = p.Size
	res.List = make([]*model.ApiPostDetail, 0, len(posts))
	// 4、根据社区id查询社区详细信息
	// 为了减少数据库的查询次数，这里将社区信息提前查询出来
	community, err := mysql.GetCommunityByID(uint64(p.CommunityID))
	if err != nil {
		logger.Error("mysql.GetCommunityByID() failed",
			zap.Uint64("community_id", uint64(p.CommunityID)),
			zap.Error(err))
		community = nil
	}
	for idx, post := range posts {
		// 过滤掉不属于该社区的帖子
		if post.CommunityID != uint64(p.CommunityID) {
			continue
		}

		// 如果有搜索关键词，检查帖子是否匹配
		if p.Search != "" {
			matched := strings.Contains(strings.ToLower(post.Title), strings.ToLower(p.Search)) ||
				strings.Contains(strings.ToLower(post.Content), strings.ToLower(p.Search))
			if !matched {
				continue
			}
		}

		// 根据作者id查询作者信息
		user, err := mysql.GetUserByID(post.AuthorId)
		if err != nil {
			logger.Error("mysql.GetUserByID() failed",
				zap.Uint64("postID", post.AuthorId),
				zap.Error(err))
			user = nil
		}
		// 接口数据拼接
		postDetail := &model.ApiPostDetail{
			VoteNum:            voteData[idx],
			Post:               post,
			CommunityDetailRes: community,
			AuthorName:         user.UserName,
		}
		res.List = append(res.List, postDetail)
	}
	return &res, nil
}

func (l *PostLogic) GetPostListPre(ctx context.Context, req *model.ParamPostList) (*model.ApiPostDetailRes, error) {
	params := &model.ParamPostList{
		Page:        req.Page,
		Size:        req.Size,
		Order:       req.Order,
		CommunityID: req.CommunityID,
		Search:      req.Search,
	}

	logger.Info("GetPostListPre called",
		zap.String("search", params.Search),
		zap.Int64("page", params.Page),
		zap.Int64("size", params.Size),
		zap.String("order", params.Order),
		zap.Int64("community_id", params.CommunityID))

	// 根据请求参数的不同,执行不同的业务逻辑
	if params.CommunityID == 0 {
		// 查询所有帖子
		return l.GetPostList2(params)
	} else {
		// 查询指定社区的帖子
		return l.GetCommunityPostList(params)
	}
}

func (l *PostLogic) GetPostById(ctx context.Context, id int64) (*model.ApiPostDetail, error) {
	// 查询帖子信息
	post, err := l.postDao.GetPostByID(id)
	if err != nil {
		logger.Error("mysql.GetPostByID(postID) failed",
			zap.Int64("postID", id),
			zap.Error(err))
		return nil, err
	}

	// 根据作者id查询作者信息
	user, err := mysql.GetUserByID(post.AuthorId)
	if err != nil {
		logger.Error("mysql.GetUserByID() failed",
			zap.Uint64("postID", post.AuthorId),
			zap.Error(err))
		return nil, err
	}
	// 根据社区id查询社区详细信息
	community, err := mysql.GetCommunityByID(post.CommunityID)
	if err != nil {
		logger.Error("mysql.GetCommunityByID() failed",
			zap.Uint64("community_id", post.CommunityID),
			zap.Error(err))
		return nil, err
	}
	// 根据帖子id查询帖子的投票数
	voteNum, err := redis.GetPostVoteNum(id)
	if err != nil {
		logger.Error("redis.GetPostVoteNum failed", zap.Error(err))
		return nil, err
	}

	// 接口数据拼接
	data := &model.ApiPostDetail{
		Post:               post,
		CommunityDetailRes: community,
		AuthorName:         user.UserName,
		VoteNum:            voteNum,
	}
	return data, nil

}

func (l *PostLogic) Vote(ctx context.Context, postID int64, direction int64, userID int64) error {
	logger.Info("Vote called",
		zap.Int64("post_id", postID),
		zap.Int64("direction", direction),
		zap.Int64("user_id", userID))

	// 4、记录用户投票
	return redis.CreatePostVote(postID, userID, direction)
}
