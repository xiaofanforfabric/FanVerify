package handlers

import (
	"log"
	"net/http"
	"strings"
)

// CorsMiddleware CORS 中间件
func CorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// AccessTokenMiddleware OpenAPI 访问令牌鉴权中间件
func AccessTokenMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 URL 参数获取 accesstoken
		accessToken := r.URL.Query().Get("accesstoken")
		if accessToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			return
		}

		// 查询开发者令牌
		var devMode, devStatus, devBIP string
		err := db.QueryRow("SELECT mode, status, BIP FROM devdata WHERE devaccesstoken = ?", accessToken).Scan(&devMode, &devStatus, &devBIP)
		if err != nil {
			// 令牌不存在
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			log.Printf("❌ 鉴权失败: 令牌不存在 %s", accessToken)
			return
		}

		// 检查模式是否为 HTTP
		if devMode != "HTTP" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			log.Printf("❌ 鉴权失败: 模式不匹配, 期望 HTTP 但得到 %s", devMode)
			return
		}

		// 检查 IP 限制
		clientIP := r.RemoteAddr
		if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
			clientIP = clientIP[:idx] // 移除端口
		}

		if devBIP == "NULL" {
			// NULL 禁止所有 HTTP 访问
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			log.Printf("❌ 鉴权失败: BIP=NULL 禁止访问")
			return
		}

		if devBIP != "ALL" && devBIP != clientIP {
			// IP 不匹配
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			log.Printf("❌ 鉴权失败: IP 不匹配, 期望 %s 但得到 %s", devBIP, clientIP)
			return
		}

		// 检查状态
		if devStatus != "ok" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			log.Printf("❌ 鉴权失败: 令牌状态为 %s", devStatus)
			return
		}

		// 鉴权成功，记录验证次数
		db.Exec("UPDATE devdata SET NOV = NOV + 1 WHERE devaccesstoken = ?", accessToken)
		log.Printf("✅ 鉴权成功: %s, IP: %s", accessToken, clientIP)

		// 继续处理
		next(w, r)
	}
}
