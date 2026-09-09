package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	UID       int    `json:"uid"`
	Level     int    `json:"level"`
	CodeLevel int    `json:"code_level"`
	RegTime   string `json:"reg_time"`
	Tag       string `json:"tag"`
	Status    string `json:"status"`
	OpenID    string `json:"openid"`
}

// CodeRequest code 请求结构
type CodeRequest struct {
	Code string `json:"code"`
	Reg  string `json:"reg"` // 协议同意标记："true" = 已同意用户协议/隐私政策并注册
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

		user, err := findUserByOpenID(wxResp.OpenID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		// 已注册用户 → 直接返回
		if user != nil {
			writeUserOK(w, user)
			return
		}

		// 未注册：仅查询注册状态（未带 reg=true）→ 404，等待用户同意协议后注册
		if req.Reg != "true" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"code":404,"error":"用户未注册"}`))
			log.Printf("ℹ️ 用户未注册，等待协议同意: OpenID=%s", wxResp.OpenID)
			return
		}

		// 已同意用户协议/隐私政策（reg=true）→ 注册
		user, err = createUser(wxResp.OpenID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}
		writeUserOK(w, user)
	}
}

// writeUserOK 输出登录成功（含用户信息）
func writeUserOK(w http.ResponseWriter, user *User) {
	response := map[string]interface{}{
		"code": 200,
		"data": []User{*user},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("✅ 用户登录成功: UID=%d", user.UID)
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

		now := time.Now()
		// 使用 TOTP 算法生成验证码
		verifyCode := generateCode(codeSecret, wxResp.OpenID, now, user.CodeLevel)
		lostTime := fmt.Sprintf("%d", getExpireTime(now))

		_, err = db.Exec("UPDATE userdata SET tmp_code = ?, code_level = ?, code_lost_time = ? WHERE UID = ?",
			verifyCode, user.CodeLevel, lostTime, user.UID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		response := map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"verify_code":    verifyCode,
				"code_lost_time": lostTime,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 生成验证码: UID=%d, Level=%d", user.UID, user.Level)
	}
}

// HandleUpdateCodeLevel 更新验证码等级
func HandleUpdateCodeLevel(appID, secret, codeSecret string) http.HandlerFunc {
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

		// 验证码等级: 1=6位数字, 2=4位数字, 3=8位混合
		if req.NewCodeLevel < 1 || req.NewCodeLevel > 3 {
			req.NewCodeLevel = 1 // 默认为 6 位数字
		}

		// 生成新的验证码
		now := time.Now()
		newVerifyCode := generateCode(codeSecret, wxResp.OpenID, now, req.NewCodeLevel)
		newLostTime := fmt.Sprintf("%d", getExpireTime(now))

		_, err = db.Exec("UPDATE userdata SET code_level = ?, tmp_code = ?, code_lost_time = ? WHERE UID = ?",
			req.NewCodeLevel, newVerifyCode, newLostTime, user.UID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":500}`))
			return
		}

		response := map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"new_code_level": req.NewCodeLevel,
				"new_tmp_code":   newVerifyCode,
				"code_lost_time": newLostTime,
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
		var devAccessToken string
		err = db.QueryRow("SELECT lostdate, UID, devaccesstoken FROM otpdata WHERE otp = ?", req.OTP).Scan(&lostdate, &existingUID, &devAccessToken)
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

		// 校验用户验证等级是否满足该业务密钥要求（level==0 视为 root，放行）
		if nel, ok := devNELByToken(devAccessToken); ok && !userLevelSatisfies(user.Level, nel) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"code":403,"error":"您的验证等级不足，无法完成该业务的验证，请在设置中升级后再试"}`))
			log.Printf("❌ OTP API: 验证等级不足 UID=%d level=%d need=%d", user.UID, user.Level, nel)
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

		// 更新开发者的 PUID 字段
		if devAccessToken != "" {
			var currentPUID sql.NullString
			err = db.QueryRow("SELECT PUID FROM devdata WHERE devaccesstoken = ?", devAccessToken).Scan(&currentPUID)
			if err == nil {
				puidValue := ""
				if currentPUID.Valid {
					puidValue = currentPUID.String
				}
				// 构造新的 PUID：检查是否已存在
				newUID := fmt.Sprintf("%d", user.UID)
				if !strings.Contains(puidValue, newUID) {
					if puidValue == "" {
						puidValue = newUID
					} else {
						puidValue = puidValue + ";" + newUID
					}
					db.Exec("UPDATE devdata SET PUID = ? WHERE devaccesstoken = ?", puidValue, devAccessToken)
					log.Printf("✅ 更新 PUID: %s, 添加 UID=%d", devAccessToken, user.UID)
				}
			}
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

// createUser 创建新用户（调用方需已确认用户同意用户协议/隐私政策）
func createUser(openID string) (*User, error) {
	result, err := db.Exec("INSERT INTO userdata (openid, level, status) VALUES (?, 1, 'ok')", openID)
	if err != nil {
		return nil, err
	}
	uid, _ := result.LastInsertId()
	return findUserByUID(int(uid))
}

func findUserByOpenID(openID string) (*User, error) {
	var user User
	err := db.QueryRow("SELECT UID, level, code_level, reg_time, tag, status, openid FROM userdata WHERE openid = ?", openID).
		Scan(&user.UID, &user.Level, &user.CodeLevel, &user.RegTime, &user.Tag, &user.Status, &user.OpenID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// TOTP 算法实现

// hmacHash 计算 HMAC-SHA1 原始值
func hmacHash(sharedSecret, msg string) []byte {
	mac := hmac.New(sha1.New, []byte(sharedSecret))
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

// generateCode 基于 TOTP 算法生成指定等级的验证码
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

// getExpireTime 返回当前时间窗口的过期时间戳（毫秒）
func getExpireTime(t time.Time) int64 {
	counter := t.Unix() / codeStep
	expireUnix := (counter + 1) * codeStep
	return expireUnix * 1000
}
func findUserByUID(uid int) (*User, error) {
	var user User
	err := db.QueryRow("SELECT UID, level, code_level, reg_time, tag, status, openid FROM userdata WHERE UID = ?", uid).
		Scan(&user.UID, &user.Level, &user.CodeLevel, &user.RegTime, &user.Tag, &user.Status, &user.OpenID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
