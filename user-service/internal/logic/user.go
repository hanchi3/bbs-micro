// user-service/internal/logic/user.go
package logic

import (
	"context"

	"bluebell_microservices/common/pkg/jwt"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/common/pkg/snowflake" // 导入公共包
	pb "bluebell_microservices/proto/user"
	"bluebell_microservices/user-service/internal/dao/mysql"
	"bluebell_microservices/user-service/internal/model"

	"errors"

	"go.uber.org/zap"
)

type UserLogic struct {
	userDao *mysql.UserDAO
}

func NewUserLogic() *UserLogic {
	return &UserLogic{
		userDao: mysql.NewUserDAO(),
	}
}

func (l *UserLogic) SignUp(ctx context.Context, req *pb.SignUpRequest) error {

	logger.Info("SignUp attempt", zap.String("username", req.Username))

	// 1、判断用户是否存在
	err := l.userDao.CheckUserExist(req.Username)
	if err == nil {
		// 用户已存在，返回错误
		logger.Warn("User already exists", zap.String("username", req.Username))
		return errors.New("用户已存在")
	}

	// 用户不存在，继续注册流程
	// 2、生成UID
	userId, err := snowflake.GetID()
	if err != nil {
		logger.Error("Failed to generate user ID", zap.Error(err))
		return err
	}

	// 3、构造用户实例
	user := &model.User{
		UserID:   userId,
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
		Gender:   req.Gender, // 将 proto 的枚举转换为 int8
	}
	// 4、保存用户信息
	err = l.userDao.Create(user)
	if err != nil {
		logger.Error("Failed to create user", zap.String("username", req.Username), zap.Error(err))
		return err
	}

	logger.Info("SignUp successful", zap.String("username", req.Username), zap.Uint64("user_id", userId))
	return nil
}

func (l *UserLogic) Login(ctx context.Context, req *pb.LoginRequest) (user *model.User, error error) {

	logger.Info("Login attempt", zap.String("username", req.Username))

	// 检查用户是否存在
	err := l.userDao.CheckUserExist(req.Username)
	if err != nil {
		logger.Warn("User does not exist", zap.String("username", req.Username), zap.Error(err))
		return nil, err
	}

	// 构造用户实例
	user = &model.User{
		Username: req.Username,
		Password: req.Password,
	}

	err = l.userDao.Select(user)
	if err != nil {
		logger.Error("Failed to select user", zap.String("username", req.Username), zap.Error(err))
		return nil, err
	}

	// 生成JWT
	accessToken, refreshToken, err := jwt.GenToken(user.UserID, user.Username)
	if err != nil {
		logger.Error("Failed to generate token", zap.String("username", req.Username), zap.Error(err))
		return nil, err
	}

	user.AccessToken = accessToken
	user.RefreshToken = refreshToken

	logger.Info("Login successful", zap.String("username", user.Username))

	return user, nil

}

func (l *UserLogic) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	traceID, _ := ctx.Value("trace_id").(string)
	logger.Info("RefreshToken attempt",
		zap.String("trace_id", traceID),
		zap.String("access_token", req.AccessToken),
		zap.String("refresh_token", req.RefreshToken))

	// 调用 jwt 包的刷新逻辑
	newAccessToken, newRefreshToken, err := jwt.RefreshToken(req.AccessToken, req.RefreshToken)
	if err != nil {
		logger.Warn("Failed to refresh token", zap.Error(err))
		return &pb.RefreshTokenResponse{
			Code: 1,
			Msg:  "刷新 token 失败: " + err.Error(),
		}, nil
	}

	logger.Info("Token refreshed successfully")
	return &pb.RefreshTokenResponse{
		Code:         0,
		Msg:          "刷新成功",
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}
