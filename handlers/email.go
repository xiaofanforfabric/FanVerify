package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ses "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ses/v20201002"
)

// sendEmailCode 发送邮箱验证码（模板 SES_TEMPLATE_ID，需含 {{code}} 变量）
func sendEmailCode(toEmail, code string) error {
	return sendSESMail(toEmail, "FanVerify 邮箱验证码", os.Getenv("SES_TEMPLATE_ID"), map[string]string{"code": code})
}

// sendSESMail 通用发信：通过腾讯云邮件推送(SES)发送一封触发类邮件（走模板）
// 依赖 env：TC_SECRET_ID/TC_SECRET_KEY（与短信共用）、SES_REGION、SES_FROM
func sendSESMail(toEmail, subject, templateID string, vars map[string]string) error {
	secretID := os.Getenv("TC_SECRET_ID")
	secretKey := os.Getenv("TC_SECRET_KEY")
	region := os.Getenv("SES_REGION")
	from := os.Getenv("SES_FROM")
	if secretID == "" || secretKey == "" || region == "" || from == "" {
		return fmt.Errorf("腾讯云邮件(SES)配置不完整")
	}
	tid, err := strconv.ParseUint(templateID, 10, 64)
	if err != nil {
		return fmt.Errorf("模板ID非法: %s (%v)", templateID, err)
	}

	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	client, err := ses.NewClient(credential, region, cpf)
	if err != nil {
		return err
	}

	data, _ := json.Marshal(vars)
	trigger := uint64(1) // 触发类：即时发送

	req := ses.NewSendEmailRequest()
	req.FromEmailAddress = common.StringPtr(from)
	req.Subject = common.StringPtr(subject)
	req.Destination = common.StringPtrs([]string{toEmail})
	req.Template = &ses.Template{
		TemplateID:   &tid,
		TemplateData: common.StringPtr(string(data)),
	}
	req.TriggerType = &trigger

	resp, err := client.SendEmail(req)
	if err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.MessageId != nil && *resp.Response.MessageId != "" {
		return nil
	}
	return fmt.Errorf("邮件发送失败：无 MessageId")
}

// nowText 当前时间字符串
func nowText() string {
	return time.Now().Format("2006-01-02 15:04")
}

// RegisterEmailBroadcast 注册“申请结算→邮件通知”广播订阅者（main 启动时调用一次）
// 申请结算后会自动尝试向开发者绑定的邮箱发送通过/未通过通知；未绑定有效邮箱时记录日志并跳过。
func RegisterEmailBroadcast() {
	RegisterPlanVisaNotifier(sendDevSettleEmail)
}

// sendDevSettleEmail 开发者申请结算（保活检测满 12 次）自动邮件通知
// 通过→模板 SES_TPL_DEV_OK；未通过→模板 SES_TPL_DEV_REJECT
func sendDevSettleEmail(r PlanVisaResult) {
	if r.UID <= 0 {
		return
	}
	var email string
	if err := db.QueryRow("SELECT email FROM userdata WHERE UID = ?", r.UID).Scan(&email); err != nil || email == "" {
		resultText := "未通过"
		if r.OK {
			resultText = "通过"
		}
		log.Printf("⚠️ 用户 UID=%d 申请%s但没有有效通知方式,跳过", r.UID, resultText)
		return
	}

	name := visaTypeName(r.Type)
	vars := map[string]string{
		"name":    name,
		"service": r.Service,
		"reason":  "",
		"date":    nowText(),
	}
	subject, tid := "", ""
	if r.OK {
		tid = os.Getenv("SES_TPL_DEV_OK")
		subject = "FanVerify 开发者申请已通过"
		vars["key"] = r.Token
		vars["nel"] = "1"
	} else {
		tid = os.Getenv("SES_TPL_DEV_REJECT")
		subject = "FanVerify 开发者申请未通过"
	}
	if tid == "" {
		return
	}
	if err := sendSESMail(email, subject, tid, vars); err != nil {
		log.Printf("❌ 开发者结果邮件发送失败 uid=%d: %v", r.UID, err)
		return
	}
	log.Printf("✅ 开发者结果邮件已发送: uid=%d ok=%v to=%s", r.UID, r.OK, maskEmail(email))
}

// sendServiceStatusEmail 发送服务状态变化通知（模板 SES_TPL_SERVICE，变量 content/level/data/time/date）
func sendServiceStatusEmail(toEmail string, content, level, data, duration string) error {
	vars := map[string]string{
		"content": content,
		"level":   level,
		"data":    data,
		"time":    duration,
		"date":    nowText(),
	}
	return sendSESMail(toEmail, "FanVerify 服务状态变化通知", os.Getenv("SES_TPL_SERVICE"), vars)
}

// maskEmail 邮箱脱敏（abc***@example.com）
func maskEmail(email string) string {
	at := -1
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			at = i
			break
		}
	}
	if at <= 3 {
		return "***@" + email[at+1:]
	}
	return email[:3] + "***" + email[at:]
}

// isVaildEmail 校验邮箱格式与长度（不超过 30 位）
func isVaildEmail(email string) bool {
	if len(email) == 0 || len(email) > 30 {
		return false
	}
	for i := 0; i < len(email); i++ {
		c := email[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if ok || c == '.' || c == '_' || c == '%' || c == '+' || c == '-' || c == '@' {
			continue
		}
		return false
	}
	at := -1
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			at = i
		}
	}
	// 必须有 @ 且 @ 前后非空，且点出现在 @ 之后
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	if !strings.Contains(email[at+1:], ".") {
		return false
	}
	return true
}
