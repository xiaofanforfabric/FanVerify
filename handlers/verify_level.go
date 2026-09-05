package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const (
	vlResendSec  = 60              // 验证码重发间隔（秒）
	vlCodeExpire = 5 * time.Minute // 验证码有效期
)

// VerifyLevelRequest 申请升级二级验证验证码（POST）
type VerifyLevelRequest struct {
	Code  string `json:"code"`  // wx.login 的 code
	Phone string `json:"phone"` // 11 位手机号
}

// HandleUpdateVerifyLevel 升级二级验证等级（手机号 + 短信验证码）
//
//	POST : 申请发送短信验证码 → 返回 session_token
//	GET  : 校验验证码 → 通过后 level 提升为 2 并写入手机号
func HandleUpdateVerifyLevel(appID, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleVerifyLevelPost(w, r, appID, secret)
		case http.MethodGet:
			handleVerifyLevelGet(w, r, appID, secret)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST / GET 请求"})
		}
	}
}

// verifyLevelAuth 微信 code 换 openid，并校验用户已注册（未注册返回 404）
func verifyLevelAuth(w http.ResponseWriter, r *http.Request, appID, secret, code string) (string, *User, bool) {
	wxURL := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", wxAPI, appID, secret, code)
	wxResp, err := callWeChatAPI(wxURL)
	if err != nil || wxResp.ErrCode != 0 {
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录验证失败"})
		return "", nil, false
	}
	user, err := findUserByOpenID(wxResp.OpenID)
	if err != nil {
		log.Printf("❌ 查询用户失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return "", nil, false
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "用户未注册"})
		return "", nil, false
	}
	return wxResp.OpenID, user, true
}

// POST：申请短信验证码
func handleVerifyLevelPost(w http.ResponseWriter, r *http.Request, appID, secret string) {
	var req VerifyLevelRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
		return
	}
	if req.Code == "" || !isCNPhone(req.Phone) {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "手机号格式不正确"})
		return
	}

	// 1. code → openid，校验用户已注册
	openid, user, ok := verifyLevelAuth(w, r, appID, secret, req.Code)
	if !ok {
		return
	}

	// 2. 已是二级 → 拒绝，避免无效刷码
	if user.Level >= 2 {
		writeJSON(w, http.StatusConflict, ApiResponse{Success: false, Error: "账号已是二级验证，无需升级"})
		return
	}

	// 3. 手机号占用检查：该手机号若已被其他用户绑定则不发验证码
	var phoneCount int
	err = db.QueryRow("SELECT COUNT(*) FROM userdata WHERE phone = ? AND openid <> ?", req.Phone, openid).Scan(&phoneCount)
	if err != nil {
		log.Printf("❌ 查询手机号占用失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}
	if phoneCount > 0 {
		writeJSON(w, http.StatusConflict, ApiResponse{Success: false, Error: "该手机号已被占用"})
		return
	}

	// 4. 查上次记录：60s 内禁止重发
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
		// 上次已过重发期 → 清除旧记录重新生成
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

	// 6. 写入 tmp_biand
	_, err = db.Exec("INSERT INTO tmp_biand (openid, phone, session_token, lost_time, pass_code, pass_lost_time) VALUES (?, ?, ?, ?, ?, ?)",
		openid, req.Phone, sessionToken, fmt.Sprintf("%d", lostTime), passCode, fmt.Sprintf("%d", passLostTime))
	if err != nil {
		log.Printf("❌ 写入 tmp_biand 失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 7. 腾讯云短信发送
	if err := sendSmsCode(req.Phone, passCode); err != nil {
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid) // 发送失败清理记录
		log.Printf("❌ 短信发送失败: %v", err)
		writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "短信发送失败，请稍后重试"})
		return
	}

	// 8. 返回 session_token
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"session_token": sessionToken,
			"lost_time":     fmt.Sprintf("%d", lostTime),
		},
	})
	log.Printf("✅ 升级验证码已发送: UID=%d, phone=%s", user.UID, maskPhone(req.Phone))
}

// GET：校验验证码并升级
func handleVerifyLevelGet(w http.ResponseWriter, r *http.Request, appID, secret string) {
	code := r.URL.Query().Get("code")
	sessionToken := r.URL.Query().Get("session_token")
	passCode := r.URL.Query().Get("pass_code")
	if code == "" || sessionToken == "" || passCode == "" {
		writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "参数缺失"})
		return
	}

	// 1. code → openid，校验用户已注册
	openid, _, ok := verifyLevelAuth(w, r, appID, secret, code)
	if !ok {
		return
	}

	// 2. 查会话记录
	var phone, dbPassCode, passLostTime string
	err := db.QueryRow(
		"SELECT phone, pass_code, pass_lost_time FROM tmp_biand WHERE openid = ? AND session_token = ?",
		openid, sessionToken,
	).Scan(&phone, &dbPassCode, &passLostTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "验证会话不存在，请重新获取验证码"})
		return
	}
	if err != nil {
		log.Printf("❌ 查询 tmp_biand 失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
		return
	}

	// 3. 验证码过期校验
	if t, e := strconv.ParseInt(passLostTime, 10, 64); e == nil && time.Now().UnixMilli() > t {
		db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码无效，可能已过期"})
		return
	}

	// 4. 验证码匹配
	if dbPassCode != passCode {
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "验证码不正确"})
		return
	}

	// 5. 全部通过：level=2 + 写入手机号
	if _, err := db.Exec("UPDATE userdata SET level = 2, phone = ? WHERE openid = ?", phone, openid); err != nil {
		log.Printf("❌ 升级等级失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "升级失败，请重试"})
		return
	}
	db.Exec("DELETE FROM tmp_biand WHERE openid = ?", openid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"level": 2,
			"phone": maskPhone(phone),
		},
	})
	log.Printf("✅ 升级二级验证成功: openid=%s, phone=%s", openid, maskPhone(phone))
}

// isCNPhone 校验中国大陆 11 位手机号
func isCNPhone(phone string) bool {
	if len(phone) != 11 || phone[0] != '1' {
		return false
	}
	for i := 0; i < len(phone); i++ {
		if phone[i] < '0' || phone[i] > '9' {
			return false
		}
	}
	return true
}

// genPassCode 生成 n 位数字验证码
func genPassCode(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	for i := range b {
		b[i] = byte('0' + b[i]%10)
	}
	return string(b)
}

// maskPhone 手机号脱敏（138****1234）
func maskPhone(phone string) string {
	if len(phone) == 11 {
		return phone[:3] + "****" + phone[7:]
	}
	return phone
}

// sendSmsCode 通过腾讯云短信发送验证码
func sendSmsCode(phone, code string) error {
	secretID := os.Getenv("TC_SECRET_ID")
	secretKey := os.Getenv("TC_SECRET_KEY")
	sdkAppID := os.Getenv("TC_SMS_APP_ID")
	sign := os.Getenv("TC_SMS_SIGN")
	templateID := os.Getenv("TC_SMS_TEMPLATE")
	if secretID == "" || secretKey == "" || sdkAppID == "" || sign == "" || templateID == "" {
		return fmt.Errorf("腾讯云短信配置不完整")
	}

	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	client, err := sms.NewClient(credential, "ap-guangzhou", cpf)
	if err != nil {
		return err
	}

	req := sms.NewSendSmsRequest()
	req.PhoneNumberSet = common.StringPtrs([]string{"+86" + phone})
	req.SmsSdkAppId = common.StringPtr(sdkAppID)
	req.SignName = common.StringPtr(sign)
	req.TemplateId = common.StringPtr(templateID)
	req.TemplateParamSet = common.StringPtrs([]string{code})

	resp, err := client.SendSms(req)
	if err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.SendStatusSet != nil && len(resp.Response.SendStatusSet) > 0 {
		status := resp.Response.SendStatusSet[0]
		if status.Code != nil && *status.Code == "Ok" {
			return nil
		}
		return fmt.Errorf("短信发送失败 code=%s msg=%s", strPtr(status.Code), strPtr(status.Message))
	}
	return fmt.Errorf("短信发送失败：无状态返回")
}

// strPtr nil 安全取字符串
func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
