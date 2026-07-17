package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

// ApiResponse API 响应结构
type ApiResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// HandleGenerateOTP 生成 OTP
func HandleGenerateOTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		// 获取 accesstoken（中间件已验证）
		accessToken := r.URL.Query().Get("accesstoken")

		otp := generateOTP(16)
		lostdate := fmt.Sprintf("%d", time.Now().Add(150*time.Second).UnixMilli())

		_, err := db.Exec("INSERT INTO otpdata (otp, lostdate, UID, band_time, devaccesstoken) VALUES (?, ?, 0, '', ?)", otp, lostdate, accessToken)
		if err != nil {
			log.Printf("❌ 写入 OTP 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成失败"})
			return
		}

		log.Printf("🆔 生成 OTP: %s, 过期时间: %s", otp, lostdate)

		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"otp": otp,
			},
		})
	}
}

// HandleGenerateQRCode 生成二维码
func HandleGenerateQRCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		otp := r.URL.Query().Get("otp")
		if otp == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "otp 参数不能为空"})
			return
		}

		qrContent := fmt.Sprintf("https://api.fanverify.cn/openapi/otp?otp=%s", otp)

		qr, err := qrcode.New(qrContent, qrcode.Medium)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成二维码失败"})
			return
		}

		qr.DisableBorder = false
		pngBytes, err := qr.PNG(256)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成 PNG 失败"})
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pngBytes)))
		w.Write(pngBytes)

		log.Printf("🖼️ 生成二维码: OTP=%s", otp)
	}
}

// HandleSeeOTP 查询 OTP 状态
func HandleSeeOTP() http.HandlerFunc {
	lastQuery := make(map[string]int64)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		otp := r.URL.Query().Get("otp")
		if otp == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "otp 参数不能为空"})
			return
		}

		now := time.Now().Unix()
		if lastTime, exists := lastQuery[otp]; exists && now-lastTime < 5 {
			w.WriteHeader(249)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"rate_limit"}`))
			return
		}
		lastQuery[otp] = now

		var lostdate string
		var uid int
		err := db.QueryRow("SELECT lostdate, UID FROM otpdata WHERE otp = ?", otp).Scan(&lostdate, &uid)
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"404"}`))
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"status":"error"}`))
			log.Printf("❌ 查询 OTP 失败: %v", err)
			return
		}

		lostTimestamp, _ := strconv.ParseInt(lostdate, 10, 64)
		if time.Now().UnixMilli() > lostTimestamp {
			db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"lost"}`))
			log.Printf("⏰ OTP 已过期并删除: %s", otp)
			return
		}

		if uid == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"wait"}`))
			return
		}

		var level int
		var regTime, tag string
		err = db.QueryRow("SELECT level, reg_time, tag FROM userdata WHERE UID = ?", uid).Scan(&level, &regTime, &tag)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"error"}`))
			return
		}

		db.Exec("DELETE FROM otpdata WHERE otp = ?", otp)

		response := map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{{
				"uid":      uid,
				"level":    strconv.Itoa(level),
				"reg_time": regTime,
				"tag":      tag,
			}},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ OTP 验证成功并删除: %s, UID: %d", otp, uid)
	}
}

// HandleUserVerify 用户验证接口
func HandleUserVerify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"Method Not Allowed"}`))
			return
		}

		uidStr := r.URL.Query().Get("uid")
		passCode := r.URL.Query().Get("pass_code")

		if uidStr == "" || passCode == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Bad Request"}`))
			return
		}

		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Bad Request"}`))
			return
		}

		var regTime, tmpCode, codeLostTime, tag string
		var level int
		err = db.QueryRow(
			"SELECT reg_time, level, tmp_code, code_lost_time, tag FROM userdata WHERE UID = ?",
			uid,
		).Scan(&regTime, &level, &tmpCode, &codeLostTime, &tag)

		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"Not Found"}`))
			log.Printf("❌ 用户验证失败: 用户不存在 UID=%d", uid)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal Server Error"}`))
			log.Printf("❌ 用户验证失败: 查询错误 %v", err)
			return
		}

		if tmpCode != passCode {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"Forbidden"}`))
			log.Printf("❌ 用户验证失败: 验证码不匹配 UID=%d", uid)
			return
		}

		if codeLostTime != "" {
			lostTimestamp, err := strconv.ParseInt(codeLostTime, 10, 64)
			if err == nil && time.Now().UnixMilli() > lostTimestamp {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"Forbidden"}`))
				log.Printf("❌ 用户验证失败: 验证码已过期 UID=%d", uid)
				return
			}
		}

		response := map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{{
				"uid":      uid,
				"level":    strconv.Itoa(level),
				"reg_time": regTime,
				"tag":      tag,
			}},
		}

		// 验证成功，更新开发者的 PUID
		accessToken := r.URL.Query().Get("accesstoken")
		if accessToken != "" {
			var currentPUID sql.NullString
			err = db.QueryRow("SELECT PUID FROM devdata WHERE devaccesstoken = ?", accessToken).Scan(&currentPUID)
			if err == nil {
				puidValue := ""
				if currentPUID.Valid {
					puidValue = currentPUID.String
				}
				// 构造新的 PUID：检查是否已存在
				newUID := strconv.Itoa(uid)
				if !strings.Contains(puidValue, newUID) {
					if puidValue == "" {
						puidValue = newUID
					} else {
						puidValue = puidValue + ";" + newUID
					}
					db.Exec("UPDATE devdata SET PUID = ? WHERE devaccesstoken = ?", puidValue, accessToken)
					log.Printf("✅ 更新 PUID: %s, 添加 UID=%d", accessToken, uid)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 用户验证成功: UID=%d, Level=%d", uid, level)
	}
}

// GetUserDataRequest 获取用户数据请求
type GetUserDataRequest struct {
	AccessToken string `json:"accesstoken"`
	UID         string `json:"uid"`
}

// HandleGetUserData 获取已验证用户的数据
func HandleGetUserData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		var req GetUserDataRequest
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

		if req.AccessToken == "" || req.UID == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}

		// 查询 devdata 表中的 PUID 字段
		var puid sql.NullString
		err = db.QueryRow("SELECT PUID FROM devdata WHERE devaccesstoken = ?", req.AccessToken).Scan(&puid)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"code":403}`))
			log.Printf("❌ 获取用户数据失败: AccessToken 不存在")
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500}`))
			log.Printf("❌ 查询 devdata 失败: %v", err)
			return
		}

		// 检查 PUID 中是否包含请求的 UID
		puidValue := ""
		if puid.Valid {
			puidValue = puid.String
		}
		if !strings.Contains(puidValue, req.UID) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"code":403}`))
			log.Printf("❌ 获取用户数据失败: UID=%s 未在 PUID 中", req.UID)
			return
		}

		// 将 UID 转换为整数
		uid, err := strconv.Atoi(req.UID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400}`))
			return
		}

		// 查询用户数据
		var level int
		var regTime, tag string
		err = db.QueryRow("SELECT level, reg_time, tag FROM userdata WHERE UID = ?", uid).Scan(&level, &regTime, &tag)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"code":404}`))
			log.Printf("❌ 用户不存在: UID=%d", uid)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500}`))
			log.Printf("❌ 查询用户数据失败: %v", err)
			return
		}

		// 返回用户数据
		response := map[string]interface{}{
			"status": "ok",
			"data": []map[string]interface{}{{
				"uid":      uid,
				"level":    strconv.Itoa(level),
				"reg_time": regTime,
				"tag":      tag,
			}},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 获取用户数据成功: UID=%d", uid)
	}
}

// HandleDevInfo 获取开发者令牌信息
func HandleDevInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		// 从 URL 参数获取 accesstoken
		accessToken := r.URL.Query().Get("accesstoken")
		if accessToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":401}`))
			return
		}

		// 查询开发者令牌信息
		var nel, buid int
		var mode, dateOfIssue, status string
		var sme sql.NullString

		err := db.QueryRow(
			"SELECT NEL, BUID, mode, Date_of_Issue, SME, status FROM devdata WHERE devaccesstoken = ?",
			accessToken,
		).Scan(&nel, &buid, &mode, &dateOfIssue, &sme, &status)

		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":401}`))
			log.Printf("❌ 获取开发者信息失败: AccessToken 不存在")
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500}`))
			log.Printf("❌ 查询 devdata 失败: %v", err)
			return
		}

		// 处理 SME 字段（可能为 NULL）
		smeValue := ""
		if sme.Valid {
			smeValue = sme.String
		}

		// 返回开发者信息
		response := map[string]interface{}{
			"need_end_level":    nel,
			"bind_uid":          buid,
			"mode":              mode,
			"Date_of_Issue":     dateOfIssue,
			"service_message":   smeValue,
			"status":            status,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 获取开发者信息成功: %s", accessToken)
	}
}

// TagRequest 打标签请求
type TagRequest struct {
	AccessToken string `json:"accesstoken"`
	TUID        string `json:"tuid"`
	Tag         string `json:"tag"`
	Message     string `json:"message"`
}

// HandleTag 开发者给用户打标签
func HandleTag() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405}`))
			return
		}

		var req TagRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"请求解析失败"}`))
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"请求解析失败"}`))
			return
		}

		// 获取 accesstoken（中间件已验证）
		accessToken := req.AccessToken
		if accessToken == "" {
			accessToken = r.URL.Query().Get("accesstoken")
		}

		if accessToken == "" || req.TUID == "" || req.Tag == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"accesstoken、tuid 和 tag 不能为空"}`))
			return
		}

		// 检查标签长度（10字节以内）
		tagByteLength := 0
		for _, r := range req.Tag {
			if r < 128 {
				tagByteLength++ // 英文 1 字节
			} else {
				tagByteLength += 2 // 汉字 2 字节
			}
		}
		if tagByteLength > 10 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"标签过长"}`))
			log.Printf("❌ 打标签失败: 标签过长 (%d 字节)", tagByteLength)
			return
		}

		// 查询开发者的 NOV
		var nov int
		err = db.QueryRow("SELECT NOV FROM devdata WHERE devaccesstoken = ?", accessToken).Scan(&nov)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":401,"message":"无效的开发者令牌"}`))
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"message":"内部错误"}`))
			log.Printf("❌ 查询 NOV 失败: %v", err)
			return
		}

		// 检查 NOV 是否大于 1000
		if nov < 1000 {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(fmt.Sprintf(`{"code":403,"message":"你当前有效认证次数为%d，未达到最低的1000"}`, nov)))
			log.Printf("❌ 打标签失败: NOV=%d 不足 1000", nov)
			return
		}

		// 转换 TUID
		tuid, err := strconv.Atoi(req.TUID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"tuid 必须是数字"}`))
			return
		}

		// 检查用户是否存在
		var userExists int
		err = db.QueryRow("SELECT UID FROM userdata WHERE UID = ?", tuid).Scan(&userExists)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"code":400,"message":"用户不存在"}`))
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"message":"内部错误"}`))
			log.Printf("❌ 查询用户失败: %v", err)
			return
		}

		// 扣除 1000 NOV
		_, err = db.Exec("UPDATE devdata SET NOV = NOV - 1000 WHERE devaccesstoken = ?", accessToken)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"message":"内部错误"}`))
			log.Printf("❌ 扣除 NOV 失败: %v", err)
			return
		}

		// 更新用户的 tag 字段
		_, err = db.Exec("UPDATE userdata SET tag = ? WHERE UID = ?", req.Tag, tuid)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"message":"内部错误"}`))
			log.Printf("❌ 更新用户 tag 失败: %v", err)
			return
		}

		// 在 tagdata 表中留存记录
		_, err = db.Exec(
			"INSERT INTO tagdata (tag, from_dev, tuid, status, message) VALUES (?, ?, ?, 'ok', ?)",
			req.Tag, accessToken, tuid, req.Message,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":500,"message":"内部错误"}`))
			log.Printf("❌ 插入 tagdata 失败: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200}`))
		log.Printf("✅ 打标签成功: UID=%d, Tag=%s, Dev=%s", tuid, req.Tag, accessToken)
	}
}

// 辅助函数
func generateOTP(length int) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Read(make([]byte, 1))
		if n > 0 {
			var b [1]byte
			rand.Read(b[:])
			result[i] = charset[int(b[0])%len(charset)]
		}
	}
	return string(result)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
