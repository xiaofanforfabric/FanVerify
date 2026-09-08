package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// DebugSendMailRequest 调试发信请求
type DebugSendMailRequest struct {
	Email string `json:"email"` // 收件邮箱
	Type  string `json:"type"`  // code / dev_ok / dev_reject / service
}

// HandleDebugSendEmail 调试接口：手动向指定邮箱发送邮件，验证 SES 模板与配置（仅 DEBUG=true 时注册）
// type:
//
//	code        邮箱验证码模板  (SES_TEMPLATE_ID)
//	dev_ok      开发者申请已通过 (SES_TPL_DEV_OK)
//	dev_reject  开发者申请未通过 (SES_TPL_DEV_REJECT)
//	service     服务状态变化通知 (SES_TPL_SERVICE)
func HandleDebugSendEmail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		var req DebugSendMailRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}

		req.Email = strings.TrimSpace(req.Email)
		typ := strings.ToLower(strings.TrimSpace(req.Type))
		if !isVaildEmail(req.Email) {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "邮箱格式不正确"})
			return
		}

		vars := map[string]string{"date": nowText()}
		subject, tid := "", ""
		switch typ {
		case "code":
			tid = os.Getenv("SES_TEMPLATE_ID")
			subject = "FanVerify 邮箱验证码"
			vars["code"] = "123456"
		case "dev_ok":
			tid = os.Getenv("SES_TPL_DEV_OK")
			subject = "FanVerify 开发者申请已通过"
			vars["name"] = "网站"
			vars["service"] = "https://demo.example.com"
			vars["key"] = "dev_DebugSendTest123456"
			vars["nel"] = "1"
		case "dev_reject":
			tid = os.Getenv("SES_TPL_DEV_REJECT")
			subject = "FanVerify 开发者申请未通过"
			vars["name"] = "网站"
			vars["service"] = "https://demo.example.com"
			vars["reason"] = "（调试发送示例：保活检测在线次数不足）"
		case "service":
			tid = os.Getenv("SES_TPL_SERVICE")
			subject = "FanVerify 服务状态变化通知"
			vars["content"] = "系统维护升级"
			vars["level"] = "受限模式"
			vars["data"] = "暂停动态验证码的生成与校验"
			vars["time"] = "约 2 小时"
		default:
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "type 需为 code / dev_ok / dev_reject / service"})
			return
		}
		if tid == "" {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "该模板尚未在 .env 中配置"})
			return
		}

		if err := sendSESMail(req.Email, subject, tid, vars); err != nil {
			log.Printf("❌ 调试发信失败 type=%s to=%s: %v", typ, req.Email, err)
			writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "邮件发送失败：" + err.Error()})
			return
		}
		log.Printf("✅ 调试发信成功 type=%s to=%s", typ, req.Email)
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"message": "测试邮件已发送",
			"type":    typ,
			"email":   req.Email,
		}})
	}
}
