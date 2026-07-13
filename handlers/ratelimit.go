package handlers

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	requests map[int][]int64 // uid → 时间戳切片
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[int][]int64),
		limit:    limit,
		window:   window,
	}
}

// allow 检查是否允许请求通过
func (rl *rateLimiter) allow(uid int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - rl.window.Milliseconds()

	// 清理过期记录，同时获取当前窗口内的请求数
	records := rl.requests[uid]
	valid := records[:0]
	for _, t := range records {
		if t > cutoff {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[uid] = valid
		return false
	}

	rl.requests[uid] = append(valid, now)
	return true
}

// 预定义的限流器：验证码生成接口每秒最多 5 次
var codeGenLimiter = newRateLimiter(5, 1*time.Second)

// rateLimitMiddleware 限流中间件
// uidFromReq 是一个函数，从请求中提取 uid
func rateLimitMiddleware(rl *rateLimiter, uidFromReq func(r *http.Request) int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := uidFromReq(r)
		if uid > 0 && !rl.allow(uid) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, ApiResponse{
				Success: false,
				Error:   "请求过于频繁，请 1 秒后再试",
			})
			return
		}
		next(w, r)
	}
}
