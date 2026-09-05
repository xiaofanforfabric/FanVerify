<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()
const loading = ref(true)
const errorMsg = ref('') // 未登录 / 网络错误
const isDev = ref(false) // 是否已是开发者（有密钥或审核中的申请）
const level = ref(0) // 用户验证等级
const devKey = ref<DevKeyItem[]>([]) // 所有可用密钥
const devVisa = ref<DevVisaItem[]>([]) // 所有审核中的申请

interface DevKeyItem {
  id: number
  devaccesstoken: string
  nel: number
  bip: string
  mode: string
  sme: string
  status: string
}
interface DevVisaItem {
  id: number
  type: string
  other_type: string
  serverip: string
  service_adderss: string
  connect_type: string
  tag: string
  status: string
  end_time: string
  admin_see: string
  plan_id: string | null
}

// 进入页面请求 /api/user/isdev：一次性拿到「是否开发者 + 等级 + 可用密钥 + 审核中的申请」
async function fetchIsDev() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch(API_BASE + '/api/user/isdev', { credentials: 'include' })
    if (res.status === 401) {
      errorMsg.value = '未登录或登录已过期'
      loading.value = false
      return
    }
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    const data = await res.json()
    const u = data.userdata && data.userdata[0]
    if (!u) throw new Error('数据格式错误')
    isDev.value = !!data.is_dev
    level.value = Number(u.level)
    devKey.value = data.dev_key || []
    devVisa.value = data.dev_visa || []
    loading.value = false
  } catch (e: any) {
    console.error('获取开发者状态失败:', e)
    errorMsg.value = e.message || '网络错误'
    loading.value = false
  }
}

function typeLabel(t: string): string {
  return ({ web: '网站', javamc: 'Java MC', cmc: '基岩 MC', none: '其他' } as any)[t] || t || '—'
}
function connLabel(c: string): string {
  return ({ http: 'HTTP', wsc: '正向 WS', wss: '反向 WS' } as any)[c] || c || '—'
}
function shortToken(t: string): string {
  return t ? t.slice(0, 16) + '…' : ''
}
function fmtTime(ms: string): string {
  const t = new Date(Number(ms))
  if (isNaN(t.getTime())) return ms || '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}`
}

function goLogin() {
  router.push('/login')
}
// 立即申请 / 申请新业务：进入开发者入驻申请页（协议弹窗 + 表单）
function applyNow() {
  router.push('/need_dev')
}

onMounted(fetchIsDev)
</script>

<template>
  <div class="dev-page">
    <nav class="dev-nav">
      <router-link to="/user" class="nav-back">← 返回用户中心</router-link>
      <span class="nav-title">开发者页面</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="dev-wrap">
      <!-- 加载中 -->
      <div v-if="loading" class="dev-card status-card">
        <div class="status-icon">⏳</div>
        <h1>正在加载…</h1>
      </div>

      <!-- 未登录 / 网络错误 -->
      <div v-else-if="errorMsg" class="dev-card status-card">
        <div class="status-icon">🔒</div>
        <h1>无法访问</h1>
        <p class="dev-desc">{{ errorMsg }}</p>
        <button class="btn-primary" @click="goLogin">去登录</button>
      </div>

      <!-- 已是开发者：有可用密钥 或 有审核中的申请 -->
      <div v-else-if="isDev" class="dev-stack">
        <div class="dev-card status-card">
          <div class="status-icon">🛠️</div>
          <h1>您已是开发者</h1>
          <p class="dev-desc">以下是您的业务密钥与申请进度。密钥管理功能正在开发中。</p>
          <div class="chips">
            <span class="chip">🔑 可用密钥 {{ devKey.length }}</span>
            <span class="chip">📋 审核中的申请 {{ devVisa.length }}</span>
          </div>
          <button class="btn-primary" @click="applyNow">申请新业务</button>
        </div>

        <div v-if="devKey.length" class="dev-card list-card">
          <h2 class="list-title">🔑 可用密钥</h2>
          <div v-for="k in devKey" :key="k.id" class="row-item">
            <div class="row-main">
              <span class="badge mode">{{ k.mode || 'HTTP' }}</span>
              <span class="row-token mono">{{ shortToken(k.devaccesstoken) }}</span>
            </div>
            <p v-if="k.sme" class="row-sub">{{ k.sme }}</p>
            <p class="row-meta">绑定 IP：{{ k.bip || '不限制' }} · 最低等级 Lv.{{ k.nel }}</p>
          </div>
        </div>

        <div v-if="devVisa.length" class="dev-card list-card">
          <h2 class="list-title">📋 审核中的申请</h2>
          <div v-for="v in devVisa" :key="v.id" class="row-item">
            <div class="row-main">
              <span class="badge type">{{ typeLabel(v.type) }}</span>
              <span class="badge conn">{{ connLabel(v.connect_type) }}</span>
              <span v-if="v.type === 'none' && v.other_type" class="row-token">{{ v.other_type }}</span>
              <span v-if="v.admin_see === 'yes'" class="badge admin">人工审核</span>
            </div>
            <p class="row-sub mono">{{ v.serverip }} → {{ v.service_adderss }}</p>
            <p v-if="v.tag" class="row-meta">{{ v.tag }}</p>
            <p class="row-meta">预计审核结束：{{ fmtTime(v.end_time) }}</p>
          </div>
        </div>

        <p class="dev-note">💡 每个帐号最多可创建 10 个业务密钥，审核等待期间与删除后 3 天内无法重复申请相同业务。</p>
      </div>

      <!-- 不是开发者 -->
      <template v-else>
        <!-- 验证等级不是 2：无申请资格 -->
        <div v-if="level < 2" class="dev-card status-card">
          <div class="status-icon">📵</div>
          <h1>您不是开发者且帐号等级不足，请升级后再来申请开发者权限</h1>
          <p class="dev-desc">请先在微信小程序内完成「手机号二级认证」，将帐号升级至 Lv.2，再回来申请。</p>
        </div>

        <!-- 等级 2：尚未申请开发者 -->
        <div v-else class="dev-card status-card">
          <div class="status-icon">🛠️</div>
          <h1>您当前不是开发者，需要申请吗？</h1>
          <p class="dev-desc">成为开发者后即可接入 FanVerify OpenAPI，将微信动态验证码集成到你的应用（个人网站、论坛、Minecraft 服务器等）。</p>
          <button class="btn-primary" @click="applyNow">立即申请</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.dev-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.dev-nav {
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

.dev-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
}

.dev-card {
  width: 100%;
  max-width: 480px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 48px 32px;
  text-align: center;
}

.status-icon {
  font-size: 52px;
  margin-bottom: 16px;
  line-height: 1;
}

.dev-card h1 {
  font-size: 21px;
  line-height: 1.45;
  margin: 0 0 14px;
}

.dev-desc {
  color: #6b7280;
  font-size: 14px;
  line-height: 1.7;
  margin: 0 auto 26px;
  max-width: 340px;
}

.dev-sub {
  color: #9ca3af;
  font-size: 13px;
  line-height: 1.7;
  margin: 12px 0 0;
}

.btn-primary {
  padding: 11px 34px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
}

/* 已是开发者视图 */
.dev-stack {
  width: 100%;
  max-width: 540px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 16px;
}

.dev-stack .dev-card {
  max-width: 540px;
  box-sizing: border-box;
}

.dev-card.list-card {
  text-align: left;
  padding: 24px;
}

.list-title {
  font-size: 16px;
  margin: 0 0 4px;
}

.chips {
  display: flex;
  gap: 10px;
  justify-content: center;
  margin-bottom: 22px;
  flex-wrap: wrap;
}

.chip {
  background: #f3f4f6;
  color: #374151;
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
}

.row-item {
  padding: 14px 0;
  border-top: 1px solid #f3f4f6;
}

.row-item:first-child {
  border-top: none;
}

.row-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.badge {
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.badge.mode {
  background: #ede9fe;
  color: #6d28d9;
}

.badge.type {
  background: #e0f2fe;
  color: #0369a1;
}

.badge.conn {
  background: #dcfce7;
  color: #15803d;
}

.badge.admin {
  background: #fef3c7;
  color: #b45309;
}

.row-token {
  font-size: 12px;
  color: #6b7280;
}

.row-sub {
  margin: 6px 0 2px;
  font-size: 13px;
  color: #374151;
  word-break: break-all;
}

.row-meta {
  margin: 2px 0 0;
  font-size: 12px;
  color: #9ca3af;
  line-height: 1.6;
  word-break: break-all;
}

.mono {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.dev-note {
  font-size: 12px;
  color: #9ca3af;
  line-height: 1.7;
  text-align: center;
  margin: 0;
  padding: 0 8px;
}
</style>
