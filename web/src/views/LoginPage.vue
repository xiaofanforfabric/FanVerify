<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()

const uid = ref('')
const verifyCode = ref('')
const step = ref<'input' | 'logging' | 'error'>('input')
const errorMsg = ref('')
const capToken = ref('')
const captchaDone = ref(false)

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
  const onToken = (token: string) => {
    if (resolved) return
    resolved = true
    capToken.value = token
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

onMounted(initCaptcha)
</script>

<template>
  <div class="login-page">
    <nav class="login-nav">
      <router-link to="/" class="nav-back">← 返回首页</router-link>
      <span class="nav-title">用户登录</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="login-wrap">
      <div class="login-card">
        <div class="card-head">
          <div class="logo-badge">🔐</div>
          <h1>FanVerify 登录</h1>
          <p class="card-desc">使用微信小程序中的 UID 与动态验证码完成登录</p>
        </div>

        <div class="form-field">
          <label for="uid">UID</label>
          <input id="uid" v-model="uid" type="text" inputmode="numeric" placeholder="例如 100000" autocomplete="username" />
        </div>

        <div class="form-field">
          <label for="code">动态验证码</label>
          <input id="code" v-model="verifyCode" type="text" inputmode="numeric" placeholder="小程序首页显示的验证码" autocomplete="one-time-code" />
        </div>

        <div class="captcha-box">
          <cap-widget data-cap-api-endpoint="https://cap.fanverify.cn/22a34a230e/"></cap-widget>
        </div>

        <p v-if="captchaDone" class="captcha-done">✓ 人机验证已通过</p>

        <button class="btn-login" :disabled="step === 'logging'" @click="submitLogin">
          {{ step === 'logging' ? '登录中…' : '登 录' }}
        </button>

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
</style>
