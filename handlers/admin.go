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

// ================= 管理员（root）模块 =================
// 说明：仅“验证等级为 0”的用户（视为 root）可调用以下 /api/admin/* 接口。

// adminAuth 管理员鉴权：仅 level==0 且状态正常(ok)的 root 可放行。
// token 来源优先级：请求体 token > URL query token > HttpOnly Cookie。
func adminAuth(w http.ResponseWriter, r *http.Request, bodyToken string) (int, bool) {
	token := bodyToken
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		if c, err := r.Cookie("fanverify_token"); err == nil {
			token = c.Value
		}
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "服务器配置错误"})
		return 0, false
	}
	claims, err := verifyJWT(secret, token)
	if err != nil || len(claims.UserData) == 0 {
		writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录已过期，请重新登录"})
		return 0, false
	}
	uid := claims.UserData[0].UID

	var level int
	var status string
	err = db.QueryRow("SELECT level, status FROM userdata WHERE UID = ?", uid).Scan(&level, &status)
	if err != nil || level != 0 || status != "ok" {
		log.Printf("🚫 管理员鉴权失败: UID=%d level=%d status=%s", uid, level, status)
		writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "仅管理员(root)可调用该接口"})
		return 0, false
	}
	return uid, true
}

// ---------- 1. 获取所有用户信息 ----------

// AdminUserItem 管理员视角的用户信息（手机号/邮箱脱敏，openid 不下发）
type AdminUserItem struct {
	UID       int    `json:"uid"`
	Level     int    `json:"level"`
	CodeLevel int    `json:"code_level"`
	RegTime   string `json:"reg_time"`
	Tag       string `json:"tag"`
	Status    string `json:"status"`
	Phone     string `json:"phone"` // 脱敏 199****2460
	QQ        string `json:"qq"`
	Email     string `json:"email"` // 脱敏
}

// HandleAdminGetAllUser 获取所有用户信息（手机号/邮箱脱敏；不返回 openid）
func HandleAdminGetAllUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		if _, ok := adminAuth(w, r, ""); !ok {
			return
		}

		rows, err := db.Query("SELECT UID, level, code_level, reg_time, tag, status, phone, qq, email FROM userdata ORDER BY UID ASC")
		if err != nil {
			log.Printf("❌ 管理员: 查询用户失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		defer rows.Close()

		users := make([]AdminUserItem, 0)
		for rows.Next() {
			var u AdminUserItem
			if err := rows.Scan(&u.UID, &u.Level, &u.CodeLevel, &u.RegTime, &u.Tag, &u.Status, &u.Phone, &u.QQ, &u.Email); err != nil {
				continue
			}
			if u.Phone != "" {
				u.Phone = maskPhone(u.Phone)
			}
			if u.Email != "" {
				u.Email = maskEmail(u.Email)
			}
			users = append(users, u)
		}
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"total": len(users),
			"users": users,
		}})
		log.Printf("🔧 管理员: 获取全部用户 %d 条", len(users))
	}
}

// ---------- 2. 封禁/解封用户 ----------

// AdminBanRequest 封禁请求（POST）
type AdminBanRequest struct {
	Token string `json:"token"`
	UID   string `json:"uid"`
}

// HandleAdminBanUser 封禁/解封用户
// POST {token, uid}           → 封禁
// GET  ?uid=xxx&token=yyy     → 解封
func HandleAdminBanUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req AdminBanRequest
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
				return
			}
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
				return
			}
			if req.UID == "" {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "uid 不能为空"})
				return
			}
			adminUID, ok := adminAuth(w, r, req.Token)
			if !ok {
				return
			}
			// 封禁
			uid, err := strconv.Atoi(req.UID)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "uid 必须是数字"})
				return
			}
			if uid == adminUID {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "不能封禁自己(管理员)"})
				return
			}
			if !userExists(uid) {
				writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "用户不存在"})
				return
			}
			db.Exec("UPDATE userdata SET status = 'bad' WHERE UID = ?", uid)
			writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{"message": fmt.Sprintf("已封禁用户 #%d", uid), "uid": uid}})
			log.Printf("🔧 管理员 #%d 封禁用户 #%d", adminUID, uid)

		case http.MethodGet:
			uidStr := r.URL.Query().Get("uid")
			if uidStr == "" {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "缺少 uid 参数"})
				return
			}
			adminUID, ok := adminAuth(w, r, "")
			if !ok {
				return
			}
			uid, err := strconv.Atoi(uidStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "uid 必须是数字"})
				return
			}
			if !userExists(uid) {
				writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "用户不存在"})
				return
			}
			db.Exec("UPDATE userdata SET status = 'ok' WHERE UID = ?", uid)
			writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{"message": fmt.Sprintf("已解封用户 #%d", uid), "uid": uid}})
			log.Printf("🔧 管理员 #%d 解封用户 #%d", adminUID, uid)

		default:
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST(封禁) / GET(解封)"})
		}
	}
}

// userExists 判断用户是否存在
func userExists(uid int) bool {
	var one int
	err := db.QueryRow("SELECT 1 FROM userdata WHERE UID = ?", uid).Scan(&one)
	return err == nil
}

// ---------- 3. 获取所有开发者令牌及其归属 ----------

// AdminDevItem 管理员视角的开发者令牌（含归属用户，openid 不下发）
type AdminDevItem struct {
	ID          int64  `json:"id"`
	DevToken    string `json:"devaccesstoken"`
	NEL         int    `json:"nel"`
	BUID        int    `json:"buid"`
	Mode        string `json:"mode"`
	BIP         string `json:"bip"`
	SME         string `json:"sme"`
	Status      string `json:"status"`
	DateOfIssue string `json:"date_of_issue"`
	NOV         int    `json:"nov"`
	OwnerTag    string `json:"owner_tag"`
	OwnerEmail  string `json:"owner_email"`
}

// HandleAdminGetDev 获取所有开发者令牌及归属
func HandleAdminGetDev() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		if _, ok := adminAuth(w, r, ""); !ok {
			return
		}

		rows, err := db.Query(`SELECT d.id, d.devaccesstoken, d.NEL, d.BUID, d.mode, d.BIP, d.SME, d.status,
			d.Date_of_Issue, d.NOV, u.tag, u.email
			FROM devdata d LEFT JOIN userdata u ON u.UID = d.BUID ORDER BY d.id DESC`)
		if err != nil {
			log.Printf("❌ 管理员: 查询开发者令牌失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		defer rows.Close()

		devs := make([]AdminDevItem, 0)
		for rows.Next() {
			var d AdminDevItem
			var ownerEmail sql.NullString
			if err := rows.Scan(&d.ID, &d.DevToken, &d.NEL, &d.BUID, &d.Mode, &d.BIP, &d.SME, &d.Status,
				&d.DateOfIssue, &d.NOV, &d.OwnerTag, &ownerEmail); err != nil {
				continue
			}
			if ownerEmail.Valid && ownerEmail.String != "" {
				d.OwnerEmail = maskEmail(ownerEmail.String)
			}
			devs = append(devs, d)
		}
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"total": len(devs),
			"devs":  devs,
		}})
		log.Printf("🔧 管理员: 获取开发者令牌 %d 条", len(devs))
	}
}

// ---------- 4. 冻结/解冻开发者令牌 ----------

// AdminEditDevRequest 冻结/解冻请求
type AdminEditDevRequest struct {
	DevKey string `json:"devkey"`
	Type   string `json:"type"` // ice=冻结 / unice=解冻
}

// HandleAdminEditDev 冻结(ice)/解冻(unice)开发者令牌
func HandleAdminEditDev() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}
		if _, ok := adminAuth(w, r, ""); !ok {
			return
		}
		var req AdminEditDevRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}
		req.DevKey = strings.TrimSpace(req.DevKey)
		typ := strings.ToLower(strings.TrimSpace(req.Type))
		if req.DevKey == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "devkey 不能为空"})
			return
		}
		if typ != "ice" && typ != "unice" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "type 需为 ice(冻结) 或 unice(解冻)"})
			return
		}

		var exists int
		if err := db.QueryRow("SELECT 1 FROM devdata WHERE devaccesstoken = ?", req.DevKey).Scan(&exists); err != nil {
			writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "令牌不存在"})
			return
		}

		message := ""
		if typ == "ice" {
			newStatus := "ice" + fmt.Sprintf("%d", time.Now().UnixMilli()) // 冻结(带时间戳)
			db.Exec("UPDATE devdata SET status = ? WHERE devaccesstoken = ?", newStatus, req.DevKey)
			message = "已冻结该令牌（OpenAPI 调用将被拒绝）"
		} else {
			db.Exec("UPDATE devdata SET status = 'ok' WHERE devaccesstoken = ?", req.DevKey)
			message = "已解冻该令牌"
		}
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"message": message,
			"devkey":  req.DevKey,
			"type":    typ,
		}})
		log.Printf("🔧 管理员: %s devkey=%s", message, req.DevKey)
	}
}

// ---------- 5. 查看所有申请 ----------

// AdminVisaItem 管理员视角的申请（含检测进度与审查状态）
type AdminVisaItem struct {
	ID          int64   `json:"id"`
	UID         int     `json:"uid"`
	Type        string  `json:"type"`
	OtherType   string  `json:"other_type"`
	ServerIP    string  `json:"serverip"`
	ServiceAddr string  `json:"service_adderss"`
	ConnectType string  `json:"connect_type"`
	Tag         string  `json:"tag"`
	Status      string  `json:"status"`
	EndTime     string  `json:"end_time"`
	AdminSee    string  `json:"admin_see"`
	Reviewed    string  `json:"reviewed"` // yes=管理员已审查 / no=未审查(前端标红)
	PlanID      *string `json:"plan_id"`
	Online      int     `json:"online"`
	Offline     int     `json:"offline"`
}

// HandleAdminVisaList 查看所有申请（未审查的 reviewed=no，前端标红）
func HandleAdminVisaList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		if _, ok := adminAuth(w, r, ""); !ok {
			return
		}

		rows, err := db.Query(`SELECT v.id, v.uid, v.type, v.other_type, v.serverip, v.service_adderss,
			v.connect_type, v.tag, v.status, v.end_time, v.admin_see, v.reviewed, v.plan_id, p.output
			FROM dev_visa_list v LEFT JOIN plan_list p ON p.plan_id = v.plan_id
			ORDER BY v.id DESC`)
		if err != nil {
			log.Printf("❌ 管理员: 查询申请失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		defer rows.Close()

		visas := make([]AdminVisaItem, 0)
		unreviewed := 0
		for rows.Next() {
			var v AdminVisaItem
			var planID, output sql.NullString
			if err := rows.Scan(&v.ID, &v.UID, &v.Type, &v.OtherType, &v.ServerIP, &v.ServiceAddr,
				&v.ConnectType, &v.Tag, &v.Status, &v.EndTime, &v.AdminSee, &v.Reviewed, &planID, &output); err != nil {
				continue
			}
			if planID.Valid {
				pid := planID.String
				v.PlanID = &pid
			}
			if output.Valid && output.String != "" {
				total, okC := parsePlanOutput(output.String)
				v.Online = okC
				v.Offline = total - okC
			}
			if v.Reviewed == "no" {
				unreviewed++
			}
			visas = append(visas, v)
		}
		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"total":       len(visas),
			"unreviewed":  unreviewed,
			"visas":       visas,
		}})
		log.Printf("🔧 管理员: 查看全部申请 %d 条(未审查 %d)", len(visas), unreviewed)
	}
}

// ---------- 6. 处理申请（pass/refused） ----------

// AdminEditVisaRequest 申请处理请求
type AdminEditVisaRequest struct {
	ID   string `json:"id"`
	Type string `json:"type"` // pass=已审查/人工通过；refused=直接拒绝(不签发)
}

// HandleAdminEditVisa 管理员处理申请
// pass    → 标记已审查；若为人工审核类(admin_see=yes / 无计划)则直接批准并签发；
//          自动检测类则仅标记已审查，等待保活检测完成后自动签发
// refused → 直接拒绝（删除计划任务，防止自动结算误签发），不颁发密钥
func HandleAdminEditVisa() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}
		if _, ok := adminAuth(w, r, ""); !ok {
			return
		}
		var req AdminEditVisaRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}
		typ := strings.ToLower(strings.TrimSpace(req.Type))
		if typ != "pass" && typ != "refused" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "type 需为 pass 或 refused"})
			return
		}
		id, err := strconv.ParseInt(strings.TrimSpace(req.ID), 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "id 必须是数字"})
			return
		}

		// 加载申请行
		var (
			visaUID                 int
			vType                   string
			serverip, service, conn string
			tag, status, planID     string
			adminSee                string
		)
		err = db.QueryRow(
			`SELECT uid, type, serverip, service_adderss, connect_type, tag, status, plan_id, admin_see
			 FROM dev_visa_list WHERE id = ?`, id,
		).Scan(&visaUID, &vType, &serverip, &service, &conn, &tag, &status, &planID, &adminSee)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "申请不存在"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		if status != "wait" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: fmt.Sprintf("该申请当前状态为 %s，无法处理", status)})
			return
		}

		result := PlanVisaResult{
			UID: int64(visaUID), VisaID: id, Type: vType, ServerIP: serverip,
			Service: service, Tag: tag,
		}

		message := ""
		if typ == "pass" {
			db.Exec("UPDATE dev_visa_list SET reviewed = 'yes' WHERE id = ?", id)
			if adminSee == "yes" || planID == "" {
				// 人工审核类：直接批准并签发
				result.OK = true
				result.Online = 12
				result.Total = 12
				settlePlan(result, planID, conn)
				message = "已批准（人工审核通过）并颁发密钥"
			} else {
				// 自动检测类：标记已审查，等待保活检测完成后自动签发
				message = "已标记已审阅，等待保活检测完成后自动签发"
			}
		} else {
			// 直接拒绝：不签发，并清理计划任务防止自动结算误签发
			db.Exec("UPDATE dev_visa_list SET reviewed = 'yes' WHERE id = ?", id)
			result.OK = false
			result.Online = 0
			result.Total = 12
			settlePlan(result, planID, conn)
			message = "已拒绝该申请，不会签发密钥"
		}

		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"message": message,
			"id":      id,
			"type":    typ,
		}})
		log.Printf("🔧 管理员: 处理申请 id=%d type=%s → %s", id, typ, message)
	}
}
