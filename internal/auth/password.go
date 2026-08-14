package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

// ===== 管理员密码（bcrypt） =====

// HashPassword 用 bcrypt 生成管理员密码哈希。
func HashPassword(plain string, cost int) (string, error) {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验管理员 bcrypt 密码。
func CheckPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// ===== ClipSync 业务用户密码（scrypt） =====
//
// 存储格式（与 ClipSync-Server 完全兼容，单字段自带算法参数）：
//
//	scrypt$N$r$p$<base64 salt>$<base64 dk>
//
// 参数：N=32768(1<<15), r=8, p=1, keyLen=32, salt 16 字节随机。
// 使用 RawStdEncoding（无填充）保持与 ClipSync-Server 一致。
const (
	scryptN      = 1 << 15 // 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 16
)

// HashUserPassword 生成带随机盐的 scrypt 哈希串，写入 users.password_hash。
func HashUserPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成盐失败: %w", err)
	}
	dk, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", fmt.Errorf("scrypt 计算失败: %w", err)
	}
	return fmt.Sprintf("scrypt$%d$%d$%d$%s$%s",
		scryptN, scryptR, scryptP,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// CheckUserPassword 用哈希串里记录的参数重算一遍，再做常数时间比较。
// 与 ClipSync-Server 的 VerifyPassword 行为完全一致。
func CheckUserPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "scrypt" {
		return false
	}
	n, err1 := strconv.Atoi(parts[1])
	r, err2 := strconv.Atoi(parts[2])
	p, err3 := strconv.Atoi(parts[3])
	salt, err4 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err5 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return false
	}
	got, err := scrypt.Key([]byte(password), salt, n, r, p, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
