package jwt

import (
	"VMQ-api-go/internal/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT Token
func GenerateToken(userID, username, role string) (string, error) {
	var jwtSecret = []byte(config.AppConfig.JWT.Secret)
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24小时过期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gin-antd-login",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetTokenRemainingTime 获取 Token 的剩余有效期
func GetTokenRemainingTime(tokenString string) (time.Duration, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return 0, err
	}

	// 计算过期时间与当前时间的差值
	expiresAt := claims.ExpiresAt.Time
	remaining := time.Until(expiresAt)

	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

func GenerateAllTokens(userID, username, role string) (string, string, error) {
	secret := []byte(config.AppConfig.JWT.Secret)
	accessTokenTTL := config.AppConfig.JWT.AccessTokenTTL
	// 1. 生成 Access Token (短效)
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL * time.Minute)), // 15分钟
		},
	}).SignedString(secret)

	// 2. 生成 Refresh Token (长效)
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		UserID:    userID,
		Username:  username,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.AppConfig.JWT.RefreshTokenTTL) * time.Hour)),
		},
	}).SignedString(secret)

	return accessToken, refreshToken, err
}
