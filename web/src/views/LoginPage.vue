<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()

const uid = ref('')
const verifyCode = ref('')
const step = ref<'input' | 'logging' | 'error'>('input')
const errorMsg = ref('')
const capToken = ref('')
const captchaDone = ref(false)
const checking = ref(true) // 是否在检测登录状态

// ===== H5 拉起微信小程序（手机版） =====
const isMobile = /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent)
const isWeChat = /MicroMessenger/i.test(navigator.userAgent)
const launching = ref(false)
const launchMsg = ref('')

// 手机端一键拉起微信小程序
async function openMiniProgram() {
  launchMsg.value = ''
  // 微信内置浏览器不允许直接 scheme 跳转，给出引导
  if (isWeChat) {
    launchMsg.value = '请点击右上角「···」→「在微信中打开」，或直接在微信搜索「FanVerify」小程序'
    return
  }
  launching.value = true
  try {
    const res = await fetch(API_BASE + '/api/wx/scheme', { credentials: 'include' })
    const data = await res.json().catch(() => ({}))
    const scheme = data && data.data && data.data.scheme
    if (res.ok && scheme) {
      window.location.href = scheme
    } else {
      launchMsg.value = (data && data.error) || '暂时无法拉起小程序，请在微信中搜索「FanVerify」'
    }
  } catch (e) {
    console.error('拉起小程序失败:', e)
    launchMsg.value = '网络错误，请在微信中搜索「FanVerify」'
  } finally {
    launching.value = false
  }
}

// 加载 Cap.js 人机验证组件
function loadCapScript(): Promise<void> {
  return new Promise((resolve) => {
    if (document.querySelector('script[src*="cap.js/widget"]')) {
      resolve()
      return
    }
    const script = document.createElement('script')
    script.src = 'https://cdn.jsdelivr.net/npm/@cap.js/widget'
    script.async = true
    script.onload = () => resolve()
    document.head.appendChild(script)
  })
}

// 提交登录：后端校验通过后 Set-Cookie（HttpOnly JWT），前端不接触 token 本体
async function doLogin(token: string) {
  step.value = 'logging'
  errorMsg.value = ''
  try {
    const res = await fetch(API_BASE + '/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include', // 关键：跨域携带并保存后端下发的 HttpOnly Cookie
      body: JSON.stringify({
        cap_token: token,
        uid: uid.value.trim(),
        verify_code: verifyCode.value.trim(),
      }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.userdata) {
      router.push('/user')
    } else {
      errorMsg.value = data.error || `登录失败 (${res.status})`
      step.value = 'input'
    }
  } catch (e) {
    console.error('登录请求失败:', e)
    errorMsg.value = '网络错误，请稍后重试'
    step.value = 'input'
  }
}

// 手动登录按钮：校验表单与人机验证后提交（支持先完成验证再填表单的顺序）
async function submitLogin() {
  if (!uid.value.trim() || !verifyCode.value.trim()) {
    errorMsg.value = '请先填写 UID 和动态验证码'
    return
  }
  if (!capToken.value) {
    errorMsg.value = '请先完成人机验证'
    return
  }
  await doLogin(capToken.value)
}

// 初始化人机验证：拦截 cap.js 的 redeem 请求获取 token
async function initCaptcha() {
  await loadCapScript()

  let resolved = false
  const onToken = (token: any) => {
    if (resolved) return
    const t = normalizeCapToken(token)
    if (!t) return
    resolved = true
    capToken.value = t
    captchaDone.value = true
    errorMsg.value = ''
  }

  const originalFetch = window.fetch.bind(window)
  window.fetch = async (input: any, init?: any) => {
    const resp = await originalFetch(input, init)
    const url = typeof input === 'string' ? input : input?.url || ''
    if (url.includes('redeem')) {
      const clone = resp.clone()
      try {
        const body = await clone.json()
        if (body.token) onToken(body.token)
        else if (body.data?.token) onToken(body.data.token)
      } catch (_) {}
    }
    return resp
  }

  // 30 秒验证超时兜底
  setTimeout(() => {
    if (resolved) return
    errorMsg.value = '人机验证超时，请刷新重试'
  }, 30000)
}

onMounted(async () => {
  // 先检测是否已登录（Cookie 中有有效 JWT）：200 直接跳 /user，否则显示登录表单
  const authed = await checkAuth()
  if (!authed) {
    initCaptcha()
  }
})

// 检测登录状态：调用 /api/me，200=已登录直接跳转，否则要求登录（401 时后端已清理过期 Cookie）
// ===== 扫码登录 =====
const mode = ref<'code' | 'qr'>('code')
const qrOtp = ref('')
const qrImg = ref('')
const qrStatus = ref<'idle' | 'waiting' | 'ok' | 'error'>('idle')
const qrMsg = ref('')
const generating = ref(false)
let pollTimer: any = null

function switchMode(m: 'code' | 'qr') {
  if (mode.value === m) return
  mode.value = m
  errorMsg.value = ''
  if (m !== 'qr') {
    stopPoll()
    qrStatus.value = 'idle'
    qrMsg.value = ''
  }
}

// 生成登录二维码：每次生成前后台静默走一次人机验证，确保 cap_token 新鲜可用
async function generateQr() {
  qrMsg.value = ''
  stopPoll()

  // 每次生成前都后台静默走一次人机验证，拿“新鲜”的 cap_token
  // （界面显示“已通过”≠ token 仍可用：cap_token 一次性，可能已被消费/过期）
  let token = await silentSolveToken()
  if (!token) token = capToken.value // 静默失败时兜底用界面上已验证的 token
  if (!token) {
    qrStatus.value = 'error'
    qrMsg.value = '人机验证未完成，请点击上方验证后重试'
    return
  }
  capToken.value = normalizeCapToken(token)
  captchaDone.value = true

  generating.value = true
  try {
    const res = await fetch(`${API_BASE}/api/login?cap_token=${encodeURIComponent(capToken.value)}`)
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.otp) {
      qrOtp.value = data.otp
      qrImg.value = `${API_BASE}/api/login/qrcode?otp=${encodeURIComponent(data.otp)}`
      qrStatus.value = 'waiting'
      startPoll()
    } else {
      qrStatus.value = 'error'
      qrMsg.value = data.error || '生成失败，请稍后重试'
    }
  } catch (e) {
    console.error('生成二维码失败:', e)
    qrStatus.value = 'error'
    qrMsg.value = '网络错误，请稍后重试'
  } finally {
    generating.value = false
  }
}

// 规范化 cap token：cap 的 solve()/redeem 可能返回字符串，也可能是对象（如 {token:"..."}）
function normalizeCapToken(v: any): string {
  if (!v) return ''
  if (typeof v === 'string') return v
  if (typeof v === 'object') {
    if (typeof v.token === 'string') return v.token
    if (v.data && typeof v.data.token === 'string') return v.data.token
    if (typeof v.solution === 'string') return v.solution
  }
  return ''
}

// 后台静默走一次人机验证：调用 cap-widget 的 solve()，无需用户操作
async function silentSolveToken(): Promise<string> {
  try {
    const el: any = document.querySelector('cap-widget')
    if (el && typeof el.solve === 'function') {
      if (typeof el.reset === 'function') el.reset()
      const raw = await el.solve()
      const token = normalizeCapToken(raw)
      if (token) {
        console.log('✅ 静默人机验证通过')
        return token
      }
      console.warn('静默人机验证返回无法识别的 token:', raw)
    }
  } catch (e) {
    console.error('静默人机验证失败:', e)
  }
  return ''
}

// 轮询登录状态（只带 otp，无需 cap_token）
function startPoll() {
  stopPoll()
  pollTimer = setInterval(async () => {
    if (!qrOtp.value) return
    try {
      const res = await fetch(`${API_BASE}/api/login?otp=${encodeURIComponent(qrOtp.value)}`)
      const data = await res.json().catch(() => ({}))
      if (data.status === 'ok') {
        stopPoll()
        qrStatus.value = 'ok'
        setTimeout(() => router.push('/user'), 600)
      } else if (data.status === 'lost' || data.status === '404') {
        stopPoll()
        // 过期：后台静默重新人机验证并自动刷新二维码，无需用户操作
        // 但如果已经登录成功（qrStatus === 'ok'），不要重新生成覆盖
        if (qrStatus.value !== 'ok') {
          generateQr()
        }
      }
    } catch (e) {
      // 单次网络抖动忽略，下一轮重试
    }
  }, 5000) // 后端对轮询做 5 秒限频，前端对齐 5 秒间隔
}
function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}
onUnmounted(() => stopPoll())

async function checkAuth(): Promise<boolean> {
  try {
    const res = await fetch(API_BASE + '/api/me', { credentials: 'include' })
    if (res.status === 200) {
      router.push('/user')
      return true
    }
    return false
  } catch (e) {
    console.error('检查登录状态失败:', e)
    return false
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <nav class="login-nav">
      <router-link to="/" class="nav-back">← 返回首页</router-link>
      <span class="nav-title">用户登录</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="login-wrap">
      <!-- 检查登录状态中 -->
      <div v-if="checking" class="login-card status-card">
        <div class="logo-badge">🔐</div>
        <p class="form-tip">正在检查登录状态…</p>
      </div>

      <!-- 登录表单 -->
      <div v-else class="login-card">
        <div class="card-head">
          <div class="logo-badge">🔐</div>
          <h1>FanVerify 登录</h1>
          <p class="card-desc">使用微信小程序中的 UID 与动态验证码完成登录</p>
        </div>

        <div class="mode-tabs">
          <button class="mode-tab" :class="{ active: mode === 'code' }" @click="switchMode('code')">动态码登录</button>
          <button class="mode-tab" :class="{ active: mode === 'qr' }" @click="switchMode('qr')">扫码登录</button>
        </div>

        <!-- 动态码登录 -->
        <template v-if="mode === 'code'">
          <div class="form-field">
            <label for="uid">UID</label>
            <input id="uid" v-model="uid" type="text" inputmode="numeric" placeholder="例如 100000" autocomplete="username" />
          </div>

          <div class="form-field">
            <label for="code">动态验证码</label>
            <input id="code" v-model="verifyCode" type="text" inputmode="numeric" placeholder="小程序首页显示的验证码" autocomplete="one-time-code" />
          </div>
        </template>

        <!-- 公共人机验证 -->
        <div class="captcha-box">
          <cap-widget data-cap-api-endpoint="https://cap.fanverify.cn/22a34a230e/"></cap-widget>
        </div>

        <p v-if="captchaDone" class="captcha-done">✓ 人机验证已通过</p>

        <!-- 动态码登录按钮 -->
        <button v-if="mode === 'code'" class="btn-login" :disabled="step === 'logging'" @click="submitLogin">
          {{ step === 'logging' ? '登录中…' : '登 录' }}
        </button>

        <!-- 扫码登录 -->
        <div v-if="mode === 'qr'" class="qr-panel">
          <p class="qr-hint">用微信扫描二维码（或截图后到小程序「扫码 / 相册」识别）即可登录</p>
          <button v-if="qrStatus === 'idle' || qrStatus === 'error'" class="btn-login" :disabled="generating" @click="generateQr">
            {{ generating ? '生成中…' : '生成登录二维码' }}
          </button>
          <div v-if="qrStatus === 'waiting' || qrStatus === 'ok'" class="qr-box">
            <img :src="qrImg" class="qr-img" alt="登录二维码" />
            <p class="qr-status">{{ qrStatus === 'ok' ? '✅ 登录成功，正在跳转…' : '⏳ 等待扫码确认…' }}</p>
          </div>
          <p v-if="qrMsg" class="form-error">{{ qrMsg }}</p>
        </div>

        <!-- 手机端：一键拉起微信小程序 -->
        <div v-if="isMobile" class="mini-entry">
          <div class="mini-divider"><span>手机用户</span></div>
          <button class="btn-weapp" :disabled="launching" @click="openMiniProgram">
            📱 {{ launching ? '正在打开…' : '打开微信小程序' }}
          </button>
          <p v-if="launchMsg" class="mini-tip">{{ launchMsg }}</p>
        </div>

        <p v-if="errorMsg" class="form-error">{{ errorMsg }}</p>
        <p v-if="step === 'logging'" class="form-tip">正在验证登录…</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.login-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
}

.nav-back {
  color: #4f46e5;
  text-decoration: none;
  font-size: 14px;
  width: 120px;
}

.nav-title {
  font-weight: 600;
  font-size: 16px;
}

.nav-placeholder {
  width: 120px;
}

.login-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.login-card {
  width: 100%;
  max-width: 420px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 36px 32px;
}

.card-head {
  text-align: center;
  margin-bottom: 28px;
}

.logo-badge {
  width: 56px;
  height: 56px;
  margin: 0 auto 12px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  color: #fff;
}

.card-head h1 {
  font-size: 22px;
  margin: 0 0 8px;
}

.card-desc {
  color: #6b7280;
  font-size: 13px;
  margin: 0;
}

.status-card {
  text-align: center;
  padding: 48px 28px;
}

.form-field {
  margin-bottom: 18px;
}

.form-field label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 6px;
}

.form-field input {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 14px;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s;
}

.form-field input:focus {
  border-color: #667eea;
}

.captcha-box {
  margin: 22px 0 8px;
  display: flex;
  justify-content: center;
}

.form-error {
  color: #dc2626;
  font-size: 13px;
  text-align: center;
  margin: 12px 0 0;
}

.captcha-done {
  color: #16a34a;
  font-size: 13px;
  text-align: center;
  margin: 12px 0 0;
}

.btn-login {
  width: 100%;
  margin-top: 18px;
  padding: 13px 0;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s, transform 0.1s;
}

.btn-login:active {
  transform: scale(0.98);
}

.btn-login:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.form-tip {
  color: #6b7280;
  font-size: 13px;
  text-align: center;
  margin: 12px 0 0;
}

/* 手机端：打开微信小程序 */
.mini-entry {
  margin-top: 20px;
}

.mini-divider {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #9ca3af;
  font-size: 12px;
  margin-bottom: 12px;
}

.mini-divider::before,
.mini-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #e5e7eb;
}

.btn-weapp {
  width: 100%;
  padding: 12px 0;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  color: #fff;
  background: #07c160;
  cursor: pointer;
  transition: opacity 0.2s;
}

.btn-weapp:active {
  opacity: 0.85;
}

.btn-weapp:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.mini-tip {
  color: #6b7280;
  font-size: 12px;
  line-height: 1.6;
  text-align: center;
  margin: 10px 0 0;
}

/* 手机端适配 */
@media (max-width: 480px) {
  .login-card {
    padding: 28px 20px;
    border-radius: 14px;
  }
}

/* 扫码登录 */
.mode-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}

.mode-tab {
  flex: 1;
  padding: 9px 0;
  border: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  color: #6b7280;
  cursor: pointer;
}

.mode-tab.active {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border-color: transparent;
}

.qr-panel {
  margin-top: 4px;
}

.qr-hint {
  color: #6b7280;
  font-size: 13px;
  line-height: 1.7;
  text-align: center;
  margin: 0 0 14px;
}

.qr-box {
  text-align: center;
}

.qr-img {
  width: 200px;
  height: 200px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.qr-status {
  color: #4f46e5;
  font-size: 13px;
  margin: 12px 0 0;
}
</style>
