package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	codeStep   = 150 // 验证码有效期 150 秒
	codeSkew   = 1   // 前后允许 1 个时间窗口的偏差
	codeLevel1 = 1   // 6 位数字
	codeLevel2 = 2   // 4 位数字
	codeLevel3 = 3   // 8 位数字+小写字母混合
	alphabet   = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// hmacHash 计算 HMAC-SHA1 原始值
func hmacHash(sharedSecret, msg string) []byte {
	mac := hmac.New(sha1.New, []byte(sharedSecret))
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

// generateCode 基于 TOTP 算法生成指定等级的验证码
// level: 1=6位数字, 2=4位数字, 3=8位数字+小写字母
func generateCode(sharedSecret, openid string, t time.Time, level int) string {
	counter := t.Unix() / codeStep
	msg := fmt.Sprintf("%s:%d:%d", openid, counter, level)
	hash := hmacHash(sharedSecret, msg)

	switch level {
	case codeLevel2:
		// 4 位数字
		offset := hash[len(hash)-1] & 0xf
		code := int(binary.BigEndian.Uint32(hash[offset:offset+4])) & 0x7fffffff
		return fmt.Sprintf("%04d", code%10000)

	case codeLevel3:
		// 8 位数字+小写字母混合
		result := make([]byte, 8)
		for i := 0; i < 8; i++ {
			idx := i * 2
			if idx+1 < len(hash) {
				result[i] = alphabet[int(hash[idx])%36]
			} else {
				result[i] = alphabet[int(hash[i])%36]
			}
		}
		return string(result)

	default:
		// level 1: 6 位数字（默认）
		offset := hash[len(hash)-1] & 0xf
		code := int(binary.BigEndian.Uint32(hash[offset:offset+4])) & 0x7fffffff
		return fmt.Sprintf("%06d", code%1000000)
	}
}

// verifyCode 验证 code 是否匹配 openid
func verifyCode(sharedSecret, openid, code string, level int) bool {
	now := time.Now()
	for skew := -codeSkew; skew <= codeSkew; skew++ {
		t := now.Add(time.Duration(skew) * codeStep * time.Second)
		if generateCode(sharedSecret, openid, t, level) == code {
			return true
		}
	}
	return false
}

// getExpireTime 返回当前时间窗口的过期时间戳（毫秒）
func getExpireTime(t time.Time) int64 {
	counter := t.Unix() / codeStep
	expireUnix := (counter + 1) * codeStep
	return expireUnix * 1000
}

// isValidCodeLevel 检查验证码等级是否合法
func isValidCodeLevel(level int) bool {
	return level >= codeLevel1 && level <= codeLevel3
}
