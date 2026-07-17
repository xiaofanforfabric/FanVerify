package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"

	"fanauth/handlers"
)

const (
	defaultPort  = "8888"
	defaultAppID = "wxea50dc0c6b97b784"
)

func main() {
	loadEnv()

	appID := getEnv("WX_APPID", defaultAppID)
	secret := getEnv("WX_SECRET", "")
	if secret == "" {
		log.Fatal(".env 文件中未设置 WX_SECRET（小程序密钥）")
	}
	port := getEnv("PORT", defaultPort)
	codeSecret := getEnv("CODE_SECRET", "")
	if codeSecret == "" {
		log.Fatal(".env 文件中未设置 CODE_SECRET（验证码共享密钥）")
	}

	// 初始化数据库
	initDB()
	handlers.SetDB(GetDB())

	// 读取 debug 模式
	debugMode := getEnv("DEBUG", "false") == "true"

	mux := http.NewServeMux()

	// 微信小程序接口
	mux.HandleFunc("/api/wx/code/openid", handlers.CorsMiddleware(handlers.HandleCodeOpenID(appID, secret)))
	mux.HandleFunc("/api/wx/code/generate", handlers.CorsMiddleware(handlers.HandleCodeGenerate(appID, secret, codeSecret)))
	mux.HandleFunc("/api/wx/update/code_level", handlers.CorsMiddleware(handlers.HandleUpdateCodeLevel(appID, secret, codeSecret)))
	mux.HandleFunc("/api/wx/otpAPI", handlers.CorsMiddleware(handlers.HandleOTPAPI(appID, secret)))

	// OpenAPI 接口（需要 accesstoken 鉴权）
	mux.HandleFunc("/openapi/otp", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleGenerateOTP())))
	mux.HandleFunc("/openapi/genqrcode", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleGenerateQRCode())))
	mux.HandleFunc("/openapi/seeotp", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleSeeOTP())))
	mux.HandleFunc("/openapi/user_verify", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleUserVerify())))
	mux.HandleFunc("/openapi/getuserdata", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleGetUserData())))
	mux.HandleFunc("/openapi/devinfo", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleDevInfo())))
	mux.HandleFunc("/openapi/tag", handlers.CorsMiddleware(handlers.AccessTokenMiddleware(handlers.HandleTag())))

	// Debug 路由
	if debugMode {
		log.Println("⚠️ DEBUG 模式已启用")
		mux.HandleFunc("/api/debug/adddev", handlers.CorsMiddleware(handlers.HandleDebugAddDev()))
		mux.HandleFunc("/api/debug/adddev/web/", handlers.HandleDebugWebUI())
	} else {
		// 非 DEBUG 模式，拦截所有 debug 路由
		mux.HandleFunc("/api/debug/adddev", handlers.Handle404Debug())
		mux.HandleFunc("/api/debug/adddev/web/", handlers.Handle404Debug())
		mux.HandleFunc("/api/debug/", handlers.Handle404Debug())
	}

	addr := ":" + port
	log.Printf("FanVerify 后端服务启动，监听端口 %s", port)
	log.Printf("API 地址: http://localhost%s/api/wx/code/openid", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Println("⚠️ .env 文件不存在，使用默认配置")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
