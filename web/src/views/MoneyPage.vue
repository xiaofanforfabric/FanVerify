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
    script.onload = () => resolve()
    document.head.appendChild(script)
  })
}

// 直接拦截 redeem 网络请求获取 token
async function initCaptcha() {
  await loadCapScript()

  let resolved = false
  const onToken = (token: string) => {
    if (resolved) return
    resolved = true
    step.value = 'loading'
    fetchBill(token)
  }

  // 拦截 fetch 请求，捕获 redeem 响应中的 token
  const originalFetch = window.fetch.bind(window)
  window.fetch = async (input: any, init?: any) => {
    const resp = await originalFetch(input, init)
    const url = typeof input === 'string' ? input : input?.url || ''
    if (url.includes('redeem')) {
      const clone = resp.clone()
      try {
        const body = await clone.json()
        if (body.token) {
          onToken(body.token)
        } else if (body.data?.token) {
          onToken(body.data.token)
        }
      } catch (_) {}
    }
    return resp
  }

  // 30 秒超时
  setTimeout(() => {
    if (resolved) return
    errorMsg.value = '验证超时，请刷新页面重试'
    step.value = 'error'
  }, 30000)
}

onMounted(() => {
  initCaptcha()
})

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

function reloadPage() {
  window.location.reload()
}
</script>

<template>
  <div class="money-page">
    <!-- 导航栏 -->
    <nav class="money-nav">
      <a href="#/" class="nav-back">← 返回首页</a>
      <span class="nav-title">运营成本公示</span>
      <span class="nav-placeholder"></span>
    </nav>

    <!-- Cap.js 验证组件（已内联到验证卡片中） -->

    <!-- 内容区 -->
    <div class="money-content">
      <!-- 等待验证 -->
      <div v-if="step === 'captcha'" class="status-card">
        <div class="status-icon">🛡️</div>
        <h2>人机验证</h2>
        <p class="status-desc">请完成下方验证后查看运营成本数据</p>
        <div class="captcha-box">
          <cap-widget
            data-cap-api-endpoint="https://cap.fanverify.cn/22a34a230e/"
          ></cap-widget>
        </div>
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
        <button class="btn-retry" @click="reloadPage">重新加载</button>
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

/* Cap.js 容器 */
.captcha-box {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
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
