// user-service/internal/dao/mysql/user.go
package mysql

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"

	"bluebell_microservices/user-service/internal/model"
)

const secret = "huchao.vip"

type UserDAO struct {
	db *sql.DB
}

func NewUserDAO() *UserDAO {
	return &UserDAO{
		db: db, // 使用包级变量 db
	}
}

// encryptPassword 对密码进行加密
func encryptPassword(data []byte) (result string) {
	h := md5.New()
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(data))
}

// CheckUserExist 检查指定用户名的用户是否存在
func (d *UserDAO) CheckUserExist(username string) error {
	sqlStr := `select count(user_id) from user where username = ?`
	var count int
	err := d.db.QueryRow(sqlStr, username).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("用户不存在")
	}
	return nil // 用户存在时返回 nil
}

// Create 创建用户
func (d *UserDAO) Create(user *model.User) error {
	// 对密码进行加密
	user.Password = encryptPassword([]byte(user.Password))

	// 执行SQL语句入库
	sqlStr := `insert into user(user_id,username,password,email,gender) values(?,?,?,?,?)`
	_, err := d.db.Exec(sqlStr, user.UserID, user.Username, user.Password, user.Email, user.Gender)
	return err
}

func (d *UserDAO) Select(user *model.User) error {
	originPassword := user.Password
	sqlStr := "select user_id, username, password from user where username = ?"
	err := d.db.QueryRow(sqlStr, user.Username).Scan(&user.UserID, &user.Username, &user.Password)
	if err != nil {
		return err
	}

	password := encryptPassword([]byte(originPassword))
	if user.Password != password {
		return err
	}
	return nil
}
