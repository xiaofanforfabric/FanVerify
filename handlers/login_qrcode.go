package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/skip2/go-qrcode"
)

// HandleLoginQRCode 生成「网页扫码登录」用的 OTP 二维码图片
// GET /api/login/qrcode?otp=xxx
// 二维码内容与 OpenAPI 的 OTP 二维码一致（小程序扫码可解析 otp；微信扫码可拉起小程序）
func HandleLoginQRCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		otp := strings.TrimSpace(r.URL.Query().Get("otp"))
		if otp == "" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "缺少 otp 参数"})
			return
		}

		content := fmt.Sprintf("https://api.fanverify.cn/openapi/otp?otp=%s", otp)
		qr, err := qrcode.New(content, qrcode.Medium)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成二维码失败"})
			return
		}
		qr.DisableBorder = false
		png, err := qr.PNG(256)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "生成二维码失败"})
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(png)
	}
}
