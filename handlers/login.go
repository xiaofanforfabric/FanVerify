package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoginRequest 登录请求体
type LoginRequest struct {
	CapToken   string `json:"cap_token"`   // 人机验证 token
	UID        string `json:"uid"`         // 用户 UID
	VerifyCode string `json:"verify_code"` // 用户当前动态验证码
}

// JWTClaims JWT 载荷
// 结构：{ ip, province, iat, exp, userdata }
// province 以省为基准绑定：容忍同省换 IP/运营商；跨省视为 token 被挪用，拒绝
// ip 字段仅记录签发时的客户端 IP（供审计）
type JWTClaims struct {
	IP       string `json:"ip"`       // 签发时的客户端 IP（记录用）
	Province string `json:"province"` // 签发时 IP 所属省份（防重放校验基准）
	IAT      int64  `json:"iat"`      // 签发时间（Unix 秒）
	EXP      int64  `json:"exp"`      // 过期时间（Unix 秒）
	UserData []User `json:"userdata"` // 用户信息（自包含在 token 内）
}

// LoginResponse 登录成功响应
type LoginResponse struct {
	UserData []User `json:"userdata"`       // 用户信息
	IP       string `json:"ip"`             // 客户端 IP
	LostTime string `json:"lost_time"`      // JWT 过期时间（Unix 毫秒）
	Email    string `json:"email,omitempty"` // 已绑定的联系邮箱
	QQ       string `json:"qq,omitempty"`    // 已绑定的 QQ(预留)
}

// defaultJWTExpire JWT 默认有效期 7 天
const defaultJWTExpire = 7 * 24 * time.Hour

// HandleLogin 登录接口，支持两种方式：
//
//	POST  : body {cap_token, uid, verify_code} —— 人机验证 + 动态码核对 + 签发 JWT
//	GET   : ?cap_token=xxx           —— OTP 式登录，人机验证通过后下发 OTP
//	        ?cap_token=xxx&otp=yyy   —— 等效 seeotp 查询登录状态，已绑定则签发 JWT
func HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleLoginPOST(w, r)
		case http.MethodGet:
			handleLoginGET(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST / GET 请求"})
		}
	}
}

// handleLoginPOST 处理 POST 方式登录
func handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	// 1. 解析请求体
	var req LoginRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
		return
	}

	// 2. 字段校验
	if req.CapToken == "" || req.UID == "" || req.VerifyCode == "" {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "cap_token、uid、verify_code 均不能为空"})
		return
	}

	// 3. 人机验证（核对是不是机器人）
	secret := os.Getenv("CAP_SECRET")
	if secret == "" {
		log.Println("❌ 环境变量 CAP_SECRET 未设置")
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return
	}
	ok, err := verifyCaptcha(secret, req.CapToken)
	if err != nil {
		log.Printf("❌ 人机验证请求失败: %v", err)
		writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "人机验证服务异常"})
		return
	}
	if !ok {
		log.Printf("❌ 人机验证未通过 UID=%s", req.UID)
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "人机验证未通过，请重新验证"})
		return
	}

	// 4. 核对用户与动态验证码
	uid, err := strconv.Atoi(req.UID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "uid 必须是数字"})
		return
	}

	var user User
	var tmpCode, codeLostTime string
	err = db.QueryRow(
		"SELECT UID, level, code_level, reg_time, tag, status, openid, tmp_code, code_lost_time FROM userdata WHERE UID = ?",
		uid,
	).Scan(&user.UID, &user.Level, &user.CodeLevel, &user.RegTime, &user.Tag, &user.Status, &user.OpenID, &tmpCode, &codeLostTime)
	if err == sql.ErrNoRows {
		log.Printf("❌ 登录失败: 用户不存在 UID=%d", uid)
		writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "用户不存在"})
		return
	}
	if err != nil {
		log.Printf("❌ 查询用户失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 封禁检查：封禁用户不可登录
	if user.Status == "bad" {
		log.Printf("🚫 封禁用户尝试登录被拒: UID=%d", uid)
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "账号已被封禁，无法登录"})
		return
	}

	// 无论成功与否，先作废本次动态验证码：防止 150 秒窗口内暴力枚举（如 4 位码仅 1 万种）
	db.Exec("UPDATE userdata SET tmp_code = '', code_lost_time = '' WHERE UID = ?", uid)

	// 校验动态验证码
	if tmpCode != req.VerifyCode {
		log.Printf("❌ 登录失败: 验证码不匹配 UID=%d", uid)
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码不正确"})
		return
	}

	// 校验验证码是否过期
	if codeLostTime != "" {
		lostTimestamp, err := strconv.ParseInt(codeLostTime, 10, 64)
		if err == nil && time.Now().UnixMilli() > lostTimestamp {
			log.Printf("❌ 登录失败: 验证码已过期 UID=%d", uid)
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码已过期，请刷新后重试"})
			return
		}
	}

	// 5. 签发 JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("❌ 环境变量 JWT_SECRET 未设置")
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return
	}

	now := time.Now()
	expireAt := now.Add(defaultJWTExpire)
	ip := clientIP(r)
	claims := JWTClaims{
		IP:       ip,
		Province: queryProvince(ip),
		IAT:      now.Unix(),
		EXP:      expireAt.Unix(),
		UserData: []User{user},
	}
	token, err := signJWT(jwtSecret, claims)
	if err != nil {
		log.Printf("❌ JWT 签发失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "令牌签发失败"})
		return
	}

	// 将 JWT 写入 cookie（仅 fanverify.cn 主域名及子域名可访问；HttpOnly 防 XSS；SameSite=Lax 防 CSRF）
	http.SetCookie(w, &http.Cookie{
		Name:     "fanverify_token",
		Value:    token,
		Path:     "/",
		Domain:   "fanverify.cn",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   int(defaultJWTExpire.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})

	// 6. 组装响应
	resp := LoginResponse{
		UserData: []User{user},
		IP:       clientIP(r),
		LostTime: strconv.FormatInt(expireAt.UnixMilli(), 10),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-FanVerify-Token", token)
	json.NewEncoder(w).Encode(resp)
	log.Printf("✅ 登录成功并签发 JWT: UID=%d, IP=%s, 过期=%s", user.UID, resp.IP, resp.LostTime)
}

// handleLoginGET 处理 GET 方式（OTP 式）登录
func handleLoginGET(w http.ResponseWriter, r *http.Request) {
	capToken := r.URL.Query().Get("cap_token")
	otp := r.URL.Query().Get("otp")

	if capToken == "" {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "缺少 cap_token 参数"})
		return
	}

	// 1. 人机验证（先核对是不是机器人）
	secret := os.Getenv("CAP_SECRET")
	if secret == "" {
		log.Println("❌ 环境变量 CAP_SECRET 未设置")
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return
	}
	ok, err := verifyCaptcha(secret, capToken)
	if err != nil {
		log.Printf("❌ 人机验证请求失败: %v", err)
		writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "人机验证服务异常"})
		return
	}
	if !ok {
		log.Printf("❌ 人机验证未通过 (OTP 式登录)")
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "人机验证未通过，请重新验证"})
		return
	}

	// 2. 未带 otp：人机验证通过后下发 OTP
	if otp == "" {
		newOTP := generateOTP(16)
		lostdate := fmt.Sprintf("%d", time.Now().Add(150*time.Second).UnixMilli())
		_, err = db.Exec(
			"INSERT INTO otpdata (otp, lostdate, UID, band_time, devaccesstoken) VALUES (?, ?, 0, '', ?)",
			newOTP, lostdate, "login",
		)
		if err != nil {
			log.Printf("❌ 写入 OTP 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "OTP 生成失败"})
			return
		}
		log.Printf("🔑 OTP 式登录: 下发 OTP=%s", newOTP)
		writeJSON(w, http.StatusOK, map[string]string{"status": "issued", "otp": newOTP})
		return
	}

	// 3. 带 otp：等效 seeotp 查询登录状态
	var lostdate string
	var uid int
	err = db.QueryRow("SELECT lostdate, UID FROM otpdata WHERE otp = ?", otp).Scan(&lostdate, &uid)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]string{"status": "404"})
		return
	}
	if err != nil {
		log.Printf("❌ 查询 OTP 失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	lostTimestamp, _ := strconv.ParseInt(lostdate, 10, 64)
	if time.Now().UnixMilli() > lostTimestamp {
		db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)
		writeJSON(w, http.StatusOK, map[string]string{"status": "lost"})
		return
	}
	if uid == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "wait"})
		return
	}

	// 4. OTP 已绑定用户 → 登录成功 → 签发 JWT 并写入 cookie
	var user User
	err = db.QueryRow(
		"SELECT UID, level, code_level, reg_time, tag, status, openid FROM userdata WHERE UID = ?",
		uid,
	).Scan(&user.UID, &user.Level, &user.CodeLevel, &user.RegTime, &user.Tag, &user.Status, &user.OpenID)
	if err != nil {
		log.Printf("❌ 查询用户失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 封禁检查：封禁用户不可通过 OTP 式登录
	if user.Status == "bad" {
		log.Printf("🚫 封禁用户 OTP 登录被拒: UID=%d", user.UID)
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "账号已被封禁，无法登录"})
		return
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("❌ 环境变量 JWT_SECRET 未设置")
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return
	}

	now := time.Now()
	expireAt := now.Add(defaultJWTExpire)
	ip := clientIP(r)
	claims := JWTClaims{
		IP:       ip,
		Province: queryProvince(ip),
		IAT:      now.Unix(),
		EXP:      expireAt.Unix(),
		UserData: []User{user},
	}
	token, err := signJWT(jwtSecret, claims)
	if err != nil {
		log.Printf("❌ JWT 签发失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "令牌签发失败"})
		return
	}

	// 将 JWT 写入 cookie（仅 fanverify.cn 主域名及子域名可访问；HttpOnly 防 XSS；SameSite=Lax 防 CSRF）
	http.SetCookie(w, &http.Cookie{
		Name:     "fanverify_token",
		Value:    token,
		Path:     "/",
		Domain:   "fanverify.cn",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   int(defaultJWTExpire.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})

	// 删除已使用的 OTP
	db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)

	resp := LoginResponse{
		UserData: []User{user},
		IP:       clientIP(r),
		LostTime: strconv.FormatInt(expireAt.UnixMilli(), 10),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-FanVerify-Token", token)
	json.NewEncoder(w).Encode(resp)
	log.Printf("✅ OTP 式登录成功: UID=%d, IP=%s", user.UID, resp.IP)
}

// clientIP 获取客户端真实 IP（优先代理头，回退 RemoteAddr）
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// b64url base64url 编码（无 padding）
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// signJWT 签发 HS256 JWT
func signJWT(secret string, claims JWTClaims) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64url(headerJSON) + "." + b64url(payloadJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := b64url(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

// verifyJWT 验证 HS256 JWT 并解析载荷（供后续鉴权中间件使用）
func verifyJWT(secret, token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("令牌格式错误")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expected := b64url(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("签名校验失败")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims JWTClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, err
	}
	if claims.EXP != 0 && time.Now().Unix() > claims.EXP {
		return nil, errors.New("令牌已过期")
	}
	return &claims, nil
}

// HandleMe 当前登录用户信息接口：从 Cookie 校验 JWT 后返回用户信息
func HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			log.Println("❌ 环境变量 JWT_SECRET 未设置")
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
			return
		}

		cookie, err := r.Cookie("fanverify_token")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "未登录"})
			return
		}

		claims, err := verifyJWT(jwtSecret, cookie.Value)
		if err != nil {
			// 令牌过期/无效：主动清除浏览器里的失效 Cookie（前端 JS 无法删除 HttpOnly Cookie）
			clearAuthCookie(w)
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录已过期，请重新登录"})
			return
		}

		// 省份校验：以省为基准（容忍同省换 IP/运营商；跨省视为 token 被挪用）
		if claims.Province != "" {
			currentProvince := queryProvince(clientIP(r))
			if currentProvince != "" && currentProvince != claims.Province {
				log.Printf("⚠️ 省份变化，拒绝访问: 绑定省=%s 当前省=%s", claims.Province, currentProvince)
				clearAuthCookie(w)
				writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录环境变化，请重新登录"})
				return
			}
		}

		// 从 JWT 自包含的 userdata 读取用户信息（无需再次查库）
		if len(claims.UserData) == 0 {
			clearAuthCookie(w)
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录信息不完整，请重新登录"})
			return
		}

		uid := claims.UserData[0].UID
		resp := LoginResponse{
			UserData: claims.UserData,
			IP:       clientIP(r),
			LostTime: strconv.FormatInt(claims.EXP*1000, 10),
		}
		// 补充最新联系方式（JWT 自包含数据不含 email/qq，此处实时查库返回）
		var email, qq string
		if err := db.QueryRow("SELECT email, qq FROM userdata WHERE UID = ?", uid).Scan(&email, &qq); err == nil {
			resp.Email = email
			resp.QQ = qq
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
		log.Printf("✅ 查询当前用户: UID=%d (JWT 自包含)", uid)
	}
}

// clearAuthCookie 指示浏览器清除登录 Cookie
// （HttpOnly Cookie 前端 JS 无法删除，只能由后端通过 Set-Cookie 指令浏览器删除）
func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "fanverify_token",
		Value:    "",
		Path:     "/",
		Domain:   "fanverify.cn",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1, // 立即过期，浏览器删除
		SameSite: http.SameSiteLaxMode,
	})
}

// HandleLogout 退出登录接口
// 幂等：无论是否已登录都清除 Cookie 并返回成功（前端 JS 无法删除 HttpOnly Cookie，必须由后端清除）
func HandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}
		clearAuthCookie(w)
		log.Printf("🚪 用户退出登录，已清除 Cookie")
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Error: ""})
	}
}
