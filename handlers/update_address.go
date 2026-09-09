package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// emailSessionCookie 邮箱绑定会话 Cookie 名（非 HttpOnly，JS 可读，5 分钟有效）
const emailSessionCookie = "fanverify_email_token"

// UpdateAddressRequest 绑定/更换联系邮箱请求（POST）
type UpdateAddressRequest struct {
	Token string `json:"token"` // JWT；Web 端 HttpOnly Cookie 时可留空自动读取
	Email string `json:"email"` // 目标邮箱，不超过 30 位
}

// HandleUpdateAddress 绑定/更换联系邮箱接口（与手机号二级验证流程一致）
//
//	POST : {token, email} → 校验 + 邮箱占用检查 + 60s 重发限制 → 生成 session_token 写 tmp_biand(email)
//	      → 下发 JS 可读的会话 Cookie → 腾讯云 SES 发送验证码邮件 → 返回 {session_token}
//	GET  : ?session_token=xxx&pass_code=yyy → 校验验证码 → 绑定 email 到 userdata → 清除会话
func HandleUpdateAddress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleUpdateAddressPost(w, r)
		case http.MethodGet:
			handleUpdateAddressGet(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST / GET 请求"})
		}
	}
}

// authByToken 解析登录令牌（body.token 或 HttpOnly Cookie），返回 uid/openid
func authByToken(w http.ResponseWriter, r *http.Request, token string) (int, string, bool) {
	if token == "" {
		if cookie, err := r.Cookie("fanverify_token"); err == nil {
			token = cookie.Value
		}
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("❌ 环境变量 JWT_SECRET 未设置")
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return 0, "", false
	}
	claims, err := verifyJWT(jwtSecret, token)
	if err != nil || len(claims.UserData) == 0 {
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "您的访问令牌无效，登录可能已过期"})
		return 0, "", false
	}
	// 省份一致性校验
	if claims.Province != "" {
		currentProvince := queryProvince(clientIP(r))
		if currentProvince != "" && currentProvince != claims.Province {
			log.Printf("⚠️ 省份变化，拒绝绑定邮箱: 绑定省=%s 当前省=%s", claims.Province, currentProvince)
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "您的访问令牌无效，登录可能已过期"})
			return 0, "", false
		}
	}
	// 封禁检查：封禁用户不可操作（实时查库）
	var st string
	if err := db.QueryRow("SELECT status FROM userdata WHERE UID = ?", claims.UserData[0].UID).Scan(&st); err == nil && st == "bad" {
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "账号已被封禁"})
		return 0, "", false
	}
	return claims.UserData[0].UID, claims.UserData[0].OpenID, true
}

// POST：申请发送邮箱验证码
func handleUpdateAddressPost(w http.ResponseWriter, r *http.Request) {
	var req UpdateAddressRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
		return
	}

	// 1. 校验登录令牌
	uid, openid, ok := authByToken(w, r, req.Token)
	if !ok || openid == "" {
		return
	}

	// 2. 邮箱格式校验（≤30 位，必须准确填写，否则后续无法接收重要通知）
	req.Email = strings.TrimSpace(req.Email)
	if !isVaildEmail(req.Email) {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "邮箱格式不正确（长度不超过 30 位）"})
		return
	}

	// 3. 邮箱占用检查：同一邮箱不能被其他帐号绑定
	var emailCount int
	err = db.QueryRow("SELECT COUNT(*) FROM userdata WHERE email = ? AND openid <> ?", req.Email, openid).Scan(&emailCount)
	if err != nil {
		log.Printf("❌ 邮箱占用检查失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}
	if emailCount > 0 {
		writeJSON(w, http.StatusConflict, ApiResponse{Success: false, Error: "该邮箱已被其他帐号绑定"})
		return
	}

	// 4. 60s 内禁止重复发送
	var prevLostTime string
	err = db.QueryRow("SELECT lost_time FROM tmp_biand WHERE openid = ?", openid).Scan(&prevLostTime)
	if err == nil {
		if t, e := strconv.ParseInt(prevLostTime, 10, 64); e == nil && time.Now().UnixMilli() < t {
			remain := int((t - time.Now().UnixMilli()) / 1000)
			if remain < 1 {
				remain = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", remain))
			writeJSON(w, http.StatusTooManyRequests, ApiResponse{Success: false, Error: fmt.Sprintf("验证码发送过于频繁，请 %d 秒后重试", remain)})
			return
		}
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)
	} else if err != sql.ErrNoRows {
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 5. 生成会话与验证码
	sessionToken := generateOTP(32)
	passCode := genPassCode(6)
	lostTime := time.Now().Add(vlResendSec * time.Second).UnixMilli()
	passLostTime := time.Now().Add(vlCodeExpire).UnixMilli()

	// 6. 写入 tmp_biand（邮箱会话，phone 为 NULL）
	_, err = db.Exec(
		"INSERT INTO tmp_biand (openid, phone, email, session_token, lost_time, pass_code, pass_lost_time) VALUES (?, NULL, ?, ?, ?, ?, ?)",
		openid, req.Email, sessionToken, fmt.Sprintf("%d", lostTime), passCode, fmt.Sprintf("%d", passLostTime),
	)
	if err != nil {
		log.Printf("❌ 写入 tmp_biand(email) 失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 7. 下发 JS 可读会话 Cookie（状态存在浏览器，便于后续 GET 校验）
	http.SetCookie(w, &http.Cookie{
		Name:     emailSessionCookie,
		Value:    sessionToken,
		Path:     "/",
		Domain:   "fanverify.cn",
		HttpOnly: false, // JS 可读（按需求）
		Secure:   true,
		MaxAge:   300, // 5 分钟
		SameSite: http.SameSiteLaxMode,
	})

	// 8. 腾讯云 SES 发送验证码邮件
	if err := sendEmailCode(req.Email, passCode); err != nil {
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid) // 发送失败清理记录
		log.Printf("❌ 验证码邮件发送失败: %v", err)
		writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "邮件发送失败，请稍后重试或检查邮箱地址"})
		return
	}

	// 9. 返回 session_token（同时已通过 Cookie 下发）
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"session_token": sessionToken,
			"email":         req.Email,
			"lost_time":     fmt.Sprintf("%d", lostTime),
		},
	})
	log.Printf("✅ 邮箱验证码已发送: UID=%d email=%s", uid, maskEmail(req.Email))
}

// GET：校验验证码并绑定邮箱
func handleUpdateAddressGet(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.URL.Query().Get("session_token")
	passCode := r.URL.Query().Get("pass_code")
	// 会话 Cookie 兜底（JS 可读，可直接读 cookie）
	if sessionToken == "" {
		if cookie, err := r.Cookie(emailSessionCookie); err == nil {
			sessionToken = cookie.Value
		}
	}
	if sessionToken == "" || passCode == "" {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "参数缺失"})
		return
	}

	// 1. 校验登录令牌（绑定前必须为本人登录态）
	uid, openid, ok := authByToken(w, r, "")
	if !ok || openid == "" {
		return
	}

	// 2. 查会话记录
	var email, dbPassCode, passLostTime string
	err := db.QueryRow(
		"SELECT email, pass_code, pass_lost_time FROM tmp_biand WHERE openid = ? AND session_token = ?",
		openid, sessionToken,
	).Scan(&email, &dbPassCode, &passLostTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "验证会话不存在，请重新获取验证码"})
		return
	}
	if err != nil {
		log.Printf("❌ 查询 tmp_biand(email) 失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}
	if email == "" {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "该会话不是邮箱验证会话"})
		return
	}

	// 3. 验证码过期校验
	if t, e := strconv.ParseInt(passLostTime, 10, 64); e == nil && time.Now().UnixMilli() > t {
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)
		clearEmailSessionCookie(w)
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码无效，可能已过期"})
		return
	}

	// 4. 验证码匹配
	if dbPassCode != passCode {
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码不正确"})
		return
	}

	// 5. 二次占用检查（发送期间邮箱可能已被他人绑定）
	var emailCount int
	err = db.QueryRow("SELECT COUNT(*) FROM userdata WHERE email = ? AND openid <> ?", email, openid).Scan(&emailCount)
	if err != nil {
		log.Printf("❌ 邮箱二次占用检查失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}
	if emailCount > 0 {
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)
		clearEmailSessionCookie(w)
		writeJSON(w, http.StatusConflict, ApiResponse{Success: false, Error: "该邮箱已被其他帐号绑定"})
		return
	}

	// 6. 绑定邮箱到 userdata
	if _, err := db.Exec("UPDATE userdata SET email = ? WHERE openid = ?", email, openid); err != nil {
		log.Printf("❌ 绑定邮箱失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "绑定失败，请重试"})
		return
	}
	db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)
	clearEmailSessionCookie(w)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"email": email,
			"mask":  maskEmail(email),
		},
	})
	log.Printf("✅ 邮箱绑定成功: UID=%d email=%s", uid, maskEmail(email))
}

// clearEmailSessionCookie 清除邮箱会话 Cookie（JS 可读版本，同样由 Set-Cookie 清除）
func clearEmailSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     emailSessionCookie,
		Value:    "",
		Path:     "/",
		Domain:   "fanverify.cn",
		HttpOnly: false,
		Secure:   true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}
