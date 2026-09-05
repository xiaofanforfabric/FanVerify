package handlers

import (
	"database/sql"
	"encoding/json"
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

// NeedDevData 开发者入驻申请业务数据（对应 dev_visa_list 一行）
type NeedDevData struct {
	Type        string `json:"type"`           // web / javamc / cmc / none(其他)
	OtherType   string `json:"other_type"`     // 选"其他"时填写的具体类型（需人工审核）
	ServerIP    string `json:"serverip"`       // 服务器请求出口 IP（仅 IPv4）
	ServiceAddr string `json:"service_adderss"` // 业务地址：IP:端口 或 域名
	ConnectType string `json:"connect_type"`   // http / wsc(正向WS) / wss(反向WS)
	Tag         string `json:"tag"`            // 业务备注（必填）
}

// NeedDevRequest 申请接口请求体 { token, cap_token, data }
type NeedDevRequest struct {
	Token    string          `json:"token"`     // JWT；Web 端存 HttpOnly Cookie 时可为空，自动回退读 Cookie
	CapToken string          `json:"cap_token"` // 人机验证 token
	Data     json.RawMessage `json:"data"`      // 业务数据（兼容对象 {…} 或数组 [{…}]）
}

// devVisaMax 每个帐号最多可创建的业务密钥数量
const devVisaMax = 10

// HandleNeedDevKey 开发者入驻申请接口
// 处理流程：
//
//	1. 人机验证 cap_token
//	2. 校验登录令牌（body.token，空则回退 HttpOnly Cookie），并做省份一致性校验
//	3. 查库获取最新用户状态（不用 JWT 缓存的 status）
//	4. 字段级校验（服务类型/IPv4/业务地址/连接类型/备注）
//	5. 业务查重：dev_visa_list 中相同服务器出口IP + 相同业务地址（两个都相同）
//	6. devdata 冻结检查：该用户(BUID)名下令牌 status 是否为 ice（删除冻结状态）
//	7. 额度检查：每帐号最多 devVisaMax 个业务密钥
//	8. 写入 dev_visa_list 申请，并创建计划任务写入 plan_list（供保活检测模块）
//	9. 返回 200 进入审核状态
//
// 返回码：
//
//	200 申请成功，业务进入审核状态
//	400 表单数据不合法
//	401 访问令牌无效，登录可能已过期
//	403 帐号状态有误（被封禁 / 名下存在删除冻结记录）
//	422 相同业务在审核中或删除冷却期
//	429 可申请额度已用完
func HandleNeedDevKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		var req NeedDevRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}

		// 1. 先做一次人机验证（核对是不是机器人）
		secret := os.Getenv("CAP_SECRET")
		if secret == "" {
			log.Println("❌ 环境变量 CAP_SECRET 未设置")
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
			return
		}
		capOK, err := verifyCaptcha(secret, req.CapToken)
		if err != nil {
			log.Printf("❌ 人机验证请求失败: %v", err)
			writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "人机验证服务异常"})
			return
		}
		if !capOK {
			log.Printf("❌ 开发者申请: 人机验证未通过")
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "人机验证未通过，请重新验证"})
			return
		}

		// 2. 校验登录令牌：body.token 优先；空则回退 HttpOnly Cookie（Web 前端 JS 读不到 Cookie）
		token := req.Token
		if token == "" {
			if cookie, err := r.Cookie("fanverify_token"); err == nil {
				token = cookie.Value
			}
		}
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			log.Println("❌ 环境变量 JWT_SECRET 未设置")
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
			return
		}
		claims, err := verifyJWT(jwtSecret, token)
		if err != nil || len(claims.UserData) == 0 {
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "您的访问令牌无效，登录可能已过期"})
			return
		}
		// 省份一致性校验：以省为基准，跨省视为 token 被挪用（与 HandleMe 一致）
		if claims.Province != "" {
			currentProvince := queryProvince(clientIP(r))
			if currentProvince != "" && currentProvince != claims.Province {
				log.Printf("⚠️ 省份变化，拒绝开发者申请: 绑定省=%s 当前省=%s", claims.Province, currentProvince)
				writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "您的访问令牌无效，登录可能已过期"})
				return
			}
		}

		uid := claims.UserData[0].UID
		openid := claims.UserData[0].OpenID

		// 3. 查库获取最新用户状态（不信任 JWT 内缓存状态）
		var dbStatus, dbOpenID string
		err = db.QueryRow("SELECT status, openid FROM userdata WHERE UID = ?", uid).Scan(&dbStatus, &dbOpenID)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "您的帐号状态有误，请确认"})
			return
		}
		if err != nil {
			log.Printf("❌ 开发者申请: 查询用户失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		if dbStatus != "ok" {
			log.Printf("❌ 开发者申请: 帐号状态异常 UID=%d status=%s", uid, dbStatus)
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "您的帐号状态有误，请确认"})
			return
		}
		if openid == "" {
			openid = dbOpenID
		}

		// 解析业务数据：兼容 data:{...} 与 data:[{...}]
		d, err := parseNeedDevData(req.Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "业务数据格式错误"})
			return
		}
		d.normalize()

		// 4. 字段级校验
		if msg := validateNeedDevData(d); msg != "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: msg})
			return
		}

		// 5. 业务查重：相同服务器出口IP + 相同业务地址（两个都要相同）→ 422
		var dupCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM dev_visa_list WHERE uid = ? AND serverip = ? AND service_adderss = ? AND status <> 'fail'",
			uid, d.ServerIP, d.ServiceAddr,
		).Scan(&dupCount)
		if err != nil {
			log.Printf("❌ 开发者申请: 查重失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		if dupCount > 0 {
			log.Printf("❌ 开发者申请: 相同业务已存在 UID=%d serverip=%s addr=%s", uid, d.ServerIP, d.ServiceAddr)
			writeJSON(w, http.StatusUnprocessableEntity, ApiResponse{Success: false, Error: "您的该业务在删除冷却期或审核状态"})
			return
		}

		// 6. devdata 冻结检查：该用户(BUID)名下令牌 status 是否以 ice 开头（删除冻结状态，时间戳格式）→ 403
		var iceCount int
		err = db.QueryRow("SELECT COUNT(*) FROM devdata WHERE BUID = ? AND status LIKE 'ice%'", uid).Scan(&iceCount)
		if err != nil {
			log.Printf("❌ 开发者申请: 冻结检查失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		if iceCount > 0 {
			log.Printf("❌ 开发者申请: 帐号处于删除冻结期 UID=%d", uid)
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "您的帐号状态有误，请确认（存在删除冻结记录，请等待冷却）"})
			return
		}

		// 7. 额度检查：每个帐号最多 devVisaMax 个业务密钥 → 429
		var quota int
		// 额度只计算进行中/已通过的申请（fail 未通过 / ice 已删除 不占额度）
		err = db.QueryRow("SELECT COUNT(*) FROM dev_visa_list WHERE uid = ? AND status IN ('wait', 'ok')", uid).Scan(&quota)
		if err != nil {
			log.Printf("❌ 开发者申请: 额度查询失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		if quota >= devVisaMax {
			log.Printf("❌ 开发者申请: 额度已用完 UID=%d quota=%d", uid, quota)
			writeJSON(w, http.StatusTooManyRequests, ApiResponse{Success: false, Error: "您的可申请额度已经用完"})
			return
		}

		// 8. 写入 dev_visa_list 申请列表
		now := time.Now()
		endTime := fmt.Sprintf("%d", now.Add(24*time.Hour).UnixMilli()) // 审核结束 = 申请时间 + 1 天
		adminSee := "no"
		if d.Type == "none" {
			adminSee = "yes" // 其他类型需人工审核
		}
		res, err := db.Exec(
			`INSERT INTO dev_visa_list
			 (openid, uid, type, other_type, serverip, service_adderss, connect_type, tag, status, end_time, admin_see, plan_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'wait', ?, ?, '')`,
			openid, uid, d.Type, d.OtherType, d.ServerIP, d.ServiceAddr, d.ConnectType, d.Tag, endTime, adminSee,
		)
		if err != nil {
			log.Printf("❌ 开发者申请: 写入 dev_visa_list 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		visaID, _ := res.LastInsertId()

		// 回填 plan_id 并创建计划任务（供后续保活检测模块轮询）。
		// 人工审核类业务（type=none/其他）不创建计划任务：plan_id 保持 NULL，不能按 1 天自动判过
		var planID string
		if d.Type == "none" {
			db.Exec("UPDATE dev_visa_list SET plan_id = NULL WHERE id = ?", visaID)
		} else {
			planID = strconv.FormatInt(visaID, 10)
			db.Exec("UPDATE dev_visa_list SET plan_id = ? WHERE id = ?", planID, visaID)
			if _, err = db.Exec(
				"INSERT INTO plan_list (plan_id, last_see_time, output) VALUES (?, ?, '')",
				planID, fmt.Sprintf("%d", now.UnixMilli()),
			); err != nil {
				log.Printf("⚠️ 开发者申请: 写入 plan_list 失败(不影响申请): %v", err)
			}
		}

		// 9. 返回成功
		log.Printf("✅ 开发者申请已提交: UID=%d type=%s serverip=%s addr=%s visa=%d admin_see=%s", uid, d.Type, d.ServerIP, d.ServiceAddr, visaID, adminSee)
		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"message":  "申请成功，您的业务已进入审核状态，请耐心等待通知",
				"plan_id":  planID,
				"end_time": endTime,
			},
		})
	}
}

// parseNeedDevData 解析业务数据：兼容 data:{...} 与 data:[{...}]
func parseNeedDevData(raw json.RawMessage) (*NeedDevData, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("业务数据为空")
	}
	var d NeedDevData
	if err := json.Unmarshal(raw, &d); err == nil && d.Type != "" {
		return &d, nil
	}
	var arr []NeedDevData
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return &arr[0], nil
	}
	return nil, fmt.Errorf("业务数据格式错误")
}

// normalize 清理字段首尾空格，空连接类型默认 http
func (d *NeedDevData) normalize() {
	d.Type = strings.ToLower(strings.TrimSpace(d.Type))
	d.OtherType = strings.TrimSpace(d.OtherType)
	d.ServerIP = strings.TrimSpace(d.ServerIP)
	d.ServiceAddr = strings.TrimSpace(d.ServiceAddr)
	d.ConnectType = strings.ToLower(strings.TrimSpace(d.ConnectType))
	d.Tag = strings.TrimSpace(d.Tag)
	if d.ConnectType == "" {
		d.ConnectType = "http"
	}
}

// isIPv4 校验是否为合法 IPv4 地址
func isIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// validateNeedDevData 字段校验，返回错误描述；合法返回空串
func validateNeedDevData(d *NeedDevData) string {
	switch d.Type {
	case "web", "javamc", "cmc", "none":
	default:
		return "服务类型不合法"
	}
	if d.Type == "none" && d.OtherType == "" {
		return "请填写具体业务类型（人工审核）"
	}
	if d.ServerIP == "" {
		return "请填写服务器请求出口 IP"
	}
	if !isIPv4(d.ServerIP) {
		return "服务器请求出口 IP 必须是合法的 IPv4 地址"
	}
	if d.ServiceAddr == "" {
		return "请填写业务地址（IP:端口 或 域名）"
	}
	if strings.ContainsAny(d.ServiceAddr, " \t\r\n") {
		return "业务地址不能包含空格"
	}
	switch d.ConnectType {
	case "http", "wsc", "wss":
	default:
		return "连接类型不合法"
	}
	if d.Tag == "" {
		return "请填写业务备注"
	}
	return ""
}
