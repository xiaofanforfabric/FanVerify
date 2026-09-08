<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'
const router = useRouter()

const loading = ref(true)
const unauth = ref('')

const step = ref<'form' | 'sent' | 'done' | 'error'>('form')
const email = ref('')
const sessionToken = ref('')
const passCode = ref('')
const msg = ref('') // 错误/提示
const okMsg = ref('')
const boundMask = ref('') // 已绑定邮箱脱敏
const sending = ref(false)
const verifying = ref(false)
const countdown = ref(0)
let timer: any = null

const emailRe = /^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$/

// 检测登录
async function fetchMe() {
  loading.value = true
  try {
    const res = await fetch(API_BASE + '/api/me', { credentials: 'include' })
    if (res.status === 401) {
      unauth.value = '未登录或登录已过期'
      return
    }
    if (!res.ok) throw new Error('请求失败')
    const data = await res.json()
    if (data.email) {
      boundMask.value = data.email // 已绑定，可覆盖更换
    }
  } catch (e) {
    console.error('检查登录失败:', e)
    unauth.value = '网络错误，请刷新重试'
  } finally {
    loading.value = false
  }
}

function goLogin() {
  router.push('/login')
}
function goBack() {
  router.push('/user')
}

// 发送验证码
async function sendCode() {
  msg.value = ''
  okMsg.value = ''
  const em = email.value.trim()
  if (!em) {
    msg.value = '请输入邮箱地址'
    return
  }
  if (em.length > 30 || !emailRe.test(em)) {
    msg.value = '邮箱格式不正确，且长度不超过 30 位'
    return
  }
  sending.value = true
  try {
    const res = await fetch(API_BASE + '/api/user/update_address', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ token: '', email: em }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.success && data.data?.session_token) {
      sessionToken.value = data.data.session_token
      step.value = 'sent'
      okMsg.value = '验证码已发送至邮箱，请查收（5 分钟内有效）'
      startCountdown(60)
    } else {
      step.value = 'form'
      msg.value = data.error || `发送失败 (${res.status})`
      if (res.status === 401) setTimeout(() => router.push('/login'), 1200)
    }
  } catch (e) {
    console.error('发送失败:', e)
    msg.value = '网络错误，请稍后重试'
  } finally {
    sending.value = false
  }
}

function startCountdown(sec: number) {
  countdown.value = sec
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
      timer = null
    }
  }, 1000)
}

// 验证并绑定
async function verifyBind() {
  msg.value = ''
  okMsg.value = ''
  const code = passCode.value.trim()
  if (!code) {
    msg.value = '请输入收到的验证码'
    return
  }
  verifying.value = true
  try {
    const url = `${API_BASE}/api/user/update_address?session_token=${encodeURIComponent(sessionToken.value)}&pass_code=${encodeURIComponent(code)}`
    const res = await fetch(url, { credentials: 'include' })
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.success) {
      step.value = 'done'
      okMsg.value = data.data?.mask || '绑定成功'
    } else {
      msg.value = data.error || `绑定失败 (${res.status})`
      if (res.status === 401) setTimeout(() => router.push('/login'), 1200)
    }
  } catch (e) {
    console.error('绑定失败:', e)
    msg.value = '网络错误，请稍后重试'
  } finally {
    verifying.value = false
  }
}

// 更换邮箱：重新编辑
function changeEmail() {
  step.value = 'form'
  sessionToken.value = ''
  passCode.value = ''
  msg.value = ''
  okMsg.value = ''
}

onMounted(fetchMe)
</script>

<template>
  <div class="bind-page">
    <nav class="bind-nav">
      <router-link to="/user" class="nav-back">← 返回用户中心</router-link>
      <span class="nav-title">添加联系方式</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="bind-wrap">
      <!-- 加载中 -->
      <div v-if="loading" class="bind-card status-card">
        <div class="status-icon">⏳</div>
        <h2>正在检查登录状态…</h2>
      </div>

      <!-- 未登录 -->
      <div v-else-if="unauth" class="bind-card status-card">
        <div class="status-icon">🔒</div>
        <h2>无法访问</h2>
        <p class="status-desc">{{ unauth }}</p>
        <button class="btn-primary" @click="goLogin">去登录</button>
      </div>

      <!-- 绑定成功 -->
      <div v-else-if="step === 'done'" class="bind-card status-card">
        <div class="status-icon">✅</div>
        <h2>邮箱绑定成功</h2>
        <p class="status-desc">已绑定：{{ okMsg }}<br />后续重要通知将通过该邮箱发送。</p>
        <button class="btn-primary" @click="goBack">返回用户中心</button>
      </div>

      <!-- 发送/验证表单 -->
      <div v-else class="bind-card">
        <div class="card-head">
          <div class="logo-badge">📮</div>
          <h1>绑定联系邮箱</h1>
          <p class="card-desc">用于接收 FanVerify 的重要通知（审核结果、服务变更等）</p>
        </div>

        <div v-if="boundMask" class="tip-banner">
          当前已绑定：{{ boundMask }}，如需更换请直接填写新邮箱。
        </div>

        <div class="form-field">
          <label for="email">电子邮箱 *</label>
          <input id="email" v-model.trim="email" type="email" placeholder="例如 name@example.com" :disabled="step === 'sent'" />
        </div>
        <p class="field-tip">⚠️ 请务必准确填写，否则后续可能无法接收到重要通知。</p>

        <button class="btn-primary btn-block" :disabled="sending || step === 'sent'" @click="sendCode">
          {{ sending ? '发送中…' : countdown > 0 ? `重新发送 (${countdown}s)` : step === 'sent' ? '验证码已发送' : boundMask ? '发送验证码并更换' : '发送验证码' }}
        </button>

        <div v-if="step === 'sent'" class="verify-box">
          <div class="form-field">
            <label for="code">邮箱验证码</label>
            <input id="code" v-model.trim="passCode" type="text" inputmode="numeric" maxlength="6" placeholder="请输入 6 位验证码" />
          </div>
          <button class="btn-primary btn-block" :disabled="verifying" @click="verifyBind">
            {{ verifying ? '验证中…' : '验证并绑定' }}
          </button>
          <button class="link-btn" @click="changeEmail">← 重新输入邮箱</button>
        </div>

        <p v-if="msg" class="form-error">{{ msg }}</p>
        <p v-if="okMsg" class="form-ok">{{ okMsg }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bind-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.bind-nav {
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

.bind-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.bind-card {
  width: 100%;
  max-width: 420px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 32px;
}

.status-card {
  text-align: center;
  padding: 48px 28px;
}

.status-icon {
  font-size: 46px;
  margin-bottom: 12px;
}

.status-card h2 {
  font-size: 20px;
  margin: 0 0 10px;
}

.status-desc {
  color: #6b7280;
  font-size: 14px;
  line-height: 1.7;
  margin: 0 0 22px;
}

.card-head {
  text-align: center;
  margin-bottom: 22px;
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
  font-size: 21px;
  margin: 0 0 8px;
}

.card-desc {
  color: #6b7280;
  font-size: 13px;
  margin: 0;
}

.tip-banner {
  background: #eef2ff;
  color: #4338ca;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.7;
  padding: 10px 14px;
  margin-bottom: 16px;
}

.form-field {
  margin-bottom: 6px;
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

.field-tip {
  color: #b45309;
  font-size: 12px;
  line-height: 1.6;
  margin: 6px 0 14px;
}

.btn-primary {
  padding: 12px 26px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s;
}

.btn-primary:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.btn-block {
  width: 100%;
  display: block;
}

.verify-box {
  margin-top: 18px;
  border-top: 1px dashed #e5e7eb;
  padding-top: 16px;
}

.link-btn {
  display: block;
  margin: 12px auto 0;
  background: none;
  border: none;
  color: #6b7280;
  font-size: 13px;
  cursor: pointer;
}

.link-btn:hover {
  color: #4f46e5;
}

.form-error {
  color: #dc2626;
  font-size: 13px;
  text-align: center;
  margin: 14px 0 0;
}

.form-ok {
  color: #16a34a;
  font-size: 13px;
  text-align: center;
  margin: 14px 0 0;
}
</style>
