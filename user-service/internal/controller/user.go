// user-service/internal/controller/user.go
package controller

import (
	"context"
	"fmt"

	"bluebell_microservices/common/pkg/logger"
	pb "bluebell_microservices/proto/user"
	"bluebell_microservices/user-service/internal/logic"

	"go.uber.org/zap"
)

type UserController struct {
	pb.UnimplementedUserServiceServer
	userLogic *logic.UserLogic
}

func NewUserController() *UserController {
	return &UserController{
		userLogic: logic.NewUserLogic(),
	}
}

func (c *UserController) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.SignUpResponse, error) {

	// req 直接包含了所有参数，不需要从 context 中获取
	if req.Password != req.ConfirmPassword {
		return &pb.SignUpResponse{
			Code: 2,
			Msg:  "两次密码不一致",
		}, nil
	}

	// 实现注册逻辑
	if err := c.userLogic.SignUp(ctx, req); err != nil {
		return &pb.SignUpResponse{
			Code: 1,
			Msg:  "注册失败",
		}, nil
	}

	return &pb.SignUpResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (c *UserController) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {

	// req 直接包含了所有参数，不需要从 context 中获取

	// 调用逻辑层
	user, err := c.userLogic.Login(ctx, req)
	if err != nil {
		return &pb.LoginResponse{
			Code: 1,
			Msg:  "登陆失败",
		}, nil
	}

	// 构造 gRPC 响应
	return &pb.LoginResponse{
		Code:         0,
		Msg:          "登陆成功",
		UserId:       fmt.Sprintf("%d", user.UserID),
		Username:     user.Username,
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
	}, nil
}

func (c *UserController) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {

	traceID, _ := ctx.Value("trace_id").(string)
	logger.Info("RefreshToken request received", zap.String("trace_id", traceID))

	resp, err := c.userLogic.RefreshToken(ctx, req)
	if err != nil {
		logger.Error("RefreshToken failed in controller", zap.String("trace_id", traceID), zap.Error(err))
		return nil, err // 返回 gRPC 错误
	}

	return resp, nil
}
