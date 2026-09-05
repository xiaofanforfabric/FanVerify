package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"os"
)

// DevKeyInfo 可用的开发者密钥信息（devdata 表）
type DevKeyInfo struct {
	ID          int64  `json:"id"`
	DevToken    string `json:"devaccesstoken"`
	NEL         int    `json:"nel"`
	BUID        int    `json:"buid"`
	BIP         string `json:"bip"`
	Mode        string `json:"mode"`
	SME         string `json:"sme"`
	Status      string `json:"status"`
	DateOfIssue string `json:"date_of_issue"`
}

// DevVisaInfo 审核中的开发者入驻申请信息（dev_visa_list 表）
type DevVisaInfo struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	OtherType   string  `json:"other_type"`
	ServerIP    string  `json:"serverip"`
	ServiceAddr string  `json:"service_adderss"`
	ConnectType string  `json:"connect_type"`
	Tag         string  `json:"tag"`
	Status      string  `json:"status"`
	EndTime     string  `json:"end_time"`
	AdminSee    string  `json:"admin_see"`
	PlanID      *string `json:"plan_id"` // NULL = 人工审核（无自动保活检测）
}

// IsDevResponse 开发者状态查询响应
type IsDevResponse struct {
	IsDev    bool          `json:"is_dev"`   // 是否已是开发者：devdata 有可用密钥 或 dev_visa_list 有审核中的申请
	UserData []User        `json:"userdata"` // 用户数据（含 level）
	DevKey   []DevKeyInfo  `json:"dev_key"`  // 所有可用密钥数据
	DevVisa  []DevVisaInfo `json:"dev_visa"` // 所有申请中的申请数据
}

// HandleIsDev 开发者状态查询接口（GET，HttpOnly Cookie JWT 鉴权）
// 返回 { is_dev, userdata, dev_key, dev_visa }，供开发者页面一次性判断状态并展示密钥/申请
func HandleIsDev() http.HandlerFunc {
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
			clearAuthCookie(w)
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录已过期，请重新登录"})
			return
		}
		// 省份一致性校验：以省为基准，跨省视为 token 被挪用
		if claims.Province != "" {
			currentProvince := queryProvince(clientIP(r))
			if currentProvince != "" && currentProvince != claims.Province {
				log.Printf("⚠️ 省份变化，拒绝开发者状态查询: 绑定省=%s 当前省=%s", claims.Province, currentProvince)
				clearAuthCookie(w)
				writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录环境变化，请重新登录"})
				return
			}
		}
		if len(claims.UserData) == 0 {
			clearAuthCookie(w)
			writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "登录信息不完整，请重新登录"})
			return
		}
		uid := claims.UserData[0].UID

		resp := IsDevResponse{UserData: claims.UserData}

		// 1. 可用密钥：devdata 中该用户(BUID)名下，排除 ice（删除冻结）状态的记录
		rows, err := db.Query(
			`SELECT id, devaccesstoken, NEL, BUID, BIP, mode, SME, status, Date_of_Issue
			 FROM devdata WHERE BUID = ? AND status NOT LIKE 'ice%' ORDER BY id DESC`,
			uid,
		)
		if err != nil {
			log.Printf("❌ 开发者状态查询: 读取 devdata 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		for rows.Next() {
			var k DevKeyInfo
			var dateOfIssue string
			if err := rows.Scan(&k.ID, &k.DevToken, &k.NEL, &k.BUID, &k.BIP, &k.Mode, &k.SME, &k.Status, &dateOfIssue); err != nil {
				log.Printf("⚠️ 开发者状态查询: 读取 devdata 行失败: %v", err)
				continue
			}
			k.DateOfIssue = dateOfIssue
			resp.DevKey = append(resp.DevKey, k)
		}
		rows.Close()

		// 2. 审核中的申请：dev_visa_list 中该 uid 名下 status = 'wait'
		rows2, err := db.Query(
			`SELECT id, type, other_type, serverip, service_adderss, connect_type, tag, status, end_time, admin_see, plan_id
			 FROM dev_visa_list WHERE uid = ? AND status = 'wait' ORDER BY id DESC`,
			uid,
		)
		if err != nil {
			log.Printf("❌ 开发者状态查询: 读取 dev_visa_list 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}
		for rows2.Next() {
			var v DevVisaInfo
			var planID sql.NullString
			if err := rows2.Scan(&v.ID, &v.Type, &v.OtherType, &v.ServerIP, &v.ServiceAddr, &v.ConnectType, &v.Tag, &v.Status, &v.EndTime, &v.AdminSee, &planID); err != nil {
				log.Printf("⚠️ 开发者状态查询: 读取 dev_visa_list 行失败: %v", err)
				continue
			}
			if planID.Valid {
				pid := planID.String
				v.PlanID = &pid
			}
			resp.DevVisa = append(resp.DevVisa, v)
		}
		rows2.Close()

		// 3. is_dev：有可用密钥 或 有待审核申请 都算开发者
		resp.IsDev = len(resp.DevKey) > 0 || len(resp.DevVisa) > 0

		writeJSON(w, http.StatusOK, resp)
		log.Printf("✅ 开发者状态查询: UID=%d is_dev=%v keys=%d visas=%d", uid, resp.IsDev, len(resp.DevKey), len(resp.DevVisa))
	}
}
