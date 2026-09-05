<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'
const router = useRouter()

// —— 登录资格 ——
const loading = ref(true)
const unauth = ref('') // 401 / 网络错误
const denied = ref(false) // 等级不足

// —— 开发者协议弹窗 ——
const showLegal = ref(false)
const legalTab = ref<'dev' | 'devp'>('dev')
const legalDev = ref('')
const legalDevp = ref('')
const legalLoading = ref(true)
const agreedChecked = ref(false)
const agreed = ref(false)

// —— 申请表单 ——
const form = ref({
  type: 'web', // web / javamc / cmc / none
  otherType: '', // type=none 时的具体类型（人工审核）
  serverip: '',
  serviceAddr: '',
  connectType: 'http', // http（wsc/wss 暂未开放）
  tag: '',
})

// —— 人机验证 ——
const capToken = ref('')
const captchaDone = ref(false)
const capError = ref('')

// —— 提交 ——
const confirmOpen = ref(false)
const submitting = ref(false)
const resultOk = ref('')
const resultError = ref('')

// ============ 登录检测 ============
async function fetchMe() {
  loading.value = true
  try {
    const res = await fetch(API_BASE + '/api/me', { credentials: 'include' })
    if (res.status === 401) {
      unauth.value = '未登录或登录已过期'
      return
    }
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    const data = await res.json()
    const u = data.userdata && data.userdata[0]
    if (!u) throw new Error('数据格式错误')
    if (Number(u.level) !== 2) {
      denied.value = true
      return
    }
    // 验证等级满足 → 加载协议并弹窗
    showLegal.value = true
    await loadLegal()
  } catch (e: any) {
    console.error('检查登录状态失败:', e)
    unauth.value = e.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function loadLegal() {
  legalLoading.value = true
  try {
    const [a, p] = await Promise.all([
      fetch(API_BASE + '/api/legal/developer', { credentials: 'include' }),
      fetch(API_BASE + '/api/legal/developer-privacy', { credentials: 'include' }),
    ])
    legalDev.value = a.ok ? await a.text() : '（开发者服务协议加载失败，请刷新重试）'
    legalDevp.value = p.ok ? await p.text() : '（开发者隐私政策加载失败，请刷新重试）'
  } catch (e) {
    console.error('加载协议失败:', e)
    legalDev.value = '（网络错误，请刷新重试）'
    legalDevp.value = '（网络错误，请刷新重试）'
  } finally {
    legalLoading.value = false
  }
}

function confirmAgree() {
  if (!agreedChecked.value) return
  agreed.value = true
  showLegal.value = false
}

// ============ cap 人机验证 ============
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

async function initCaptcha() {
  await loadCapScript()

  let resolved = false
  const onToken = (token: string) => {
    if (resolved) return
    resolved = true
    capToken.value = token
    captchaDone.value = true
    capError.value = ''
    resultError.value = ''
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
    capError.value = '人机验证超时，请刷新重试'
  }, 30000)
}

// ============ 提交 ============
function goLogin() {
  router.push('/login')
}
function goDeveloper() {
  router.push('/developer')
}

// 点击「提交申请」：先本地校验，再弹确认窗口
function submit() {
  resultError.value = ''
  resultOk.value = ''
  const f = form.value

  if (f.type === 'none' && !f.otherType.trim()) {
    resultError.value = '请填写具体业务类型（将进行人工审核）'
    return
  }
  if (!f.serverip.trim()) {
    resultError.value = '请填写服务器请求出口 IP'
    return
  }
  if (!/^(\d{1,3}\.){3}\d{1,3}$/.test(f.serverip.trim())) {
    resultError.value = '服务器请求出口 IP 格式不正确（仅 IPv4）'
    return
  }
  if (!f.serviceAddr.trim()) {
    resultError.value = '请填写业务地址（IP:端口 或 域名）'
    return
  }
  if (!f.tag.trim()) {
    resultError.value = '请填写业务备注'
    return
  }
  if (!capToken.value) {
    resultError.value = '请先完成人机验证'
    return
  }
  confirmOpen.value = true
}

async function confirmSubmit() {
  confirmOpen.value = false
  submitting.value = true
  resultError.value = ''
  resultOk.value = ''

  const f = form.value
  try {
    const res = await fetch(API_BASE + '/api/need_dev_key', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include', // HttpOnly Cookie 中的 JWT 由后端自动读取（token 传空）
      body: JSON.stringify({
        token: '',
        cap_token: capToken.value,
        data: {
          type: f.type,
          other_type: f.otherType.trim(),
          serverip: f.serverip.trim(),
          service_adderss: f.serviceAddr.trim(),
          connect_type: f.connectType,
          tag: f.tag.trim(),
        },
      }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.success) {
      resultOk.value = data.data?.message || '申请成功，您的业务已进入审核状态，请耐心等待通知'
    } else {
      resultError.value = data.error || `申请失败 (${res.status})`
      if (res.status === 401) {
        setTimeout(() => router.push('/login'), 1500)
      }
    }
  } catch (e) {
    console.error('提交申请失败:', e)
    resultError.value = '网络错误，请稍后重试'
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  await fetchMe()
  if (!loading.value && !unauth.value && !denied.value) {
    initCaptcha()
  }
})
</script>

<template>
  <div class="need-page">
    <nav class="need-nav">
      <router-link to="/developer" class="nav-back">← 返回开发者页面</router-link>
      <span class="nav-title">开发者入驻申请</span>
      <span class="nav-placeholder"></span>
    </nav>

    <!-- 加载中 -->
    <div v-if="loading" class="need-wrap">
      <div class="need-card status-card">
        <div class="status-icon">⏳</div>
        <h2>正在检查登录状态…</h2>
      </div>
    </div>

    <!-- 未登录 -->
    <div v-else-if="unauth" class="need-wrap">
      <div class="need-card status-card">
        <div class="status-icon">🔒</div>
        <h2>无法访问</h2>
        <p class="status-desc">{{ unauth }}</p>
        <button class="btn-primary" @click="goLogin">去登录</button>
      </div>
    </div>

    <!-- 等级不足 -->
    <div v-else-if="denied" class="need-wrap">
      <div class="need-card status-card">
        <div class="status-icon">📵</div>
        <h2>帐号等级不足</h2>
        <p class="status-desc">申请开发者权限需要 Lv.2 验证等级（手机号二级认证），请先在微信小程序内完成升级。</p>
        <button class="btn-primary" @click="goDeveloper">返回开发者页面</button>
      </div>
    </div>

    <!-- 已通过等级校验 -->
    <div v-else class="need-wrap">
      <!-- 协议弹窗 -->
      <div v-if="showLegal" class="modal-mask">
        <div class="modal-box legal-modal">
          <div class="modal-head">
            <div class="modal-icon">🤝</div>
            <h2>开发者入驻须知</h2>
            <p class="modal-sub">请先阅读并同意以下协议，才能继续填写申请</p>
          </div>

          <div class="legal-tabs">
            <button class="legal-tab" :class="{ active: legalTab === 'dev' }" @click="legalTab = 'dev'">开发者服务协议</button>
            <button class="legal-tab" :class="{ active: legalTab === 'devp' }" @click="legalTab = 'devp'">开发者隐私政策</button>
          </div>

          <div class="legal-body">
            <div v-if="legalLoading" class="legal-loading">正在加载…</div>
            <pre v-else>{{ legalTab === 'dev' ? legalDev : legalDevp }}</pre>
          </div>

          <label class="agree-line">
            <input v-model="agreedChecked" type="checkbox" />
            <span>我已阅读并同意《开发者服务协议》与《开发者隐私政策》</span>
          </label>

          <button class="btn-primary btn-block" :disabled="!agreedChecked || legalLoading" @click="confirmAgree">同意并继续</button>
        </div>
      </div>

      <!-- 申请成功 -->
      <div v-if="resultOk" class="need-card status-card">
        <div class="status-icon">✅</div>
        <h2>申请已提交</h2>
        <p class="status-desc">{{ resultOk }}</p>
        <button class="btn-primary" @click="goDeveloper">返回开发者页面</button>
      </div>

      <!-- 申请表单 -->
      <div v-else class="need-card">
        <div class="card-head">
          <div class="logo-badge">🛠️</div>
          <h1>开发者业务申请</h1>
          <p class="card-desc">填写你的业务信息，审核通过后即可使用 OpenAPI 密钥</p>
        </div>

        <!-- 服务类型 -->
        <div class="form-field">
          <label>服务类型 *</label>
          <div class="opt-group">
            <label class="opt" :class="{ sel: form.type === 'web' }">
              <input v-model="form.type" type="radio" value="web" />
              <span class="opt-name">网站</span>
              <span class="opt-desc">个人网站 / 论坛 / Web 应用</span>
            </label>
            <label class="opt" :class="{ sel: form.type === 'javamc' }">
              <input v-model="form.type" type="radio" value="javamc" />
              <span class="opt-name">Java MC 服务器</span>
              <span class="opt-desc">Java 版 Minecraft 服务器登录验证</span>
            </label>
            <label class="opt" :class="{ sel: form.type === 'cmc' }">
              <input v-model="form.type" type="radio" value="cmc" />
              <span class="opt-name">基岩 MC 服务器</span>
              <span class="opt-desc">Bedrock / 基岩版 Minecraft 服务器登录验证</span>
            </label>
            <label class="opt" :class="{ sel: form.type === 'none' }">
              <input v-model="form.type" type="radio" value="none" />
              <span class="opt-name">其他</span>
              <span class="opt-desc">其他服务类型，需人工审核</span>
            </label>
          </div>
        </div>

        <div v-if="form.type === 'none'" class="form-field">
          <label for="otherType">具体业务类型 *（人工审核）</label>
          <input id="otherType" v-model.trim="form.otherType" type="text" placeholder="例如：云服务器管理面板" />
        </div>

        <!-- 服务器请求出口 IP -->
        <div class="form-field">
          <label for="serverip">服务器请求出口 IP *</label>
          <input id="serverip" v-model.trim="form.serverip" type="text" inputmode="decimal" placeholder="仅 IPv4，例如 1.2.3.4" />
        </div>

        <!-- 业务地址 -->
        <div class="form-field">
          <label for="serviceAddr">业务地址 *</label>
          <input id="serviceAddr" v-model.trim="form.serviceAddr" type="text" placeholder="IP:端口 或 域名，例如 1.2.3.4:8080 或 api.example.com" />
        </div>

        <!-- 连接类型 -->
        <div class="form-field">
          <label>连接类型 *</label>
          <div class="opt-group">
            <label class="opt" :class="{ sel: form.connectType === 'http' }">
              <input v-model="form.connectType" type="radio" value="http" />
              <span class="opt-name">HTTP</span>
              <span class="opt-desc">服务器主动请求接口（推荐）</span>
            </label>
            <label class="opt disabled">
              <input type="radio" disabled />
              <span class="opt-name">正向 WS</span>
              <span class="opt-tag">开发中</span>
            </label>
            <label class="opt disabled">
              <input type="radio" disabled />
              <span class="opt-name">反向 WS</span>
              <span class="opt-tag">开发中</span>
            </label>
          </div>
        </div>

        <!-- 业务备注 -->
        <div class="form-field">
          <label for="tag">业务备注 *</label>
          <textarea id="tag" v-model.trim="form.tag" rows="3" placeholder="请简要描述业务用途、预计调用场景等，便于审核"></textarea>
        </div>

        <!-- 人机验证 -->
        <div class="captcha-box">
          <cap-widget data-cap-api-endpoint="https://cap.fanverify.cn/22a34a230e/"></cap-widget>
        </div>
        <p v-if="captchaDone" class="captcha-done">✓ 人机验证已通过</p>
        <p v-if="capError && !captchaDone" class="form-error">{{ capError }}</p>

        <!-- 提交 -->
        <button class="btn-primary btn-block" :disabled="submitting" @click="submit">
          {{ submitting ? '提交中…' : '提交申请' }}
        </button>

        <p v-if="resultError" class="form-error">{{ resultError }}</p>
      </div>
    </div>

    <!-- 提交确认弹窗 -->
    <div v-if="confirmOpen" class="modal-mask">
      <div class="modal-box confirm-modal">
        <div class="modal-head">
          <div class="modal-icon">⚠️</div>
          <h2>确认提交申请</h2>
        </div>
        <p class="confirm-text">
          请确保信息填写无误，每个帐号仅可创建 10 个业务密钥，且审核等待期间和删除后 3 天内无法创建或修改相同业务。请确保您的业务不是国家明令禁止的业务（如赌博、色情网站），否则我们有权拒绝并封禁您的 FanVerify 帐号。申请提交后会有 1 天保活检测（具体见用户协议）可能会人工复审，该密钥在检查通过后可正常使用。
        </p>
        <div class="modal-actions">
          <button class="btn-ghost" @click="confirmOpen = false">再想想</button>
          <button class="btn-primary" @click="confirmSubmit">确认提交</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.need-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.need-nav {
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

.need-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.need-card {
  width: 100%;
  max-width: 480px;
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
  margin-bottom: 24px;
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

/* 表单 */
.form-field {
  margin-bottom: 18px;
}

.form-field label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 8px;
}

.form-field input[type='text'] {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 14px;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s;
}

.form-field input:focus,
.form-field textarea:focus {
  border-color: #667eea;
}

.form-field textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 14px;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  font-size: 14px;
  font-family: inherit;
  outline: none;
  resize: vertical;
  transition: border-color 0.2s;
}

/* 单选卡片 */
.opt-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.opt {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.opt input {
  accent-color: #667eea;
  flex-shrink: 0;
}

.opt .opt-name {
  font-size: 14px;
  font-weight: 600;
}

.opt .opt-desc {
  font-size: 12px;
  color: #9ca3af;
  margin-left: auto;
  text-align: right;
}

.opt.sel {
  border-color: #667eea;
  background: #f5f6ff;
}

.opt.disabled {
  cursor: not-allowed;
  opacity: 0.5;
  background: #fafafa;
}

.opt .opt-tag {
  font-size: 11px;
  color: #6b7280;
  background: #f3f4f6;
  border-radius: 999px;
  padding: 2px 8px;
  margin-left: auto;
}

/* 按钮 */
.btn-primary {
  padding: 12px 28px;
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
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-block {
  width: 100%;
  display: block;
}

.btn-ghost {
  padding: 12px 28px;
  background: #ffffff;
  color: #374151;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
}

/* cap / 提示 */
.captcha-box {
  margin: 22px 0 8px;
  display: flex;
  justify-content: center;
}

.captcha-done {
  color: #16a34a;
  font-size: 13px;
  text-align: center;
  margin: 12px 0 0;
}

.form-error {
  color: #dc2626;
  font-size: 13px;
  text-align: center;
  margin: 14px 0 0;
}

/* 弹窗 */
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(17, 24, 39, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}

.modal-box {
  width: 100%;
  max-width: 560px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  padding: 28px;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
}

.modal-head {
  text-align: center;
  margin-bottom: 16px;
}

.modal-icon {
  font-size: 40px;
  margin-bottom: 8px;
}

.modal-head h2 {
  font-size: 20px;
  margin: 0 0 6px;
}

.modal-sub {
  color: #6b7280;
  font-size: 13px;
  margin: 0;
}

/* 协议弹窗 */
.legal-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.legal-tab {
  flex: 1;
  padding: 9px 0;
  background: #f3f4f6;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #6b7280;
  cursor: pointer;
}

.legal-tab.active {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
}

.legal-body {
  flex: 1;
  min-height: 260px;
  max-height: 46vh;
  overflow: auto;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 14px 16px;
  margin-bottom: 14px;
}

.legal-body pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-size: 13px;
  line-height: 1.7;
  color: #374151;
  margin: 0;
}

.legal-loading {
  color: #9ca3af;
  text-align: center;
  padding: 40px 0;
}

.agree-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  color: #374151;
  margin: 0 0 14px;
  cursor: pointer;
}

.agree-line input {
  accent-color: #667eea;
  margin-top: 2px;
}

/* 确认弹窗 */
.confirm-modal {
  max-width: 480px;
}

.confirm-text {
  color: #4b5563;
  font-size: 14px;
  line-height: 1.8;
  background: #f9fafb;
  border-radius: 10px;
  padding: 14px 16px;
  margin: 0 0 20px;
}

.modal-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
</style>
