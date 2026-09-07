package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// EditDevKeyRequest 修改开发者密钥验证等级请求
type EditDevKeyRequest struct {
	Token    string `json:"token"`          // JWT；Web 端 HttpOnly Cookie 时可留空，后端自动读取 Cookie
	DevKey   string `json:"devkey"`         // 要修改的 devaccesstoken
	NewLevel string `json:"new_need_leve"`  // 新的要求验证等级：只能是 1 或 2
}

// HandleEditDevKey 修改开发者密钥（NEL = 要求验证等级）
// 流程：校验 token → 校验 new_need_leve ∈ {1,2} → 校验密钥存在于 devdata 且 status='ok' → 校验归属 → 更新 NEL
// 返回：200 成功 / 400 参数错误 / 401 令牌无效或过期 / 403 无权修改该密钥 / 404 密钥不存在或不可用
func HandleEditDevKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}

		var req EditDevKeyRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}

		// 1. 校验登录令牌：body.token 优先，空则回退 HttpOnly Cookie
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
		// 省份一致性校验
		if claims.Province != "" {
			currentProvince := queryProvince(clientIP(r))
			if currentProvince != "" && currentProvince != claims.Province {
				log.Printf("⚠️ 省份变化，拒绝修改密钥: 绑定省=%s 当前省=%s", claims.Province, currentProvince)
				writeJSON(w, http.StatusUnauthorized, ApiResponse{Success: false, Error: "您的访问令牌无效，登录可能已过期"})
				return
			}
		}
		uid := claims.UserData[0].UID

		// 2. 参数校验
		req.DevKey = strings.TrimSpace(req.DevKey)
		newLevel := strings.TrimSpace(req.NewLevel)
		if req.DevKey == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "devkey 不能为空"})
			return
		}
		if newLevel != "1" && newLevel != "2" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "new_need_leve 只能是 1 或 2"})
			return
		}
		newLevelInt, _ := strconv.Atoi(newLevel)

		// 3. 查询该密钥：必须存在且 status='ok'（可用密钥）
		var buid int
		err = db.QueryRow("SELECT BUID FROM devdata WHERE devaccesstoken = ? AND status = 'ok'", req.DevKey).Scan(&buid)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: "密钥不存在或不可用"})
			return
		}
		if err != nil {
			log.Printf("❌ 修改密钥等级: 查询失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}

		// 4. 归属校验：只能修改自己名下的密钥
		if buid != uid {
			log.Printf("❌ 修改密钥等级: 无权修改他人密钥 UID=%d BUID=%d", uid, buid)
			writeJSON(w, http.StatusForbidden, ApiResponse{Success: false, Error: "无权修改该密钥"})
			return
		}

		// 5. 更新 NEL
		_, err = db.Exec("UPDATE devdata SET NEL = ? WHERE devaccesstoken = ?", newLevelInt, req.DevKey)
		if err != nil {
			log.Printf("❌ 修改密钥等级: 更新失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "内部错误"})
			return
		}

		log.Printf("✅ 密钥验证等级已修改: UID=%d token=%s new_level=%d", uid, req.DevKey, newLevelInt)
		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"message": "密钥要求验证等级已更新为 Lv." + newLevel,
				"nel":     newLevelInt,
			},
		})
	}
}
