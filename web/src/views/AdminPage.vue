<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'
const router = useRouter()

// 登录 / 权限
const loading = ref(true)
const errorMsg = ref('') // 401 或非 root
const isAdmin = ref(false)

// Tab
type Tab = 'users' | 'devs' | 'visas'
const tab = ref<Tab>('users')

// 用户列表
interface AdminUser {
  uid: number
  level: number
  code_level: number
  reg_time: string
  tag: string
  status: string
  phone: string
  qq: string
  email: string
}
const users = ref<AdminUser[]>([])
const userBusy = ref<number | null>(null)

// 开发者令牌
interface AdminDev {
  id: number
  devaccesstoken: string
  nel: number
  buid: number
  mode: string
  bip: string
  sme: string
  status: string
  date_of_issue: string
  owner_tag: string
  owner_email: string
}
const devs = ref<AdminDev[]>([])
const devBusy = ref<number | null>(null)

// 申请列表
interface AdminVisa {
  id: number
  uid: number
  type: string
  other_type: string
  serverip: string
  service_adderss: string
  connect_type: string
  tag: string
  status: string
  end_time: string
  admin_see: string
  reviewed: string
  plan_id: string | null
  online: number
  offline: number
}
const visas = ref<AdminVisa[]>([])
const visaBusy = ref<number | null>(null)
const actionMsg = ref('')

async function fetchMe() {
  loading.value = true
  try {
    const res = await fetch(API_BASE + '/api/me', { credentials: 'include' })
    if (res.status === 401) {
      errorMsg.value = '未登录或登录已过期'
      return
    }
    if (!res.ok) throw new Error('请求失败')
    const data = await res.json()
    const u = data.userdata && data.userdata[0]
    if (!u) throw new Error('数据格式错误')
    if (Number(u.level) !== 0) {
      errorMsg.value = '仅管理员(root)可访问'
      return
    }
    isAdmin.value = true
    await Promise.all([fetchUsers(), fetchDevs(), fetchVisas()])
  } catch (e: any) {
    console.error('初始化失败:', e)
    errorMsg.value = e.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function fetchUsers() {
  const res = await fetch(API_BASE + '/api/admin/getalluser', { credentials: 'include' })
  const data = await res.json().catch(() => ({}))
  if (res.ok && data.success) users.value = data.data?.users || []
  else actionMsg.value = data.error || '加载用户失败'
}
async function fetchDevs() {
  const res = await fetch(API_BASE + '/api/admin/getdev', { credentials: 'include' })
  const data = await res.json().catch(() => ({}))
  if (res.ok && data.success) devs.value = data.data?.devs || []
}
async function fetchVisas() {
  const res = await fetch(API_BASE + '/api/admin/visalist', { credentials: 'include' })
  const data = await res.json().catch(() => ({}))
  if (res.ok && data.success) visas.value = data.data?.visas || []
}

// 封禁/解封
async function toggleBan(u: AdminUser) {
  userBusy.value = u.uid
  actionMsg.value = ''
  try {
    if (u.status === 'bad') {
      const res = await fetch(`${API_BASE}/api/admin/ban_user?uid=${u.uid}`, { credentials: 'include' })
      const data = await res.json().catch(() => ({}))
      if (res.ok) { u.status = 'ok' }
      else actionMsg.value = data.error || '解封失败'
    } else {
      const res = await fetch(API_BASE + '/api/admin/ban_user', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ token: '', uid: String(u.uid) }),
      })
      const data = await res.json().catch(() => ({}))
      if (res.ok) { u.status = 'bad' }
      else actionMsg.value = data.error || '封禁失败'
    }
  } catch (e) {
    actionMsg.value = '网络错误'
  } finally {
    userBusy.value = null
  }
}

// 冻结/解冻
async function toggleIce(d: AdminDev) {
  devBusy.value = d.id
  actionMsg.value = ''
  const frozen = d.status.startsWith('ice')
  try {
    const res = await fetch(API_BASE + '/api/admin/editdev', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ devkey: d.devaccesstoken, type: frozen ? 'unice' : 'ice' }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok) d.status = frozen ? 'ok' : 'ice' + Date.now()
    else actionMsg.value = data.error || '操作失败'
  } catch (e) {
    actionMsg.value = '网络错误'
  } finally {
    devBusy.value = null
  }
}

// 申请处理 pass/refused
async function handleVisa(v: AdminVisa, typ: 'pass' | 'refused') {
  visaBusy.value = v.id
  actionMsg.value = ''
  try {
    const res = await fetch(API_BASE + '/api/admin/editvisa', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ id: String(v.id), type: typ }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok) {
      v.reviewed = 'yes'
      if (typ === 'pass') {
        if (v.admin_see === 'yes' || !v.plan_id) {
          v.status = 'ok'
        }
      } else {
        v.status = 'fail'
      }
    } else actionMsg.value = data.error || '操作失败'
  } catch (e) {
    actionMsg.value = '网络错误'
  } finally {
    visaBusy.value = null
  }
}

function typeLabel(t: string): string {
  return ({ web: '网站', javamc: 'Java MC', cmc: '基岩 MC', none: '其他' } as any)[t] || t
}
function statusLabel(s: string): string {
  if (s.startsWith('ice')) return '已冻结'
  return ({ wait: '审核中', ok: '正常', bad: '已封禁', fail: '未通过' } as any)[s] || s
}
function statusClass(s: string): string {
  if (s.startsWith('ice') || s === 'bad' || s === 'fail') return 'bad'
  return 'ok'
}
function fmtTime(ms: string): string {
  const t = new Date(Number(ms))
  if (isNaN(t.getTime())) return ms || '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}`
}
function goLogin() { router.push('/login') }
function goUser() { router.push('/user') }

onMounted(fetchMe)
</script>

<template>
  <div class="admin-page">
    <nav class="admin-nav">
      <router-link to="/user" class="nav-back">← 返回用户中心</router-link>
      <span class="nav-title">管理员后台</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="admin-body">
      <!-- 加载中 -->
      <div v-if="loading" class="box-card status-card">
        <div class="status-icon">⏳</div>
        <h2>正在校验管理员身份…</h2>
      </div>

      <!-- 无权 / 未登录 -->
      <div v-else-if="errorMsg" class="box-card status-card">
        <div class="status-icon">🔒</div>
        <h2>无法访问</h2>
        <p class="status-desc">{{ errorMsg }}</p>
        <button v-if="errorMsg.includes('未登录')" class="btn-primary" @click="goLogin">去登录</button>
        <button v-else class="btn-primary" @click="goUser">返回用户中心</button>
      </div>

      <!-- 管理面板 -->
      <template v-else-if="isAdmin">
        <div class="admin-tabs">
          <button class="tab" :class="{ active: tab === 'users' }" @click="tab = 'users'">👥 用户管理 ({{ users.length }})</button>
          <button class="tab" :class="{ active: tab === 'devs' }" @click="tab = 'devs'">🔑 开发者令牌 ({{ devs.length }})</button>
          <button class="tab" :class="{ active: tab === 'visas' }" @click="tab = 'visas'">📋 申请审查 ({{ visas.length }})</button>
        </div>

        <p v-if="actionMsg" class="action-msg">{{ actionMsg }}</p>

        <!-- 用户管理 -->
        <div v-if="tab === 'users'" class="box-card">
          <h2 class="list-title">👥 全部用户</h2>
          <p v-if="!users.length" class="empty-tip">暂无用户</p>
          <div v-for="u in users" :key="u.uid" class="row-item" :class="{ banned: u.status === 'bad' }">
            <div class="row-main">
              <span class="badge uid">#{{ u.uid }}</span>
              <span class="badge lv">Lv.{{ u.level }}</span>
              <span class="badge" :class="u.status === 'ok' ? 'ok' : 'bad'">{{ statusLabel(u.status) }}</span>
            </div>
            <div class="row-meta">
              手机：{{ u.phone || '未绑定' }} · 邮箱：{{ u.email || '未绑定' }} · QQ：{{ u.qq || '—' }} · 注册：{{ u.reg_time }}
              <span v-if="u.tag" class="row-meta-tag">· 标签：{{ u.tag }}</span>
            </div>
            <button class="btn-mini" :class="u.status === 'bad' ? 'green' : 'red'" :disabled="userBusy === u.uid" @click="toggleBan(u)">
              {{ userBusy === u.uid ? '处理中…' : u.status === 'bad' ? '解封' : '封禁' }}
            </button>
          </div>
        </div>

        <!-- 开发者令牌 -->
        <div v-if="tab === 'devs'" class="box-card">
          <h2 class="list-title">🔑 全部开发者令牌</h2>
          <p v-if="!devs.length" class="empty-tip">暂无开发者令牌</p>
          <div v-for="d in devs" :key="d.id" class="row-item">
            <div class="row-main">
              <span class="badge mode">{{ d.mode || 'HTTP' }}</span>
              <span class="badge" :class="d.status.startsWith('ice') ? 'bad' : 'ok'">{{ statusLabel(d.status) }}</span>
              <span class="badge lv">NEL Lv.{{ d.nel }}</span>
              <span class="row-token mono">{{ d.devaccesstoken }}</span>
            </div>
            <div class="row-meta">
              归属 UID #{{ d.buid }}{{ d.owner_tag ? ' · ' + d.owner_tag : '' }}{{ d.owner_email ? ' · ' + d.owner_email : '' }} ·
              BIP {{ d.bip || '—' }} · 签发 {{ fmtTime(d.date_of_issue) }}
            </div>
            <div v-if="d.sme" class="row-meta">{{ d.sme }}</div>
            <button class="btn-mini" :class="d.status.startsWith('ice') ? 'green' : 'red'" :disabled="devBusy === d.id" @click="toggleIce(d)">
              {{ devBusy === d.id ? '处理中…' : d.status.startsWith('ice') ? '解冻' : '冻结' }}
            </button>
          </div>
        </div>

        <!-- 申请审查 -->
        <div v-if="tab === 'visas'" class="box-card">
          <h2 class="list-title">📋 全部申请</h2>
          <p class="list-hint">🟥 红色 = 管理员尚未审查（reviewed=no）</p>
          <p v-if="!visas.length" class="empty-tip">暂无申请</p>
          <div v-for="v in visas" :key="v.id" class="row-item" :class="{ unreviewed: v.reviewed === 'no' }">
            <div class="row-main">
              <span class="badge uid">#{{ v.id }}</span>
              <span class="badge type">{{ typeLabel(v.type) }}</span>
              <span v-if="v.type === 'none' && v.other_type" class="badge other">{{ v.other_type }}</span>
              <span class="badge" :class="v.admin_see === 'yes' ? 'admin' : ''">{{ v.admin_see === 'yes' ? '人工审核' : '自动检测' }}</span>
              <span class="badge" :class="statusClass(v.status)">{{ statusLabel(v.status) }}</span>
              <span v-if="v.reviewed === 'no'" class="badge warn">未审查</span>
            </div>
            <div class="row-meta">用户 #{{ v.uid }} · {{ v.serverip }} → {{ v.service_adderss }}</div>
            <div v-if="v.tag" class="row-meta">{{ v.tag }}</div>
            <div v-if="v.plan_id" class="row-meta">检测进度：🟢{{ v.online }} / 🔴{{ v.offline }} · 结束 {{ fmtTime(v.end_time) }}</div>
            <div v-else class="row-meta">无自动检测 · 结束 {{ fmtTime(v.end_time) }}</div>
            <div v-if="v.status === 'wait'" class="row-actions">
              <button class="btn-mini green" :disabled="visaBusy === v.id" @click="handleVisa(v, 'pass')">✓ 通过</button>
              <button class="btn-mini red" :disabled="visaBusy === v.id" @click="handleVisa(v, 'refused')">✗ 拒绝</button>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.admin-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}
.admin-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
}
.nav-back { color: #4f46e5; text-decoration: none; font-size: 14px; width: 120px; }
.nav-title { font-weight: 600; font-size: 16px; }
.nav-placeholder { width: 120px; }

.admin-body {
  flex: 1;
  padding: 28px 16px 48px;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.admin-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
  margin-bottom: 18px;
  max-width: 760px;
  width: 100%;
}
.tab {
  padding: 9px 18px;
  border: 1px solid #e5e7eb;
  background: #fff;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 600;
  color: #6b7280;
  cursor: pointer;
}
.tab.active { background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; border-color: transparent; }

.action-msg { color: #dc2626; font-size: 13px; margin: 0 0 12px; }

.box-card {
  width: 100%;
  max-width: 760px;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 22px 24px;
}
.status-card { text-align: center; padding: 48px 28px; }
.status-icon { font-size: 44px; margin-bottom: 12px; }
.status-card h2 { font-size: 20px; margin: 0 0 10px; }
.status-desc { color: #6b7280; font-size: 14px; margin: 0 0 20px; }

.list-title { font-size: 16px; margin: 0 0 4px; }
.list-hint { color: #9ca3af; font-size: 12px; margin: 2px 0 10px; }
.empty-tip { color: #9ca3af; text-align: center; font-size: 13px; padding: 18px 0; margin: 0; }

.row-item {
  padding: 14px 0;
  border-top: 1px solid #f3f4f6;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.row-item:first-of-type { border-top: none; }
.row-item.unreviewed {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 10px;
  padding: 12px 14px;
  margin-top: 8px;
}
.row-item.banned { opacity: 0.7; }

.row-main { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.row-meta { font-size: 12px; color: #6b7280; line-height: 1.6; word-break: break-all; }
.row-meta-tag { color: #9ca3af; }
.row-token { font-size: 12px; color: #374151; word-break: break-all; }
.row-actions { display: flex; gap: 8px; margin-top: 4px; }

.badge { padding: 2px 10px; border-radius: 999px; font-size: 12px; font-weight: 600; }
.badge.uid { background: #f3f4f6; color: #374151; }
.badge.lv { background: #ede9fe; color: #6d28d9; }
.badge.mode { background: #ede9fe; color: #6d28d9; }
.badge.type { background: #e0f2fe; color: #0369a1; }
.badge.other { background: #f1f5f9; color: #475569; }
.badge.admin { background: #fef3c7; color: #b45309; }
.badge.ok { background: #dcfce7; color: #16a34a; }
.badge.bad { background: #fee2e2; color: #dc2626; }
.badge.warn { background: #fee2e2; color: #b91c1c; }

.mono { font-family: 'SF Mono', 'Courier New', monospace; }

.btn-primary { padding: 10px 26px; background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; border: none; border-radius: 10px; font-size: 14px; font-weight: 600; cursor: pointer; }
.btn-mini { padding: 5px 14px; border-radius: 8px; font-size: 12px; font-weight: 600; cursor: pointer; border: 1px solid #e5e7eb; background: #fff; }
.btn-mini:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-mini.red { color: #dc2626; }
.btn-mini.red:hover { background: #fef2f2; }
.btn-mini.green { color: #059669; }
.btn-mini.green:hover { background: #ecfdf5; }
</style>
