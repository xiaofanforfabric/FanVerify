<script setup lang="ts">
import { ref, onMounted } from 'vue'

const API_BASE = 'https://api.fanverify.cn'

// 状态管理
const step = ref<'captcha' | 'loading' | 'done' | 'error'>('captcha')
const errorMsg = ref('')
const billData = ref<{
  total_cost: number
  total_pay_amount: number
  item_count: number
  month_count: number
  months: string[]
  monthly: {
    total_cost: number
    total_pay_amount: number
    item_count: number
    month: string
    items: {
      business_name: string
      resource_name: string
      resource_id: string
      cost: number
      pay_amount: number
      fee_begin_time: string
      fee_end_time: string
      month: string
    }[]
  }[]
} | null>(null)

// 获取当前月份
const currentMonth = ref('')

// 加载 Cap.js 脚本
function loadCapScript(): Promise<void> {
  return new Promise((resolve) => {
    if (document.querySelector('script[src*="cap.js/widget"]')) {
      resolve()
      return
    }
    const script = document.createElement('script')
    script.src = 'https://cdn.jsdelivr.net/npm/@cap.js/widget'
    script.async = true
    script.onload = () => {
      // 脚本加载后等待一帧让 widget 完成初始化
      requestAnimationFrame(() => resolve())
    }
    document.head.appendChild(script)
  })
}

// 无感静默验证
async function silentCaptcha() {
  step.value = 'captcha'
  console.log('[Cap] 开始静默验证流程')

  // 等待 widget 元素出现在 DOM 中
  const widget = await new Promise<any>((resolve) => {
    const check = () => {
      const el = document.querySelector('cap-widget')
      if (el) { console.log('[Cap] ✅ widget 元素已找到', el); resolve(el) }
      else setTimeout(check, 50)
    }
    check()
  })

  // 加载 Cap.js 脚本
  await loadCapScript()
  console.log('[Cap] ✅ Cap.js 脚本已加载')

  // 等待 widget 初始化完成
  await new Promise<void>((resolve) => setTimeout(resolve, 1000))
  console.log('[Cap] 已等待 1s 让 widget 初始化')

  // 记录 widget 的所有属性
  console.log('[Cap] widget 属性:', {
    token: (widget as any).token,
    value: (widget as any).value,
    dataset: { ...widget.dataset },
    attrs: [...widget.attributes].map(a => `${a.name}=${a.value}`),
    shadowRoot: !!widget.shadowRoot,
  })

  // 检查 window.Cap 是否存在（Cap.js 的 API）
  console.log('[Cap] window.Cap:', (window as any).Cap)
  if ((window as any).Cap?.widget) {
    console.log('[Cap] 使用 Cap API 获取 token...')
    try {
      const capWidget = (window as any).Cap.widget
      if (capWidget.getToken) {
        const t = capWidget.getToken()
        console.log('[Cap] Cap.getToken():', t)
        if (t) { fetchBill(t); return }
      }
    } catch (e) { console.error('[Cap] Cap API 调用失败:', e) }
  }

  // Cap.js 在 invisible 模式下会自动验证
  console.log('[Cap] 调用 widget.verify()...')
  try { widget.verify?.() } catch (e) { console.log('[Cap] verify 调用结果:', e) }

  let resolved = false
  const done = (token: string, source: string) => {
    if (resolved) { console.log('[Cap] ⏭ 已处理，跳过'); return }
    console.log(`[Cap] 🎯 通过 ${source} 获取到 token:`, token.slice(0, 10) + '...')
    resolved = true
    clearTimeout(timeoutId)
    clearInterval(pollTimer)
    fetchBill(token)
  }
  const fail = (msg: string) => {
    if (resolved) return
    console.log('[Cap] ❌ 验证失败:', msg)
    resolved = true
    clearTimeout(timeoutId)
    clearInterval(pollTimer)
    errorMsg.value = msg
    step.value = 'error'
  }

  // 1. 监听所有自定义事件，看看 Cap.js 到底发了什么
  const allEvents = ['success', 'error', 'cap-success', 'cap-error', 'token', 'complete', 'ready', 'load']
  allEvents.forEach(name => {
    widget.addEventListener(name, (e: Event) => {
      console.log(`[Cap] 📢 事件 "${name}"`, {
        type: e.type,
        detail: (e as CustomEvent).detail,
        target: e.target,
        currentTarget: e.currentTarget,
      })
      const detail = (e as CustomEvent).detail
      if (detail?.token && typeof detail.token === 'string') {
        done(detail.token, `事件:${name}`)
      }
    })
  })

  // 2. 在 document 上监听
  document.addEventListener('success', (e: Event) => {
    const target = e.target as Element
    console.log('[Cap] 📢 document 捕获 success', { target: target?.tagName, detail: (e as CustomEvent).detail })
    if (target?.tagName === 'CAP-WIDGET') {
      const token = (e as CustomEvent).detail?.token
      if (token) done(token, 'document.success')
    }
  })

  // 3. 拦截 fetch 获取 redeem 的响应（终极方案）
  const originalFetch = window.fetch
  window.fetch = async (...args) => {
    const url = typeof args[0] === 'string' ? args[0] : args[0]?.url
    const resp = await originalFetch(...args)
    if (typeof url === 'string' && url.includes('redeem')) {
      const clone = resp.clone()
      try {
        const json = await clone.json()
        console.log('[Cap] 🔥 redeem 响应拦截:', json)
        if (json.token) done(json.token, 'fetch拦截')
        if (json.data?.token) done(json.data.token, 'fetch拦截.data')
      } catch (_) {}
    }
    return resp
  }

  // 4. 轮询获取 token
  const pollTimer = setInterval(() => {
    if (resolved) return
    try {
      const sources = [
        { val: (widget as any).token, src: 'widget.token' },
        { val: (widget as any).value, src: 'widget.value' },
        { val: widget.getAttribute('data-token'), src: 'data-token' },
        { val: widget.getAttribute('token'), src: 'attr.token' },
        { val: widget.getAttribute('value'), src: 'attr.value' },
        { val: (widget as any).response?.token, src: 'widget.response.token' },
      ]
      for (const s of sources) {
        if (s.val && typeof s.val === 'string' && s.val.length > 8) {
          done(s.val, `轮询:${s.src}`)
          return
        }
      }
    } catch (_) { /* ignore */ }
  }, 200)

  // 15 秒超时
  const timeoutId = setTimeout(() => {
    console.log('[Cap] ⏰ 超时！最后检查 widget:', {
      token: (widget as any).token,
      value: (widget as any).value,
      attrs: [...widget.attributes].map(a => `${a.name}=${a.value}`),
    })
    fail('验证超时，请刷新页面重试')
  }, 15000)
}

// 获取账单数据
async function fetchBill(token: string) {
  step.value = 'loading'
  errorMsg.value = ''

  const now = new Date()
  const month = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  currentMonth.value = month

  try {
    const url = `${API_BASE}/api/getmoneylist?month=${month}&captcha_token=${encodeURIComponent(token)}`
    console.log('📡 请求账单:', url.replace(token, '***'))
    const res = await fetch(url)

    if (!res.ok) {
      let errMsg = `HTTP ${res.status}`
      try {
        const errData = await res.json()
        errMsg = errData.error || errMsg
      } catch (_) {
        // 忽略解析错误
      }
      throw new Error(errMsg)
    }

    const data = await res.json()
    billData.value = data
    step.value = 'done'
  } catch (e: any) {
    console.error('❌ 账单请求失败:', e)
    errorMsg.value = e.message || '网络错误，请检查控制台'
    step.value = 'error'
  }
}

// 格式化金额
function formatMoney(n: number): string {
  return n.toFixed(2)
}

// 获取当前月份中文
function monthLabel(month: string): string {
  const parts = month.split('-')
  return `${parts[0]}年${parts[1]}月`
}

onMounted(() => {
  // 页面加载后自动开始静默验证
  setTimeout(() => silentCaptcha(), 500)
})
</script>

<template>
  <div class="money-page">
    <!-- 导航栏 -->
    <nav class="money-nav">
      <a href="#/" class="nav-back">← 返回首页</a>
      <span class="nav-title">运营成本公示</span>
      <span class="nav-placeholder"></span>
    </nav>

    <!-- CAPTCHA 阶段（隐藏的验证组件，置于屏幕外保证能正常初始化） -->
    <div class="captcha-container">
      <cap-widget
        data-cap-api-endpoint="https://cap.fanverify.cn/22a34a230e/"
        data-mode="invisible"
      ></cap-widget>
    </div>

    <!-- 内容区 -->
    <div class="money-content">
      <!-- 验证中 -->
      <div v-if="step === 'captcha'" class="status-card">
        <div class="status-icon">🛡️</div>
        <h2>正在安全验证...</h2>
        <p class="status-desc">静默检测中，无需任何操作</p>
        <div class="spinner"></div>
      </div>

      <!-- 加载中 -->
      <div v-else-if="step === 'loading'" class="status-card">
        <div class="status-icon">📊</div>
        <h2>正在获取账单数据</h2>
        <div class="spinner"></div>
      </div>

      <!-- 错误 -->
      <div v-else-if="step === 'error'" class="status-card error">
        <div class="status-icon">❌</div>
        <h2>加载失败</h2>
        <p class="status-desc">{{ errorMsg }}</p>
        <button class="btn-retry" @click="silentCaptcha">重新加载</button>
      </div>

      <!-- 账单数据 -->
      <div v-else-if="step === 'done' && billData" class="bill-container">
        <div class="bill-header">
          <div class="bill-period">累计运营成本</div>
          <div class="bill-summary">
            <div class="summary-item">
              <span class="summary-label">覆盖月份</span>
              <span class="summary-value">{{ billData.month_count }} 个月</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">原价总成本</span>
              <span class="summary-value summary-original">¥{{ formatMoney(billData.total_cost) }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">实际支付</span>
              <span class="summary-value">¥{{ formatMoney(billData.total_pay_amount) }}</span>
            </div>
          </div>
        </div>

        <!-- 逐月展示 -->
        <div v-for="monthly in billData.monthly" :key="monthly.month" class="bill-section">
          <h3 class="section-title">
            ☁️ {{ monthLabel(monthly.month) }}
            <span class="section-sub">¥{{ formatMoney(monthly.total_pay_amount) }}</span>
          </h3>
          <div class="bill-table">
            <div class="table-header">
              <span class="col-name">资源名称</span>
              <span class="col-type">类型</span>
              <span class="col-cost">实付金额</span>
            </div>
            <div
              v-for="item in monthly.items"
              :key="item.resource_id"
              class="table-row"
            >
              <span class="col-name">{{ item.resource_name || item.resource_id }}</span>
              <span class="col-type">{{ item.business_name }}</span>
              <span class="col-cost">¥{{ formatMoney(item.pay_amount) }}</span>
            </div>
          </div>
        </div>

        <p class="bill-footnote">
          数据来源：腾讯云账单 · 仅展示服务器与域名相关费用 · 实际成本以腾讯云账单为准
        </p>
      </div>
    </div>

    <!-- 备案信息 -->
    <footer class="money-footer">
      <div class="money-footer-icp">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">陕ICP备2026018433号</a>
        <span class="icp-sep">|</span>
        <a href="http://www.beian.gov.cn/portal/registerSystemInfo" target="_blank" rel="noopener">
          <img src="/110.png" alt="公安备案图标" class="beian-icon"/>
          陕公网安备61100202000217号
        </a>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.money-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.money-content {
  flex: 1;
}

/* 导航栏 */
.money-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  position: sticky;
  top: 0;
  z-index: 10;
}

.nav-back {
  color: #4f46e5;
  text-decoration: none;
  font-size: 0.875rem;
  font-weight: 500;
  transition: color 0.2s;
}

.nav-back:hover {
  color: #6366f1;
}

.nav-title {
  font-size: 1rem;
  font-weight: 700;
  color: #1f2937;
}

.nav-placeholder {
  width: 5rem;
}

/* 状态卡片 */
.status-card {
  text-align: center;
  padding: 4rem 2rem;
  max-width: 420px;
  margin: 4rem auto;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  border: 1px solid #e5e7eb;
}

.status-card.error {
  border-color: #fecaca;
}

.status-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.status-card h2 {
  font-size: 1.25rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}

.status-desc {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 1.5rem;
}

.btn-retry {
  padding: 0.625rem 1.5rem;
  background: #4f46e5;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}

.btn-retry:hover {
  background: #6366f1;
}

/* 加载动画 */
.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid #e5e7eb;
  border-top-color: #4f46e5;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin: 0 auto;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 账单内容 */
.bill-container {
  max-width: 720px;
  margin: 2rem auto;
  padding: 0 1rem;
}

.bill-header {
  background: linear-gradient(135deg, #eef2ff, #faf5ff);
  border: 1px solid #e0e7ff;
  border-radius: 16px;
  padding: 2rem;
  margin-bottom: 1.5rem;
  text-align: center;
}

.bill-period {
  font-size: 1.5rem;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 1rem;
}

.bill-summary {
  display: flex;
  justify-content: center;
  gap: 3rem;
}

.summary-item {
  text-align: center;
}

.summary-label {
  display: block;
  font-size: 0.75rem;
  color: #6b7280;
  margin-bottom: 0.25rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.summary-value {
  font-size: 1.75rem;
  font-weight: 700;
  color: #4f46e5;
}

.summary-original {
  font-size: 1.25rem;
  color: #9ca3af;
  text-decoration: line-through;
  font-weight: 500;
}

/* 表格 */
.bill-section {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  overflow: hidden;
  margin-bottom: 1rem;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  padding: 1rem 1.25rem;
  margin: 0;
  border-bottom: 1px solid #f3f4f6;
  color: #374151;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-sub {
  font-size: 0.8125rem;
  font-weight: 600;
  color: #4f46e5;
}

.bill-table {
  padding: 0.5rem 0;
}

.table-header {
  display: flex;
  padding: 0.625rem 1.25rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid #f3f4f6;
}

.table-row {
  display: flex;
  padding: 0.75rem 1.25rem;
  font-size: 0.875rem;
  align-items: center;
  transition: background 0.15s;
}

.table-row:hover {
  background: #f9fafb;
}

.col-name {
  flex: 1;
  font-weight: 500;
  color: #1f2937;
}

.col-type {
  width: 120px;
  color: #6b7280;
  font-size: 0.8125rem;
}

.col-cost {
  width: 100px;
  text-align: right;
  font-weight: 600;
  color: #1f2937;
  font-variant-numeric: tabular-nums;
}

.bill-footnote {
  text-align: center;
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 1rem;
  padding-bottom: 2rem;
  line-height: 1.6;
}

/* Cap.js 容器 - 置于屏幕外确保能正常初始化 */
.captcha-container {
  position: absolute;
  left: -9999px;
  top: 0;
  width: 1px;
  height: 1px;
  overflow: hidden;
}

/* 备案信息 */
.money-footer {
  text-align: center;
  padding: 1.5rem 1rem;
  border-top: 1px solid #e5e7eb;
  background: #ffffff;
  margin-top: 2rem;
}

.money-footer-icp {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  font-size: 0.75rem;
}

.money-footer-icp a {
  color: #6b7280;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  transition: color 0.2s;
}

.money-footer-icp a:hover {
  color: #4f46e5;
}

.money-footer-icp .beian-icon {
  width: 16px;
  height: 16px;
  vertical-align: middle;
}

.money-footer-icp .icp-sep {
  color: #9ca3af;
}

/* 响应式 */
@media (max-width: 640px) {
  .money-nav {
    padding: 0.75rem 1rem;
  }

  .bill-summary {
    gap: 2rem;
  }

  .summary-value {
    font-size: 1.375rem;
  }

  .col-type {
    width: 80px;
  }

  .col-cost {
    width: 80px;
  }
}
</style>
