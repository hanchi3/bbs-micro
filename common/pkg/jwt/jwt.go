package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type MyClaims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	jwt.StandardClaims
}

var mySecret = []byte("bluebell-plus")

func keyFunc(_ *jwt.Token) (interface{}, error) {
	return mySecret, nil
}

const TokenExpireDuration = time.Hour * 24
const AccessTokenExpireDuration = time.Hour * 24 * 1000
const RefreshTokenExpireDuration = time.Hour * 24 * 1000

func GenToken(userID uint64, username string) (aToken, rToken string, err error) {
	c := MyClaims{
		userID,
		username,
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(AccessTokenExpireDuration).Unix(),
			Issuer:    "bluebell-plus",
		},
	}
	aToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(mySecret)
	if err != nil {
		return "", "", err
	}

	rToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		ExpiresAt: time.Now().Add(RefreshTokenExpireDuration).Unix(),
		Issuer:    "bluebell-plus",
	}).SignedString(mySecret)
	return
}

func ParseToken(tokenString string) (claims *MyClaims, err error) {
	claims = new(MyClaims)
	token, err := jwt.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func RefreshToken(aToken, rToken string) (newAToken, newRToken string, err error) {
	if aToken == "" {
		return "", "", fmt.Errorf("access token is empty")
	}
	if rToken == "" {
		return "", "", fmt.Errorf("refresh token is empty")
	}

	// 验证 Refresh Token 是否有效
	if _, err = jwt.Parse(rToken, keyFunc); err != nil {
		return "", "", fmt.Errorf("invalid refresh token: %v", err)
	}

	// 解析 Access Token
	var claims MyClaims
	_, err = jwt.ParseWithClaims(aToken, &claims, keyFunc)
	if err != nil {
		// 检查是否是 ValidationError
		if v, ok := err.(*jwt.ValidationError); ok {
			if v.Errors&jwt.ValidationErrorExpired != 0 {
				// Access Token 过期，生成新 token
				return GenToken(claims.UserID, claims.Username)
			}
			return "", "", fmt.Errorf("access token validation error: %v", v.Error())
		}
		return "", "", fmt.Errorf("failed to parse access token: %v", err)
	}

	// 如果 Access Token 未过期，返回错误
	return "", "", fmt.Errorf("access token is still valid, no need to refresh")
}
