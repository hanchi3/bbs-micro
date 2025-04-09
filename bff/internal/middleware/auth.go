package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"bluebell_microservices/common/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserIDKey = "userID"
)

// JWTAuthMiddleware 基于JWT的认证中间件
// 中间件 主要验证 Access Token 是否有效
func JWTAuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "请求头缺少Auth Token",
			})
			c.Abort()
			return
		}
		// 按空格分割
		//&& parts[0] == "Bearer"
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "Token格式不对",
			})
			c.Abort()
			return
		}
		// parts[1]是获取到的tokenString，我们使用之前定义好的解析JWT的函数来解析它
		mc, err := jwt.ParseToken(parts[1])
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "无效的Token",
			})
			c.Abort()
			return
		}
		// 将当前请求的userID信息保存到请求的上下文c上
		c.Set(ContextUserIDKey, mc.UserID)
		c.Next() // 后续的处理函数可以用过c.Get(ContextUserIDKey)来获取当前请求的用户信息
	}
}
