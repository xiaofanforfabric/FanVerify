package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// ===== H5 拉起小程序：URL Scheme =====
// 说明：手机浏览器打开官网时，可请求本接口拿到 weixin:// 的 URL Scheme 直接拉起小程序。
// 依赖 .env 的 WX_APPID / WX_SECRET（与小程序登录共用）。
// 注意：URL Scheme 仅对「已发布上线」的小程序生效；开发版/体验版无法通过 scheme 拉起。

var (
	wxTokenMu     sync.Mutex
	wxCachedToken string
	wxTokenExpire time.Time

	wxSchemeMu     sync.Mutex
	wxCachedScheme string
	wxSchemeExpire time.Time
)

// getWXAccessToken 获取并缓存小程序 access_token（使用 stable_token 接口）
func getWXAccessToken() (string, error) {
	wxTokenMu.Lock()
	defer wxTokenMu.Unlock()

	if wxCachedToken != "" && time.Now().Before(wxTokenExpire) {
		return wxCachedToken, nil
	}

	appid := os.Getenv("WX_APPID")
	secret := os.Getenv("WX_SECRET")
	if appid == "" || secret == "" {
		return "", errors.New("微信配置不完整（WX_APPID / WX_SECRET）")
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"grant_type":    "client_credential",
		"appid":         appid,
		"secret":        secret,
		"force_refresh": false,
	})
	resp, err := http.Post("https://api.weixin.qq.com/cgi-bin/stable_token",
		"application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("获取 access_token 失败: %d %s", r.Errcode, r.Errmsg)
	}
	wxCachedToken = r.AccessToken
	exp := r.ExpiresIn
	if exp <= 600 {
		exp = 7200
	}
	wxTokenExpire = time.Now().Add(time.Duration(exp-300) * time.Second) // 提前 5 分钟过期
	return wxCachedToken, nil
}

// HandleWXScheme 生成用于 H5 拉起小程序的 URL Scheme（缓 25 天复用）
// GET /api/wx/scheme?path=pages/index/index&query=
func HandleWXScheme() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		wxSchemeMu.Lock()
		defer wxSchemeMu.Unlock()

		if wxCachedScheme != "" && time.Now().Before(wxSchemeExpire) {
			writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
				"scheme": wxCachedScheme,
			}})
			return
		}

		token, err := getWXAccessToken()
		if err != nil {
			log.Printf("❌ 生成URL Scheme: 获取 access_token 失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "小程序配置未就绪，请稍后再试"})
			return
		}

		path := r.URL.Query().Get("path")
		if path == "" {
			path = "pages/index/index"
		}
		query := r.URL.Query().Get("query")

		reqBody, _ := json.Marshal(map[string]interface{}{
			"jump_wxa": map[string]string{
				"path":  path,
				"query": query,
			},
			"is_expire":       true,
			"expire_type":     1,  // 1=按天
			"expire_interval": 30, // 最长 30 天
		})
		resp, err := http.Post("https://api.weixin.qq.com/wxa/generatescheme?access_token="+token,
			"application/json", bytes.NewReader(reqBody))
		if err != nil {
			log.Printf("❌ 生成URL Scheme 请求失败: %v", err)
			writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "生成失败，请稍后再试"})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var rr struct {
			Errcode  int    `json:"errcode"`
			Errmsg   string `json:"errmsg"`
			Openlink string `json:"openlink"`
		}
		if err := json.Unmarshal(body, &rr); err != nil {
			log.Printf("❌ 生成URL Scheme 解析失败: %v body=%s", err, string(body))
			writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "生成失败，请稍后再试"})
			return
		}
		if rr.Openlink == "" {
			log.Printf("❌ 生成URL Scheme 失败: %d %s", rr.Errcode, rr.Errmsg)
			// 常见：小程序未发布线上版
			writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: "生成失败：" + rr.Errmsg})
			return
		}

		wxCachedScheme = rr.Openlink
		wxSchemeExpire = time.Now().Add(25 * 24 * time.Hour)
		log.Printf("✅ 已生成小程序 URL Scheme（缓存25天）")

		writeJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]interface{}{
			"scheme": rr.Openlink,
		}})
	}
}
