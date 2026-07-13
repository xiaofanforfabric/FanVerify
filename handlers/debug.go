package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// AddDevRequest 添加开发者请求
type AddDevRequest struct {
	DevAccessToken string `json:"devaccesstoken"`
	NEL            int    `json:"nel"`
	BUID           int    `json:"buid"`
	BIP            string `json:"bip"`
	Mode           string `json:"mode"`
	SME            string `json:"sme"`
}

// Handle404Debug 处理 debug 404
func Handle404Debug() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not Found"}`))
	}
}

// HandleDebugAddDev 添加开发者
func HandleDebugAddDev() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"success":false,"error":"仅支持 POST 请求"}`))
			return
		}

		var req AddDevRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"请求解析失败"}`))
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"JSON 格式错误"}`))
			return
		}

		if req.DevAccessToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"开发者令牌不能为空"}`))
			return
		}
		if req.NEL < 1 {
			req.NEL = 1
		}
		if req.Mode == "" {
			req.Mode = "HTTP"
		}

		_, err = db.Exec(
			"INSERT INTO devdata (devaccesstoken, NEL, BUID, BIP, mode, SME, status) VALUES (?, ?, ?, ?, ?, ?, 'wait')",
			req.DevAccessToken, req.NEL, req.BUID, req.BIP, req.Mode, req.SME,
		)
		if err != nil {
			log.Printf("❌ 添加开发者失败: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"添加失败，令牌可能已存在"}`))
			return
		}

		log.Printf("✅ 添加开发者: %s, BUID=%d, Mode=%s", req.DevAccessToken, req.BUID, req.Mode)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"开发者添加成功"}`))
	}
}

// HandleDebugWebUI 调试 Web 界面
func HandleDebugWebUI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FanVerify - 添加开发者</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px; }
        .container { background: white; border-radius: 16px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); max-width: 500px; width: 100%; padding: 40px; }
        h1 { color: #333; font-size: 28px; margin-bottom: 10px; text-align: center; }
        .subtitle { color: #666; font-size: 14px; text-align: center; margin-bottom: 30px; }
        .form-group { margin-bottom: 20px; }
        label { display: block; color: #555; font-size: 14px; font-weight: 600; margin-bottom: 8px; }
        input, select, textarea { width: 100%; padding: 12px 16px; border: 2px solid #e0e0e0; border-radius: 8px; font-size: 14px; transition: all 0.3s; }
        input:focus, select:focus, textarea:focus { outline: none; border-color: #667eea; box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1); }
        textarea { resize: vertical; min-height: 80px; font-family: inherit; }
        .btn { width: 100%; padding: 14px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: transform 0.2s, box-shadow 0.2s; }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 25px rgba(102, 126, 234, 0.4); }
        .btn:active { transform: translateY(0); }
        .btn:disabled { opacity: 0.6; cursor: not-allowed; transform: none; }
        .generate-token { display: inline-block; color: #667eea; font-size: 13px; margin-top: 5px; cursor: pointer; text-decoration: none; }
        .generate-token:hover { text-decoration: underline; }
        .message { padding: 12px 16px; border-radius: 8px; margin-top: 20px; font-size: 14px; display: none; }
        .message.success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .message.error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .row { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔑 添加开发者</h1>
        <p class="subtitle">FanVerify Debug Panel</p>
        
        <form id="devForm">
            <div class="form-group">
                <label for="token">开发者令牌 *</label>
                <input type="text" id="token" name="devaccesstoken" required placeholder="dev_abc123xyz456">
                <a class="generate-token" onclick="generateToken()">随机生成</a>
            </div>
            
            <div class="row">
                <div class="form-group">
                    <label for="nel">要求最低等级 *</label>
                    <select id="nel" name="nel" required>
                        <option value="1">等级 1</option>
                        <option value="2">等级 2</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label for="buid">绑定 UID</label>
                    <input type="number" id="buid" name="buid" value="0" min="0" placeholder="0">
                </div>
            </div>
            
            <div class="row">
                <div class="form-group">
                    <label for="bip">服务器 IP *</label>
                    <select id="bip" name="bip" required>
                        <option value="ALL">允许所有 IP</option>
                        <option value="NULL">不限制</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label for="mode">模式 *</label>
                    <select id="mode" name="mode" required>
                        <option value="HTTP">HTTP</option>
                        <option value="FWS">正向 WebSocket</option>
                        <option value="RWS">反向 WebSocket</option>
                    </select>
                </div>
            </div>
            
            <div class="form-group">
                <label for="sme">业务描述</label>
                <textarea id="sme" name="sme" placeholder="请描述该令牌的使用场景..."></textarea>
            </div>
            
            <button type="submit" class="btn" id="submitBtn">✅ 确认添加</button>
        </form>
        
        <div id="message" class="message"></div>
    </div>
    
    <script>
        function generateToken() {
            const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
            let token = 'dev_';
            for (let i = 0; i < 32; i++) {
                token += chars.charAt(Math.floor(Math.random() * chars.length));
            }
            document.getElementById('token').value = token;
        }
        
        document.getElementById('devForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const submitBtn = document.getElementById('submitBtn');
            const message = document.getElementById('message');
            
            submitBtn.disabled = true;
            submitBtn.textContent = '⏳ 添加中...';
            message.style.display = 'none';
            
            const formData = {
                devaccesstoken: document.getElementById('token').value,
                nel: parseInt(document.getElementById('nel').value),
                buid: parseInt(document.getElementById('buid').value),
                bip: document.getElementById('bip').value,
                mode: document.getElementById('mode').value,
                sme: document.getElementById('sme').value
            };
            
            try {
                const response = await fetch('/api/debug/adddev', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(formData)
                });
                
                const data = await response.json();
                
                if (data.success) {
                    message.className = 'message success';
                    message.textContent = '✅ ' + data.message;
                    message.style.display = 'block';
                    document.getElementById('devForm').reset();
                } else {
                    message.className = 'message error';
                    message.textContent = '❌ ' + data.error;
                    message.style.display = 'block';
                }
            } catch (error) {
                message.className = 'message error';
                message.textContent = '❌ 网络错误: ' + error.message;
                message.style.display = 'block';
            } finally {
                submitBtn.disabled = false;
                submitBtn.textContent = '✅ 确认添加';
            }
        });
    </script>
</body>
</html>`
		w.Write([]byte(html))
	}
}
