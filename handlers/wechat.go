package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

var db *sql.DB

func SetDB(database *sql.DB) {
	db = database
}

// WeChatAPIResponse 微信 API 响应结构
type WeChatAPIResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// User 用户结构
type User struct {
	UID     int    `json:"uid"`
	Level   int    `json:"level"`
	RegTime string `json:"reg_time"`
	Tag     string `json:"tag"`
	OpenID  string `json:"openid"`
}

// CodeRequest code 请求结构
type CodeRequest struct {
	Code string `json:"code"`
}

// CodeGenerateRequest 生成验证码请求
type CodeGenerateRequest struct {
	Code string `json:"code"`
}

// UpdateCodeLevelRequest 更新验证码等级请求
type UpdateCodeLevelRequest struct {
	Code         string `json:"code"`
	NewCodeLevel int    `json:"new_code_level"`
}

// OTPAPIRequest OTP API 请求
type OTPAPIRequest struct {
	Code string `json:"code"`
	OTP  string `json:"otp"`
}

const wxAPI = "https://api.weixin.qq.com/sns/jscode2session"

// HandleCodeOpenID 处理微信 code 换取 openid
func HandleCodeOpenID(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		var req CodeRequest
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

		if req.Code == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}

		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil || wxResp.ErrCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ 微信验证失败")
			return
		}

		user, err := findOrCreateUser(wxResp.OpenID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		response := map[string]interface{}{
			"code": 200,
			"data": []User{*user},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 用户登录成功: UID=%d, OpenID=%s", user.UID, user.OpenID)
	}
}

// HandleCodeGenerate 生成验证码
func HandleCodeGenerate(appID, secret, codeSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		var req CodeGenerateRequest
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

		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil || wxResp.ErrCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			return
		}

		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			return
		}

		verifyCode := generateVerifyCode(user.Level, codeSecret)
		lostTime := fmt.Sprintf("%d", time.Now().Add(5*time.Minute).UnixMilli())

		_, err = db.Exec("UPDATE userdata SET tmp_code = ?, code_level = ?, code_lost_time = ? WHERE UID = ?",
			verifyCode, user.Level, lostTime, user.UID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		response := map[string]interface{}{
			"code": 200,
			"data": map[string]string{
				"verify_code": verifyCode,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 生成验证码: UID=%d, Level=%d", user.UID, user.Level)
	}
}

// HandleUpdateCodeLevel 更新验证码等级
func HandleUpdateCodeLevel(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		var req UpdateCodeLevelRequest
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

		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil || wxResp.ErrCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			return
		}

		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			return
		}

		if req.NewCodeLevel < 0 || req.NewCodeLevel > 2 {
			req.NewCodeLevel = 0
		}

		_, err = db.Exec("UPDATE userdata SET code_level = ? WHERE UID = ?", req.NewCodeLevel, user.UID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		response := map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"new_code_level": req.NewCodeLevel,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 更新验证码等级: UID=%d, NewLevel=%d", user.UID, req.NewCodeLevel)
	}
}

// HandleOTPAPI 小程序 OTP 绑定
func HandleOTPAPI(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

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

		wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, req.Code)
		wxResp, err := callWeChatAPI(wxURL)
		if err != nil || wxResp.ErrCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ OTP API: 微信验证失败")
			return
		}

		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ OTP API: 用户不存在")
			return
		}

		var lostdate string
		var existingUID int
		err = db.QueryRow("SELECT lostdate, UID FROM otpdata WHERE otp = ?", req.OTP).Scan(&lostdate, &existingUID)
		if err == sql.ErrNoRows {
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

		lostTimestamp, _ := strconv.ParseInt(lostdate, 10, 64)
		if time.Now().UnixMilli() > lostTimestamp {
			db.Exec("DELETE FROM otpdata WHERE otp = ?", req.OTP)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("⏰ OTP API: OTP 已过期 %s", req.OTP)
			return
		}

		if existingUID != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ OTP API: OTP 已被绑定 %s", req.OTP)
			return
		}

		bandTime := fmt.Sprintf("%d", time.Now().UnixMilli())
		_, err = db.Exec("UPDATE otpdata SET UID = ?, band_time = ? WHERE otp = ?", user.UID, bandTime, req.OTP)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ OTP API: 绑定失败 %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200}`))
		log.Printf("✅ OTP API: 绑定成功, UID=%d, OTP=%s", user.UID, req.OTP)
	}
}

// 辅助函数
func callWeChatAPI(url string) (*WeChatAPIResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wxResp WeChatAPIResponse
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return nil, err
	}

	return &wxResp, nil
}

func findOrCreateUser(openID string) (*User, error) {
	user, err := findUserByOpenID(openID)
	if err == nil && user != nil {
		return user, nil
	}

	result, err := db.Exec("INSERT INTO userdata (openid, level, status) VALUES (?, 1, 'ok')", openID)
	if err != nil {
		return nil, err
	}

	uid, _ := result.LastInsertId()
	return findUserByUID(int(uid))
}

func findUserByOpenID(openID string) (*User, error) {
	var user User
	err := db.QueryRow("SELECT UID, level, reg_time, tag, openid FROM userdata WHERE openid = ?", openID).
		Scan(&user.UID, &user.Level, &user.RegTime, &user.Tag, &user.OpenID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func findUserByUID(uid int) (*User, error) {
	var user User
	err := db.QueryRow("SELECT UID, level, reg_time, tag, openid FROM userdata WHERE UID = ?", uid).
		Scan(&user.UID, &user.Level, &user.RegTime, &user.Tag, &user.OpenID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func generateVerifyCode(level int, secret string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d_%d", secret, level, timestamp)
}
