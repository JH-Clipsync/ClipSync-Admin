package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CalcSign 计算请求签名。
//
// 待签串格式（用 \n 分隔）：
//
//	METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5
//
// 各字段说明：
//   - METHOD:    大写 HTTP 方法，如 GET / POST / PUT / DELETE
//   - PATH:      去掉 /api/admin 前缀的相对路径，如 /auth/me
//   - QUERY:     按 key 字典序排列的 k1=v1&k2=v2 串；无 query 时为空串
//   - TIMESTAMP: 毫秒时间戳字符串
//   - NONCE:     随机串（16 字节 hex）
//   - BODY_MD5:  请求体 MD5 hex；GET / 无 body 时为空字符串
//
// 签名 = HMAC-SHA256(secret, 待签串) 的 hex 小写。
func CalcSign(method, path, query, timestamp, nonce, bodyMD5, secret string) string {
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", method, path, query, timestamp, nonce, bodyMD5)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Equal 安全比较两个签名串（常数时间，防时序攻击）。
func Equal(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// GenSignSecret 生成 32 字节随机 hex 的签名密钥。
// 随登录会话动态下发，TTL 与 JWT 一致，登出即失效。
func GenSignSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// BuildSortedQuery 将 query 参数 map 按 key 字典序拼成 k1=v1&k2=v2 串。
// 多值参数取第一个值。空 map 返回空串。
// 前后端必须用同一规则，确保签名一致。
func BuildSortedQuery(params map[string][]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k, vs := range params {
		if len(vs) > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k][0])
	}
	return strings.Join(parts, "&")
}
