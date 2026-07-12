package main

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/skip2/go-qrcode"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort  = "8888"
	defaultAppID = "wxea50dc0c6b97b784"
	wxAPI        = "https://api.weixin.qq.com/sns/jscode2session"
)

var otpRateLimiter = make(map[string]int64) // OTP 限流器：otp -> 上次请求时间戳

type CodeRequest struct {
	Code string `json:"code"`
}

type WxResponse struct {
	OpenID     string `json:"openid,omitempty"`
	SessionKey string `json:"session_key,omitempty"`
	UnionID    string `json:"unionid,omitempty"`
	ErrCode    int    `json:"errcode,omitempty"`
	ErrMsg     string `json:"errmsg,omitempty"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type UserData struct {
	UID          int    `json:"uid"`
	RegTime      string `json:"reg_time"`
	Level        int    `json:"level"`
	Phone        string `json:"phone"`
	TmpCode      string `json:"tmp_code"`
	CodeLevel    int    `json:"code_level"`
	CodeLostTime string `json:"code_lost_time"`
	Status       string `json:"status"`
	Tag          string `json:"tag"`
	OpenID       string `json:"openid"`
}

func main() {
	loadEnv()

	appID := getEnv("WX_APPID", defaultAppID)
	secret := getEnv("WX_SECRET", "")
	if secret == "" {
		log.Fatal(".env 文件中未设置 WX_SECRET（小程序密钥）")
	}
	port := getEnv("PORT", defaultPort)

	// 初始化数据库
	initDB()

	// 读取 debug 模式
	debugMode := getEnv("DEBUG", "false") == "true"

	mux := http.NewServeMux()
	codeSecret := getEnv("CODE_SECRET", "")
	if codeSecret == "" {
		log.Fatal(".env 文件中未设置 CODE_SECRET（验证码共享密钥）")
	}

	mux.HandleFunc("/api/wx/code/openid", corsMiddleware(handleCodeOpenID(appID, secret)))
	mux.HandleFunc("/api/wx/code/generate", corsMiddleware(handleCodeGenerate(appID, secret, codeSecret)))
	mux.HandleFunc("/api/wx/update/code_level", corsMiddleware(handleUpdateCodeLevel(appID, secret, codeSecret)))
	mux.HandleFunc("/api/wx/otpAPI", corsMiddleware(handleOTPAPI(appID, secret)))
	mux.HandleFunc("/openapi/otp", corsMiddleware(accessTokenMiddleware(handleGenerateOTP())))
	mux.HandleFunc("/openapi/genqrcode", corsMiddleware(accessTokenMiddleware(handleGenerateQRCode())))
	mux.HandleFunc("/openapi/seeotp", corsMiddleware(accessTokenMiddleware(handleSeeOTP())))

	// Debug 路由
	if debugMode {
		log.Println("⚠️ DEBUG 模式已启用")
		mux.HandleFunc("/api/debug/adddev", corsMiddleware(handleDebugAddDev()))
		mux.HandleFunc("/api/debug/adddev/web/", handleDebugWebUI())
	} else {
		// 非 DEBUG 模式，拦截所有 debug 路由
		mux.HandleFunc("/api/debug/adddev", handle404Debug())
		mux.HandleFunc("/api/debug/adddev/web/", handle404Debug())
		mux.HandleFunc("/api/debug/", handle404Debug())
	}

	addr := ":" + port
	log.Printf("FanVerify 后端服务启动，监听端口 %s", port)
	log.Printf("API 地址: http://localhost%s/api/wx/code/openid", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func handleCodeOpenID(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		// 解析请求体
		var req CodeRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "读取请求体失败: " + err.Error()})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "解析 JSON 失败: " + err.Error()})
			return
		}
		if req.Code == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "code 不能为空"})
			return
		}

		// 调用微信服务器 jscode2session
		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
			wxAPI, appID, secret, req.Code)

		wxResp, err := callWeChatAPI(wxURL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: err.Error()})
			return
		}

		// 微信返回 errcode 表示失败
		if wxResp.ErrCode != 0 {
			writeJSON(w, http.StatusOK, ApiResponse{
				Success: false,
				Error:   fmt.Sprintf("微信返回错误 (errcode=%d): %s", wxResp.ErrCode, wxResp.ErrMsg),
			})
			return
		}

		// 查库：根据 openid 查找用户
		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "数据库查询失败: " + err.Error()})
			return
		}

		if user == nil {
			// 未找到 → 新用户，自动注册
			newUID, err := insertUser(wxResp.OpenID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "注册用户失败: " + err.Error()})
				return
			}
			user = &UserData{
				UID:    newUID,
				OpenID: wxResp.OpenID,
				Level:  1,
				Status: "ok",
			}
			log.Printf("🆕 新用户注册 UID=%d, openid=%s", newUID, wxResp.OpenID)
		} else {
			log.Printf("🔑 用户登录 UID=%d, openid=%s", user.UID, user.OpenID)
		}

		// 成功返回用户数据
		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data:    user,
		})
	}
}

func callWeChatAPI(wxURL string) (*WxResponse, error) {
	resp, err := http.Get(wxURL)
	if err != nil {
		return nil, fmt.Errorf("请求微信服务器失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取微信响应失败: %w", err)
	}

	var wxResp WxResponse
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	return &wxResp, nil
}

func findUserByOpenID(openid string) (*UserData, error) {
	row := db.QueryRow(`SELECT UID, reg_time, level, phone, tmp_code, code_level, code_lost_time, status, tag, openid
		FROM userdata WHERE openid = ?`, openid)

	var u UserData
	err := row.Scan(&u.UID, &u.RegTime, &u.Level, &u.Phone, &u.TmpCode, &u.CodeLevel, &u.CodeLostTime, &u.Status, &u.Tag, &u.OpenID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func insertUser(openid string) (int, error) {
	result, err := db.Exec(`INSERT INTO userdata (openid) VALUES (?)`, openid)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// ===== 验证码生成接口 =====

type CodeGenerateRequest struct {
	Code string `json:"code"`
}

func handleCodeGenerate(appID, secret, sharedSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		var req CodeGenerateRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "读取请求体失败: " + err.Error()})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "解析 JSON 失败: " + err.Error()})
			return
		}
		if req.Code == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "code 不能为空"})
			return
		}

		// 调用微信 API 获取 openid
		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "调用微信 API 失败: " + err.Error()})
			return
		}
		if wxResp.ErrCode != 0 {
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "微信验证失败: " + wxResp.ErrMsg})
			return
		}

		// 根据 openid 查用户
		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "查询用户失败: " + err.Error()})
			return
		}
		if user == nil {
			writeJSON(w, http.StatusOK, ApiResponse{Success: false, Error: "用户不存在"})
			return
		}

		// 后端限流：每秒每 uid 最多 5 次
		if !codeGenLimiter.allow(user.UID) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, ApiResponse{
				Success: false,
				Error:   "请求过于频繁，请 1 秒后再试",
			})
			return
		}

		now := time.Now()
		expireMs := getExpireTime(now)

		// 检查现有验证码是否仍有效
		if user.TmpCode != "" && user.CodeLostTime != "" {
			var lostTime int64
			fmt.Sscanf(user.CodeLostTime, "%d", &lostTime)
			if lostTime > now.UnixMilli() {
				// 未过期，直接返回现有 code
				log.Printf("♻️ UID=%d 复用验证码 %s，剩余 %ds", user.UID, user.TmpCode, (lostTime-now.UnixMilli())/1000)
				writeJSON(w, http.StatusOK, ApiResponse{
					Success: true,
					Data: map[string]interface{}{
						"code":           user.TmpCode,
						"code_lost_time": user.CodeLostTime,
					},
				})
				return
			}
		}

		// 生成新验证码（按用户当前 code_level）
		codeLevel := user.CodeLevel
		if !isValidCodeLevel(codeLevel) {
			codeLevel = codeLevel1
		}
		newCode := generateCode(sharedSecret, user.OpenID, now, codeLevel)
		expireStr := fmt.Sprintf("%d", expireMs)

		// 写入数据库
		_, err = db.Exec(`UPDATE userdata SET tmp_code = ?, code_lost_time = ? WHERE UID = ?`,
			newCode, expireStr, user.UID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "保存验证码失败: " + err.Error()})
			return
		}

		log.Printf("🆕 UID=%d 生成验证码 %s，过期 %d", user.UID, newCode, expireMs)

		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"code":           newCode,
				"code_lost_time": expireStr,
			},
		})
	}
}

// ===== 更新验证码等级接口 =====

type UpdateCodeLevelRequest struct {
	Code         string `json:"code"`
	NewCodeLevel int    `json:"new_code_level"`
}

type OTPAPIRequest struct {
	Code string `json:"code"`
	OTP  string `json:"otp"`
}

func handleUpdateCodeLevel(appID, secret, codeSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		var req UpdateCodeLevelRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "读取请求体失败: " + err.Error()})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "解析 JSON 失败: " + err.Error()})
			return
		}
		if req.Code == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "code 不能为空"})
			return
		}
		if !isValidCodeLevel(req.NewCodeLevel) {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "new_code_level 必须为 1、2 或 3"})
			return
		}

		// 用 code 换 openid
		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
			wxAPI, appID, secret, req.Code)

		wxResp, err := callWeChatAPI(wxURL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: err.Error()})
			return
		}
		if wxResp.ErrCode != 0 {
			writeJSON(w, http.StatusForbidden, ApiResponse{
				Success: false,
				Error:   fmt.Sprintf("微信验证失败 (errcode=%d): %s", wxResp.ErrCode, wxResp.ErrMsg),
			})
			return
		}

		// 查库
		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "数据库查询失败: " + err.Error()})
			return
		}
		if user == nil {
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "用户不存在，拒绝访问"})
			return
		}

		// 更新 code_level
		_, err = db.Exec(`UPDATE userdata SET code_level = ? WHERE UID = ?`, req.NewCodeLevel, user.UID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "更新失败: " + err.Error()})
			return
		}

		// 立刻重新生成验证码
		now := time.Now()
		newCode := generateCode(codeSecret, user.OpenID, now, req.NewCodeLevel)
		expireMs := getExpireTime(now)
		expireStr := fmt.Sprintf("%d", expireMs)

		_, err = db.Exec(`UPDATE userdata SET tmp_code = ?, code_lost_time = ? WHERE UID = ?`,
			newCode, expireStr, user.UID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成验证码失败: " + err.Error()})
			return
		}

		log.Printf("🔁 UID=%d code_level 更新为 %d，新验证码 %s", user.UID, req.NewCodeLevel, newCode)

		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"code":           200,
				"new_code_level": req.NewCodeLevel,
				"new_tmp_code":   newCode,
				"code_lost_time":  expireStr,
			},
		})
	}
}

func findUserByUID(uid int) (*UserData, error) {
	row := db.QueryRow(`SELECT UID, reg_time, level, phone, tmp_code, code_level, code_lost_time, status, tag, openid
		FROM userdata WHERE UID = ?`, uid)

	var u UserData
	err := row.Scan(&u.UID, &u.RegTime, &u.Level, &u.Phone, &u.TmpCode, &u.CodeLevel, &u.CodeLostTime, &u.Status, &u.Tag, &u.OpenID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

// accessTokenMiddleware OpenAPI 访问令牌鉴权中间件
func accessTokenMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// loadEnv 读取 .env 文件并设置到环境变量
func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Println("未找到 .env 文件，仅使用系统环境变量")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// ===== OTP 生成接口 =====

func handleGenerateOTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		// 生成 16 位 OTP（大小写字母 + 数字）
		otp := generateOTP(16)

		// 计算过期时间（150 秒后）
		lostdate := fmt.Sprintf("%d", time.Now().Add(150*time.Second).UnixMilli())

		// 写入数据库
		_, err := db.Exec("INSERT INTO otpdata (otp, lostdate, UID, band_time) VALUES (?, ?, 0, '')", otp, lostdate)
		if err != nil {
			log.Printf("❌ 写入 OTP 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成失败"})
			return
		}

		log.Printf("🆔 生成 OTP: %s, 过期时间: %s", otp, lostdate)

		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"otp": otp,
			},
		})
	}
}

// generateOTP 生成指定长度的随机 OTP（大小写字母 + 数字）
func generateOTP(length int) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Read(make([]byte, 1))
		if n > 0 {
			var b [1]byte
			rand.Read(b[:])
			result[i] = charset[int(b[0])%len(charset)]
		}
	}
	return string(result)
}

// ===== 小程序码生成接口 =====

// ===== 二维码生成接口 =====

func handleGenerateQRCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		// 获取参数
		otp := r.URL.Query().Get("otp")
		if otp == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "otp 参数不能为空"})
			return
		}

		// 获取尺寸参数
		pxStr := r.URL.Query().Get("px")
		px := 200 // 默认 200
		if pxStr != "" {
			if val, err := strconv.Atoi(pxStr); err == nil {
				if val >= 120 && val <= 350 {
					px = val
				}
			}
		}

		// 生成二维码内容 URL
		qrContent := fmt.Sprintf("https://api.fanverify.cn/openapi/otp?otp=%s", otp)

		// 生成二维码
		qrCode, err := qrcode.Encode(qrContent, qrcode.Medium, px)
		if err != nil {
			log.Printf("❌ 生成二维码失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成二维码失败: " + err.Error()})
			return
		}

		log.Printf("📷 生成二维码: otp=%s, size=%dx%d, data=%d bytes", otp, px, px, len(qrCode))

		// 返回 PNG 图片
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(qrCode)))
		w.WriteHeader(http.StatusOK)
		w.Write(qrCode)
	}
}

// ===== OTP 轮询接口 =====

func handleSeeOTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		// 获取参数
		otp := r.URL.Query().Get("otp")
		if otp == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "otp 参数不能为空"})
			return
		}

		// 限流检查：5 秒间隔
		now := time.Now().Unix()
		if lastTime, exists := otpRateLimiter[otp]; exists {
			if now-lastTime < 5 {
				w.WriteHeader(249)
				w.Write([]byte(`{"status":"rate_limit"}`))
				return
			}
		}
		otpRateLimiter[otp] = now

		// 查询 OTP
		var lostdate, bandTime string
		var uid int
		err := db.QueryRow("SELECT lostdate, UID, band_time FROM otpdata WHERE otp = ?", otp).Scan(&lostdate, &uid, &bandTime)

		// OTP 不存在
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"404"}`))
			return
		}
		if err != nil {
			log.Printf("❌ 查询 OTP 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "查询失败"})
			return
		}

		// 检查是否过期
		lostTimestamp, _ := strconv.ParseInt(lostdate, 10, 64)
		if time.Now().UnixMilli() > lostTimestamp {
			// 删除过期 OTP
			db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"lost"}`))
			log.Printf("⏰ OTP 已过期并删除: %s", otp)
			return
		}

		// 检查是否已扫描
		if uid == 0 || bandTime == "" {
			// 未扫描
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"wait"}`))
			return
		}

		// 已扫描，查询用户信息
		var regTime, tag string
		var level int
		err = db.QueryRow("SELECT reg_time, level, tag FROM userdata WHERE UID = ?", uid).Scan(&regTime, &level, &tag)
		if err != nil {
			log.Printf("❌ 查询用户信息失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "查询失败"})
			return
		}

		// 删除 OTP 记录
		db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)

		// 返回用户信息
		response := map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{{
				"uid":      uid,
				"level":    strconv.Itoa(level),
				"reg_time": regTime,
				"tag":      tag,
			}},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ OTP 验证成功并删除: %s, UID: %d", otp, uid)
	}
}

// ===== 小程序 OTP 绑定接口 =====

func handleOTPAPI(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		// 解析请求
		var req OTPAPIRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}

		if req.Code == "" || req.OTP == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}

		// 验证微信 code
		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil || wxResp.ErrCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ OTP API: 微信验证失败")
			return
		}

		// 查询用户
		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ OTP API: 用户不存在")
			return
		}

		// 查询 OTP 是否存在且未过期
		var lostdate string
		var existingUID int
		err = db.QueryRow("SELECT lostdate, UID FROM otpdata WHERE otp = ?", req.OTP).Scan(&lostdate, &existingUID)
		if err == sql.ErrNoRows {
			// OTP 不存在
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ OTP API: OTP 不存在 %s", req.OTP)
			return
		}
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			return
		}

		// 检查是否过期
		lostTimestamp, _ := strconv.ParseInt(lostdate, 10, 64)
		if time.Now().UnixMilli() > lostTimestamp {
			db.Exec("DELETE FROM otpdata WHERE otp = ?", req.OTP)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("⏰ OTP API: OTP 已过期 %s", req.OTP)
			return
		}

		// 检查是否已被绑定
		if existingUID != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ OTP API: OTP 已被绑定 %s", req.OTP)
			return
		}

		// 绑定 OTP 到用户
		bandTime := fmt.Sprintf("%d", time.Now().UnixMilli())
		_, err = db.Exec("UPDATE otpdata SET UID = ?, band_time = ? WHERE otp = ?", user.UID, bandTime, req.OTP)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ OTP API: 绑定失败 %v", err)
			return
		}

		// 返回成功
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200}`))
		log.Printf("✅ OTP API: 绑定成功, UID=%d, OTP=%s", user.UID, req.OTP)
	}
}

// ===== Debug 接口 =====

func handle404Debug() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not Found"}`))
	}
}

type AddDevRequest struct {
	DevAccessToken string `json:"devaccesstoken"`
	NEL            int    `json:"nel"`
	BUID           int    `json:"buid"`
	BIP            string `json:"bip"`
	Mode           string `json:"mode"`
	SME            string `json:"sme"`
}

func handleDebugAddDev() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"success":false,"error":"仅支持 POST 请求"}`))
			return
		}

		// 解析请求
		var req AddDevRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"请求解析失败"}`))
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"JSON 格式错误"}`))
			return
		}

		// 校验必填字段
		if req.DevAccessToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"开发者令牌不能为空"}`))
			return
		}
		if req.NEL < 1 {
			req.NEL = 1
		}
		if req.Mode == "" {
			req.Mode = "HTTP"
		}

		// 插入数据库
		_, err = db.Exec(
			"INSERT INTO devdata (devaccesstoken, NEL, BUID, BIP, mode, SME, status) VALUES (?, ?, ?, ?, ?, ?, 'wait')",
			req.DevAccessToken, req.NEL, req.BUID, req.BIP, req.Mode, req.SME,
		)
		if err != nil {
			log.Printf("❌ 添加开发者失败: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"添加失败，令牌可能已存在"}`))
			return
		}

		log.Printf("✅ 添加开发者: %s, BUID=%d, Mode=%s", req.DevAccessToken, req.BUID, req.Mode)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"开发者添加成功"}`))
	}
}

func handleDebugWebUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FanVerify - 添加开发者</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px; }
        .container { background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); max-width: 500px; width: 100%; padding: 40px; }
        h1 { color: #333; font-size: 28px; margin-bottom: 10px; text-align: center; }
        .subtitle { color: #666; font-size: 14px; text-align: center; margin-bottom: 30px; }
        .form-group { margin-bottom: 20px; }
        label { display: block; color: #555; font-size: 14px; font-weight: 600; margin-bottom: 8px; }
        input, select, textarea { width: 100%; padding: 12px 16px; border: 2px solid #e0e0e0; border-radius: 8px; font-size: 14px; transition: all 0.3s; }
        input:focus, select:focus, textarea:focus { outline: none; border-color: #667eea; box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1); }
        textarea { resize: vertical; min-height: 80px; font-family: inherit; }
        .btn { width: 100%; padding: 14px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 0.2s, box-shadow 0.2s; }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 25px rgba(102, 126, 234, 0.4); }
        .btn:active { transform: translateY(0); }
        .btn:disabled { opacity: 0.6; cursor: not-allowed; transform: none; }
        .generate-token { display: inline-block; color: #667eea; font-size: 13px; margin-top: 5px; cursor: pointer; text-decoration: none; }
        .generate-token:hover { text-decoration: underline; }
        .message { padding: 12px 16px; border-radius: 8px; margin-top: 20px; font-size: 14px; display: none; }
        .message.success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .message.error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .row { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔑 添加开发者</h1>
        <p class="subtitle">FanVerify Debug Panel</p>
        
        <form id="devForm">
            <div class="form-group">
                <label for="token">开发者令牌 *</label>
                <input type="text" id="token" name="devaccesstoken" required placeholder="dev_abc123xyz456">
                <a class="generate-token" onclick="generateToken()">随机生成</a>
            </div>
            
            <div class="row">
                <div class="form-group">
                    <label for="nel">要求最低等级 *</label>
                    <select id="nel" name="nel" required>
                        <option value="1">等级 1</option>
                        <option value="2">等级 2</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label for="buid">绑定 UID</label>
                    <input type="number" id="buid" name="buid" value="0" min="0" placeholder="0">
                </div>
            </div>
            
            <div class="row">
                <div class="form-group">
                    <label for="bip">服务器 IP *</label>
                    <select id="bip" name="bip" required>
                        <option value="ALL">允许所有 IP</option>
                        <option value="NULL">不限制</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label for="mode">模式 *</label>
                    <select id="mode" name="mode" required>
                        <option value="HTTP">HTTP</option>
                        <option value="FWS">正向 WebSocket</option>
                        <option value="RWS">反向 WebSocket</option>
                    </select>
                </div>
            </div>
            
            <div class="form-group">
                <label for="sme">业务描述</label>
                <textarea id="sme" name="sme" placeholder="请描述该令牌的使用场景..."></textarea>
            </div>
            
            <button type="submit" class="btn" id="submitBtn">✅ 确认添加</button>
        </form>
        
        <div id="message" class="message"></div>
    </div>
    
    <script>
        function generateToken() {
            const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
            let token = 'dev_';
            for (let i = 0; i < 32; i++) {
                token += chars.charAt(Math.floor(Math.random() * chars.length));
            }
            document.getElementById('token').value = token;
        }
        
        document.getElementById('devForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const submitBtn = document.getElementById('submitBtn');
            const message = document.getElementById('message');
            
            submitBtn.disabled = true;
            submitBtn.textContent = '⭳ 添加中...';
            message.style.display = 'none';
            
            const formData = {
                devaccesstoken: document.getElementById('token').value,
                nel: parseInt(document.getElementById('nel').value),
                buid: parseInt(document.getElementById('buid').value),
                bip: document.getElementById('bip').value,
                mode: document.getElementById('mode').value,
                sme: document.getElementById('sme').value
            };
            
            try {
                const response = await fetch('/api/debug/adddev', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(formData)
                });
                
                const data = await response.json();
                
                if (data.success) {
                    message.className = 'message success';
                    message.textContent = '✅ ' + data.message;
                    message.style.display = 'block';
                    document.getElementById('devForm').reset();
                } else {
                    message.className = 'message error';
                    message.textContent = '❌ ' + data.error;
                    message.style.display = 'block';
                }
            } catch (error) {
                message.className = 'message error';
                message.textContent = '❌ 网络错误: ' + error.message;
                message.style.display = 'block';
            } finally {
                submitBtn.disabled = false;
                submitBtn.textContent = '✅ 确认添加';
            }
        });
    </script>
</body>
</html>`
		w.Write([]byte(html))
	}
}
