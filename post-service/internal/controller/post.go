package controller

import (
	"context"
	"time"

	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/common/pkg/snowflake"
	"bluebell_microservices/post-service/internal/logic"
	"bluebell_microservices/post-service/internal/model"
	pb "bluebell_microservices/proto/post"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PostController struct {
	pb.UnimplementedPostServiceServer
	postLogic *logic.PostLogic
}

func NewPostController() (*PostController, error) {
	postLogic, err := logic.NewPostLogic()
	if err != nil {
		logger.Error("Failed to create post logic", zap.Error(err))
		return nil, err
	}

	return &PostController{
		postLogic: postLogic,
	}, nil
}

func (c *PostController) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.CreatePostResponse, error) {
	logger.Info("Received CreatePost request",
		zap.Int64("author_id", req.AuthorId),
		zap.Int64("community_id", req.CommunityId),
		zap.String("title", req.Title))

	// 生成帖子ID
	postID, err := snowflake.GetID()
	if err != nil {
		logger.Error("Failed to generate post ID", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to generate post ID: %v", err)
	}

	// 确保 author_id 不为 0
	if req.AuthorId == 0 {
		logger.Error("Invalid author_id", zap.Int64("author_id", req.AuthorId))
		return nil, status.Errorf(codes.InvalidArgument, "invalid author_id: %d", req.AuthorId)
	}

	post := &model.Post{
		PostID:      postID,
		Title:       req.Title,
		Content:     req.Content,
		AuthorId:    uint64(req.AuthorId), // 确保正确转换类型
		CommunityID: uint64(req.CommunityId),
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	}

	logger.Info("Creating post",
		zap.Uint64("post_id", post.PostID),
		zap.Uint64("author_id", post.AuthorId),
		zap.Uint64("community_id", post.CommunityID))

	err = c.postLogic.CreatePost(ctx, post)
	if err != nil {
		logger.Error("Failed to create post", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to create post: %v", err)
	}

	return &pb.CreatePostResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (c *PostController) GetPostById(ctx context.Context, req *pb.GetPostByIdRequest) (*pb.GetPostByIdResponse, error) {
	logger.Info("Received GetPostById request", zap.Int64("post_id", req.PostId))

	post, err := c.postLogic.GetPostById(ctx, req.PostId)
	if err != nil {
		logger.Error("GetPostById failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get post by id: %v", err)
	}

	return &pb.GetPostByIdResponse{
		Code: 0,
		Msg:  "success",
		Post: &pb.ApiPostDetail{
			Post: &pb.Post{
				PostId:      int64(post.Post.PostID),
				AuthorId:    int64(post.Post.AuthorId),
				CommunityId: int64(post.Post.CommunityID),
				Status:      post.Post.Status,
				Title:       post.Post.Title,
				Content:     post.Post.Content,
				CreateTime:  post.Post.CreateTime.Format("2006-01-02 15:04:05"),
				UpdateTime:  post.Post.UpdateTime.Format("2006-01-02 15:04:05"),
			},
			Community: &pb.CommunityDetail{
				CommunityId:   int64(post.CommunityDetailRes.CommunityID),
				CommunityName: post.CommunityDetailRes.CommunityName,
				Introduction:  post.CommunityDetailRes.Introduction,
				CreateTime:    post.CommunityDetailRes.CreateTime,
			},
			AuthorName: post.AuthorName,
			VoteNum:    post.VoteNum,
		},
	}, nil
}

func (c *PostController) GetPostList(ctx context.Context, req *pb.GetPostListRequest) (*pb.GetPostListResponse, error) {
	logger.Info("Received GetPostList request",
		zap.String("search", req.Search),
		zap.Int64("page", req.Page),
		zap.Int64("size", req.Size),
		zap.String("order", req.Order),
		zap.Int64("community_id", req.CommunityId))

	param := &model.ParamPostList{
		Page:        req.Page,
		Size:        req.Size,
		Order:       req.Order,
		CommunityID: req.CommunityId,
		Search:      req.Search,
	}

	// 调用逻辑层获取帖子列表
	data, err := c.postLogic.GetPostListPre(ctx, param)
	if err != nil {
		logger.Error("GetPostList failed",
			zap.String("search", req.Search),
			zap.Int64("page", req.Page),
			zap.Int64("size", req.Size),
			zap.String("order", req.Order),
			zap.Int64("community_id", req.CommunityId),
			zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get post list: %v", err)
	}

	logger.Info("GetPostList success",
		zap.Int64("total", data.Page.Total),
		zap.Int64("page", data.Page.Page),
		zap.Int64("size", data.Page.Size),
		zap.Int("post_count", len(data.List)))
	// 构造 gRPC 响应
	return &pb.GetPostListResponse{
		Code: 0, // 成功时的 gRPC 内部码
		Msg:  "success",
		Page: &pb.Page{
			Total: data.Page.Total,
			Page:  data.Page.Page,
			Size:  data.Page.Size,
		},
		Posts: convertPostList(data.List),
	}, nil
}

func (c *PostController) SearchPosts(ctx context.Context, req *pb.SearchPostsRequest) (*pb.SearchPostsResponse, error) {
	logger.Info("Received SearchPosts request",
		zap.String("search", req.Search),
		zap.Int64("page", req.Page),
		zap.Int64("size", req.Size),
		zap.String("order", req.Order),
		zap.Int64("community_id", req.CommunityId))

	param := &model.ParamPostList{
		Page:        req.Page,
		Size:        req.Size,
		Order:       req.Order,
		CommunityID: req.CommunityId,
		Search:      req.Search,
	}

	data, err := c.postLogic.GetPostListPre(ctx, param)
	if err != nil {
		logger.Error("SearchPosts failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to search posts: %v", err)
	}

	return &pb.SearchPostsResponse{
		Code: 0,
		Msg:  "success",
		Page: &pb.Page{
			Total: data.Page.Total,
			Page:  data.Page.Page,
			Size:  data.Page.Size,
		},
		Posts: convertPostList(data.List),
	}, nil
}

// convertPostList 将 model.ApiPostDetail 列表转换为 pb.ApiPostDetail 列表
func convertPostList(posts []*model.ApiPostDetail) []*pb.ApiPostDetail {
	result := make([]*pb.ApiPostDetail, 0, len(posts))
	for _, postDetail := range posts {
		pbPost := &pb.ApiPostDetail{
			Post: &pb.Post{
				PostId:      int64(postDetail.Post.PostID),
				AuthorId:    int64(postDetail.Post.AuthorId),
				CommunityId: int64(postDetail.Post.CommunityID),
				Status:      postDetail.Post.Status,
				Title:       postDetail.Post.Title,
				Content:     postDetail.Post.Content,
				CreateTime:  postDetail.Post.CreateTime.Format("2006-01-02 15:04:05"),
				UpdateTime:  postDetail.Post.UpdateTime.Format("2006-01-02 15:04:05"),
			},
			Community: &pb.CommunityDetail{
				CommunityId:   int64(postDetail.CommunityDetailRes.CommunityID),
				CommunityName: postDetail.CommunityDetailRes.CommunityName,
				Introduction:  postDetail.CommunityDetailRes.Introduction,
				CreateTime:    postDetail.CommunityDetailRes.CreateTime,
			},
			AuthorName: postDetail.AuthorName,
			VoteNum:    postDetail.VoteNum,
		}
		result = append(result, pbPost)
	}
	return result
}

func (c *PostController) Vote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	logger.Info("Received Vote request",
		zap.Int64("post_id", req.PostId),
		zap.Int64("direction", req.Direction),
		zap.Int64("user_id", req.UserId))

	err := c.postLogic.Vote(ctx, req.PostId, req.Direction, req.UserId)
	if err != nil {
		logger.Error("Vote failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to vote: %v", err)
	}

	return &pb.VoteResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}
