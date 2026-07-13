package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

		otp := generateOTP(16)
		lostdate := fmt.Sprintf("%d", time.Now().Add(150*time.Second).UnixMilli())

		_, err := db.Exec("INSERT INTO otpdata (otp, lostdate, UID, band_time) VALUES (?, ?, 0, '')", otp, lostdate)
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("✅ 用户验证成功: UID=%d, Level=%d", uid, level)
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
