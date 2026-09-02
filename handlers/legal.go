package handlers

import (
	_ "embed"
	"net/http"
)

// 用户协议与隐私政策内容（编译时嵌入二进制，更新内容需重新编译）

//go:embed legal/user-agreement.md
var userAgreementMD string

//go:embed legal/privacy-policy.md
var privacyPolicyMD string

// HandleLegalAgreement 返回用户协议（纯文本 Markdown）
func HandleLegalAgreement() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(userAgreementMD))
	}
}

// HandleLegalPrivacy 返回隐私政策（纯文本 Markdown）
func HandleLegalPrivacy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(privacyPolicyMD))
	}
}
