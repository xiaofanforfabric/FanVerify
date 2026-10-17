# FanVerify

> 面向开发者与普通用户的微信小程序两步验证（2FA）鉴权后端

FanVerify 提供基于 TOTP 的动态验证码鉴权服务，帮助业务在「账号 + 密码」之外增加一层安全性。用户通过微信小程序获取动态验证码，业务方通过 OpenAPI 校验，无需自建短信/邮件通道。

本仓库为 FanVerify 的后端服务与官网，使用 Go + Vue 3 编写，可独立部署。

## 仓库组成

| 目录 | 说明 | 技术栈 |
| --- | --- | --- |
| `.`（根目录） | 后端服务：鉴权、OpenAPI、登录、后台任务 | Go 1.26 · net/http · SQLite / MySQL |
| `web/` | 官网与用户控制台 | Vue 3 · TypeScript · Vite |

> 微信小程序客户端不在本仓库。后端已提供完整对接接口（`/api/wx/*`）。

## 许可证

本项目基于 **GNU General Public License v3.0（GPL-3.0）** 开源，详见 [`LICENSE`](LICENSE)。

你可以自由使用、修改和分发本软件，但**衍生作品必须以相同许可证开源**，且需保留原始版权声明。

## 核心功能

### 鉴权体系
- **TOTP 动态验证码**：基于 `CODE_SECRET` 与用户 UID 派生，30 秒刷新
- **验证等级**：`0`（root/管理员）、`1`（普通）、`2`（高级），可通过短信验证码升级
- **账号状态**：`ok`（正常）、`bad`（封禁，所有接口返回 401）、`ice`（冻结）

### 登录方式
- **动态码登录**：UID + 当前验证码 + 人机验证
- **网页扫码登录**：生成 OTP 二维码 → 小程序扫码自动确认 → 轮询领取 JWT
- **H5 拉起小程序**：手机端浏览器一键跳转小程序，携带 OTP 免扫码自动登录
- **开发者后台**：密钥管理、业务入驻申请、结算账单查询

### OpenAPI（供业务方接入）

所有 `/openapi/*` 接口均需 `AccessToken` 鉴权，并校验客户端 IP 与业务绑定关系。

| 接口 | 用途 |
| --- | --- |
| `POST /openapi/otp` | 生成一次性验证凭证 |
| `POST /openapi/seeotp` | 查询用户动态码 |
| `POST /openapi/user_verify` | 校验用户身份 |
| `POST /openapi/genqrcode` | 生成鉴权二维码 |
| `POST /openapi/getuserdata` | 获取用户资料 |
| `POST /openapi/devinfo` | 查询业务信息 |
| `POST /openapi/tag` | 用户标签管理 |

### 管理员接口

仅 `level == 0` 且 `status == ok` 的用户可调用：用户列表、封禁/解封、业务冻结、入驻申请审核。

## 安全设计

- **JWT 自包含令牌**：用户信息内嵌于 claims，7 天有效期
- **省份绑定防重放**：令牌绑定签发时 IP 所属省份（ip2region 离线库），跨省使用视为挪用
- **HttpOnly Cookie**：前端不接触令牌本体，`SameSite=Lax` 防 CSRF
- **真实 IP 获取**：优先读取 nginx 覆写的 `X-Real-IP`，防止 `X-Forwarded-For` 伪造
- **人机验证**：Cap.js 保护登录与账单查询，轮询阶段改用 5 秒限频
- **轮询限频**：内存级 5 秒窗口，超限返回 `429`

## 快速开始

### 后端

```bash
cp .env.example .env   # 填写 WX_SECRET / CODE_SECRET / JWT_SECRET / CAP_SECRET
go mod download
go run .
```

服务默认监听 `:8888`。构建发布版：

```bash
./build.sh             # Linux/macOS，产出 fanverify-linux-amd64
./build.ps1            # Windows
```

### 官网 / 控制台

```bash
cd web
npm install
npm run dev            # 开发服务器 http://localhost:5173
npm run build          # 产出 dist/（部署到腾讯云 EO）
```

## 环境变量

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `WX_APPID` | ✅ | 微信小程序 AppID |
| `WX_SECRET` | ✅ | 微信小程序 AppSecret |
| `CODE_SECRET` | ✅ | TOTP 验证码签名密钥 |
| `JWT_SECRET` | ✅ | JWT 签发密钥 |
| `CAP_SECRET` | ✅ | Cap.js 人机验证密钥 |
| `PORT` | ❌ | 监听端口，默认 `8888` |
| `STATUS` | ❌ | `test`=SQLite（默认），`ok`=MySQL（生产） |
| `DEBUG` | ❌ | 是否启用 `/api/debug/*` 调试接口 |
| `USER_PLAN_MODE` | ❌ | 是否启用开发者业务保活检测计划任务 |
| `CORS_ALLOW_ORIGINS` | ❌ | 额外允许的跨域来源，逗号分隔 |

## 目录结构

```
.
├── main.go              # 入口：路由注册、启动定时任务
├── db.go                # 数据库初始化（SQLite / MySQL 双模式）
├── totp.go              # TOTP 算法实现
├── handlers/            # 业务处理
│   ├── login.go         # 登录（动态码 / OTP 扫码）+ JWT 签发
│   ├── login_qrcode.go  # 扫码登录二维码生成
│   ├── wechat.go        # 小程序接口（openid / 取码 / 改等级）
│   ├── openapi.go       # 对外 OpenAPI
│   ├── middleware.go    # AccessToken 鉴权 + 业务绑定
│   ├── admin.go         # 管理员接口
│   ├── scheme.go        # H5 拉起小程序 URL Scheme
│   ├── billing.go       # 账单查询与定时拉取
│   ├── planchecker.go   # 业务保活检测定时任务
│   ├── email.go         # 腾讯云 SES 邮件通知
│   ├── ratelimit.go     # 限流
│   └── legal/           # 用户协议与隐私政策
├── web/                 # 官网与控制台（Vue 3 + Vite）
│   └── src/views/       # 登录 / 用户 / 开发者 / 管理员页面
├── .env.example         # 环境变量模板
└── LICENSE              # GPL-3.0
```

## 依赖服务

- **微信小程序平台**：openid 换取、URL Scheme 生成
- **Cap.js**：人机验证（`cap.fanverify.cn`）
- **腾讯云 SES / SMS**：邮件与短信验证码
- **ip2region**：离线 IP 归属地库（已内嵌二进制）

## 部署提示

- 生产环境需反向代理（nginx）并设置：`proxy_set_header X-Real-IP $remote_addr`
- 使用 MySQL 时设置 `STATUS=ok`，并在 `db.go` 的配置中填写连接信息
- `DEBUG=false` 时所有 `/api/debug/*` 路由返回 404

## 贡献

欢迎提交 Issue 与 Pull Request。提交代码即表示你同意以 GPL-3.0 授权你的贡献。

请在新增的 Go 源文件顶部保留版权声明：

```go
// FanVerify - 微信小程序两步验证（2FA）鉴权平台后端
// Copyright (C) 2026  FanVerify
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
```

## 版权

Copyright (C) 2026 FanVerify

本程序为自由软件，在 GPL-3.0 条款下可自由分发与修改；本程序**不提供任何担保**，包括但不限于适销性与特定用途适用性的默示担保。
