package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// isAllowedOrigin 判断跨域来源是否被允许（fanverify.cn 主域名及所有子域名）
// 额外来源可通过环境变量 CORS_ALLOW_ORIGINS（逗号分隔）追加，供本地开发等场景使用
func isAllowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "fanverify.cn" || strings.HasSuffix(host, ".fanverify.cn") {
		return true
	}
	for _, extra := range strings.Split(os.Getenv("CORS_ALLOW_ORIGINS"), ",") {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		if eu, err := url.Parse(extra); err == nil && host == strings.ToLower(eu.Hostname()) {
			return true
		}
	}
	return false
}

// CorsMiddleware CORS 中间件：仅允许 fanverify.cn 主域名及子域名携带 Cookie 跨域访问
func CorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// 非浏览器请求（无 Origin，如 OpenAPI 服务端调用）：保持开放
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			next(w, r)
			return
		}
		// 浏览器跨域请求：仅放行 fanverify.cn 主域名及子域名
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// CaptchaVerifyRequest CAPTCHA 验证请求
type CaptchaVerifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

// CaptchaVerifyResponse CAPTCHA 验证响应
type CaptchaVerifyResponse struct {
	Success bool `json:"success"`
}

// CaptchaMiddleware 人机验证中间件
func CaptchaMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("captcha_token")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":401,"error":"缺少 captcha_token 参数"}`))
			return
		}

		secret := os.Getenv("CAP_SECRET")
		if secret == "" {
			log.Println("❌ 环境变量 CAP_SECRET 未设置")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"error":"服务器配置错误"}`))
			return
		}

		ok, err := verifyCaptcha(secret, token)
		if err != nil {
			log.Printf("❌ 人机验证请求失败: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"code":502,"error":"人机验证服务异常"}`))
			return
		}
		if !ok {
			log.Println("❌ 人机验证未通过")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"code":403,"error":"人机验证未通过，请重新验证"}`))
			return
		}

		next(w, r)
	}
}

// verifyCaptcha 调用 CAPTCHA 验证接口
func verifyCaptcha(secret, token string) (bool, error) {
	bodyData := CaptchaVerifyRequest{
		Secret:   secret,
		Response: token,
	}
	bodyBytes, err := json.Marshal(bodyData)
	if err != nil {
		return false, err
	}

	resp, err := http.Post("https://cap.fanverify.cn/siteverify",
		"application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result CaptchaVerifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, err
	}

	return result.Success, nil
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
