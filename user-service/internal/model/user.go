// user-service/internal/model/user.go
package model

// User 定义数据库用户模型
type User struct {
	UserID       uint64 `db:"user_id"`  // 用户ID
	Username     string `db:"username"` // 用户名
	Password     string `db:"password"` // 密码
	Email        string `db:"email"`    // 邮箱
	Gender       string `db:"gender"`   // 性别
	AccessToken  string
	RefreshToken string
}

type RegisterForm struct {
	UserName        string `json:"username" binding:"required"`  // 用户名
	Email           string `json:"email" binding:"required"`     // 邮箱
	Gender          int    `json:"gender" binding:"oneof=0 1 2"` // 性别 0:未知 1:男 2:女
	Password        string `json:"password" binding:"required"`  // 密码
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
}

type LoginForm struct {
	UserName string
	Password string
}
