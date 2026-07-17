package handlers

import (
	"encoding/json"
	"io"
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
		// 从 URL 参数或 JSON body 获取 accesstoken
		accessToken := r.URL.Query().Get("accesstoken")
		
		// 如果 URL 参数中没有，尝试从 JSON body 中获取
		if accessToken == "" && r.Method == http.MethodPost {
			contentType := r.Header.Get("Content-Type")
			
			// 处理 multipart/form-data
			if strings.Contains(contentType, "multipart/form-data") {
				if err := r.ParseMultipartForm(32 << 20); err == nil {
					accessToken = r.FormValue("accesstoken")
					if accessToken != "" {
						log.Printf("从 form-data 获取到 accesstoken: %s", accessToken)
					}
				}
			} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
				// 处理 application/x-www-form-urlencoded
				if err := r.ParseForm(); err == nil {
					accessToken = r.FormValue("accesstoken")
					if accessToken != "" {
						log.Printf("从 form 获取到 accesstoken: %s", accessToken)
					}
				}
			} else {
				// 处理 application/json
			var bodyData map[string]interface{}
			if body, err := io.ReadAll(r.Body); err == nil {
				if err := json.Unmarshal(body, &bodyData); err == nil {
					if token, ok := bodyData["accesstoken"].(string); ok {
						accessToken = token
						log.Printf("从 JSON body 获取到 accesstoken: %s", token)
					}
				} else {
					log.Printf("JSON 解析失败: %v, body: %s", err, string(body))
				}
				// 重新填充 body 供后续处理使用
				r.Body = io.NopCloser(strings.NewReader(string(body)))
			}
			}
		}
		
		if accessToken == "" {
			log.Printf("❌ 鉴权失败: accesstoken 为空")
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
