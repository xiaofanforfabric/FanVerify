<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()
const loading = ref(true)
const errorMsg = ref('') // 未登录 / 网络错误
const isDev = ref(false)
const level = ref(0)
const devKey = ref<DevKeyItem[]>([])
const devVisa = ref<DevVisaItem[]>([])

interface DevKeyItem {
  id: number
  devaccesstoken: string
  nel: number
  buid: number
  bip: string
  mode: string
  sme: string
  status: string
  date_of_issue: string
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
  online: number // 保活检测成功(在线)次数
  offline: number // 保活检测失败(离线)次数
}

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
function fmtTime(ms: string): string {
  const t = new Date(Number(ms))
  if (isNaN(t.getTime())) return ms || '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}`
}

// 复制密钥（完整展示 + 一键复制）
const copiedToken = ref('')
async function copyToken(k: DevKeyItem) {
  const text = k.devaccesstoken
  try {
    await navigator.clipboard.writeText(text)
    copiedToken.value = text
    setTimeout(() => {
      if (copiedToken.value === text) copiedToken.value = ''
    }, 1600)
  } catch (e) {
    console.error('clipboard 失败，使用降级方案:', e)
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
      copiedToken.value = text
      setTimeout(() => {
        if (copiedToken.value === text) copiedToken.value = ''
      }, 1600)
    } catch (_) {}
    document.body.removeChild(ta)
  }
}

// 修改密钥要求验证等级（1/2）
const editKey = ref<DevKeyItem | null>(null)
const editLevel = ref(1)
const editBusy = ref(false)
const editMsg = ref('')

function openEdit(k: DevKeyItem) {
  editKey.value = k
  editLevel.value = k.nel === 2 ? 2 : 1
  editMsg.value = ''
}
function closeEdit() {
  editKey.value = null
  editBusy.value = false
  editMsg.value = ''
}
async function submitEdit() {
  if (!editKey.value) return
  editBusy.value = true
  editMsg.value = ''
  try {
    const res = await fetch(API_BASE + '/api/dev/editdevkey', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        token: '',
        devkey: editKey.value.devaccesstoken,
        new_need_leve: String(editLevel.value),
      }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok && data.success) {
      if (editKey.value) editKey.value.nel = Number(editLevel.value)
      closeEdit()
    } else {
      editMsg.value = data.error || `操作失败 (${res.status})`
      if (res.status === 401) setTimeout(() => router.push('/login'), 1200)
    }
  } catch (e) {
    console.error('修改密钥等级失败:', e)
    editMsg.value = '网络错误，请稍后重试'
  } finally {
    editBusy.value = false
  }
}

function goLogin() {
  router.push('/login')
}
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

      <!-- ===== 已是开发者：列表式控制台 ===== -->
      <div v-else-if="isDev" class="dev-stack">
        <!-- 概览条 -->
        <div class="dev-hero">
          <div class="hero-left">
            <div class="hero-icon">🛠️</div>
            <div>
              <h1>开发者控制台</h1>
              <p class="hero-sub">管理你的业务密钥与入驻申请</p>
            </div>
          </div>
          <button class="btn-primary btn-sm" @click="applyNow">＋ 申请新业务</button>
        </div>

        <!-- 可用密钥列表 -->
        <div class="dev-card list-card">
          <div class="list-head">
            <h2 class="list-title">🔑 可用密钥</h2>
            <span class="list-count">{{ devKey.length }}/10</span>
          </div>
          <p v-if="!devKey.length" class="empty-tip">暂无可用密钥</p>
          <div v-for="k in devKey" :key="k.id" class="key-row">
            <div class="key-head">
              <div class="key-badges">
                <span class="badge mode">{{ k.mode || 'HTTP' }}</span>
                <span v-if="k.status === 'ok'" class="badge ok">正常</span>
              </div>
              <div class="key-actions">
                <button class="btn-mini copy" @click="copyToken(k)">
                  {{ copiedToken === k.devaccesstoken ? '✓ 已复制' : '📋 复制密钥' }}
                </button>
                <button class="btn-mini" @click="openEdit(k)">改等级 · Lv.{{ k.nel }}</button>
              </div>
            </div>
            <div class="key-token mono">{{ k.devaccesstoken }}</div>
            <p v-if="k.sme" class="row-sub">{{ k.sme }}</p>
            <p class="row-meta">绑定 IP：{{ k.bip || '不限制' }} · 要求用户等级 Lv.{{ k.nel }} · 签发于 {{ fmtTime(k.date_of_issue) }}</p>
          </div>
        </div>

        <!-- 审核中的申请列表 -->
        <div class="dev-card list-card">
          <div class="list-head">
            <h2 class="list-title">📋 审核中的申请</h2>
            <span class="list-count">{{ devVisa.length }}</span>
          </div>
          <p v-if="!devVisa.length" class="empty-tip">暂无审核中的申请</p>
          <div v-for="v in devVisa" :key="v.id" class="visa-row">
            <div class="key-head">
              <div class="key-badges">
                <span class="badge type">{{ typeLabel(v.type) }}</span>
                <span class="badge conn">{{ connLabel(v.connect_type) }}</span>
                <span v-if="v.type === 'none' && v.other_type" class="badge other">{{ v.other_type }}</span>
              </div>
              <span v-if="v.admin_see === 'yes'" class="badge admin">人工审核</span>
              <span v-else class="badge waiting">保活检测中</span>
            </div>
            <p class="key-sub mono">{{ v.serverip }} → {{ v.service_adderss }}</p>
            <p v-if="v.tag" class="row-sub">{{ v.tag }}</p>

            <!-- 自动保活检测进度：绿=在线 红=离线 -->
            <div v-if="v.admin_see !== 'yes'" class="prog-block">
              <div class="prog-bar">
                <span class="prog-ok" :style="{ width: Math.min(100, (v.online / 12) * 100) + '%' }"></span>
                <span class="prog-bad" :style="{ width: Math.min(100, (v.offline / 12) * 100) + '%' }"></span>
              </div>
              <div class="prog-text">
                <span class="prog-on">🟢 在线 {{ v.online }}</span>
                <span class="prog-off">🔴 离线 {{ v.offline }}</span>
                <span class="prog-total">已检测 {{ v.online + v.offline }}/12 次</span>
                <span v-if="v.online + v.offline >= 12" class="prog-total"> · 满 12 次后将自动结算</span>
              </div>
            </div>
            <p v-else class="row-meta">⏳ 该申请由人工审核，请耐心等待通知</p>

            <p class="row-meta">预计审核结束：{{ fmtTime(v.end_time) }}</p>
          </div>
        </div>

        <p class="dev-note">💡 每个帐号最多 10 个业务密钥；申请将进行 12 次保活检测（每 2 小时一次），在线 ≥8 次即通过并自动颁发密钥。人工审核类（其他）除外。</p>
      </div>

      <!-- ===== 不是开发者 ===== -->
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

    <!-- 修改密钥验证等级弹窗 -->
    <div v-if="editKey" class="modal-mask">
      <div class="modal-box">
        <h3 class="modal-title">修改密钥要求验证等级</h3>
        <p class="row-meta mono modal-token">{{ editKey.devaccesstoken }}</p>
        <div class="opt-group">
          <label class="opt" :class="{ sel: editLevel === 1 }">
            <input v-model="editLevel" type="radio" :value="1" />
            <span class="opt-name">Lv.1</span>
            <span class="opt-desc">普通用户即可完成验证</span>
          </label>
          <label class="opt" :class="{ sel: editLevel === 2 }">
            <input v-model="editLevel" type="radio" :value="2" />
            <span class="opt-name">Lv.2</span>
            <span class="opt-desc">需手机号二级认证的用户</span>
          </label>
        </div>
        <p v-if="editMsg" class="form-error">{{ editMsg }}</p>
        <div class="modal-actions">
          <button class="btn-ghost" @click="closeEdit">取消</button>
          <button class="btn-primary" :disabled="editBusy" @click="submitEdit">{{ editBusy ? '保存中…' : '保存' }}</button>
        </div>
      </div>
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

/* 已是开发者视图：样式见下方“列表式控制台”段落 */
.dev-note {
  font-size: 12px;
  color: #9ca3af;
  line-height: 1.7;
  text-align: center;
  margin: 0;
  padding: 0 8px;
}

/* ===== 列表式控制台 ===== */
.dev-stack {
  width: 100%;
  max-width: 640px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 16px;
}

.dev-stack .dev-card {
  max-width: 640px;
  box-sizing: border-box;
}

.dev-hero {
  width: 100%;
  max-width: 640px;
  box-sizing: border-box;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(102, 126, 234, 0.25);
  padding: 22px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.hero-icon {
  font-size: 36px;
}

.dev-hero h1 {
  font-size: 19px;
  margin: 0 0 4px;
}

.hero-sub {
  margin: 0;
  font-size: 13px;
  opacity: 0.85;
}

.btn-primary.btn-sm {
  padding: 10px 18px;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.5);
}

.dev-card.list-card {
  text-align: left;
  padding: 22px 24px;
}

.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.list-title {
  font-size: 16px;
  margin: 0;
}

.list-count {
  font-size: 12px;
  color: #9ca3af;
  background: #f3f4f6;
  border-radius: 999px;
  padding: 3px 10px;
}

.empty-tip {
  color: #9ca3af;
  font-size: 13px;
  text-align: center;
  padding: 18px 0;
  margin: 0;
}

.key-row,
.visa-row {
  padding: 14px 0;
  border-top: 1px solid #f3f4f6;
}

.key-row:first-of-type,
.visa-row:first-of-type {
  border-top: none;
}

.key-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.key-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.key-actions {
  display: flex;
  gap: 8px;
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

.badge.other {
  background: #f1f5f9;
  color: #475569;
}

.badge.ok {
  background: #dcfce7;
  color: #16a34a;
}

.badge.admin {
  background: #fef3c7;
  color: #b45309;
}

.badge.waiting {
  background: #e0e7ff;
  color: #4338ca;
}

.key-token {
  margin: 8px 0 4px;
  font-size: 13px;
  color: #1f2937;
  background: #f8fafc;
  border: 1px dashed #e2e8f0;
  border-radius: 8px;
  padding: 8px 10px;
  word-break: break-all;
  user-select: all;
}

.key-sub {
  margin: 8px 0 2px;
  font-size: 13px;
  color: #374151;
  word-break: break-all;
}

.row-sub {
  margin: 4px 0 2px;
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

.btn-mini {
  padding: 5px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid #e5e7eb;
  background: #fff;
  color: #4f46e5;
  transition: background 0.15s;
}

.btn-mini:hover {
  background: #eef2ff;
}

.btn-mini.copy {
  color: #059669;
}

.btn-mini.copy:hover {
  background: #ecfdf5;
}

/* 检测进度条 */
.prog-block {
  margin-top: 10px;
}

.prog-bar {
  height: 8px;
  border-radius: 999px;
  background: #f3f4f6;
  overflow: hidden;
  display: flex;
}

.prog-ok {
  height: 100%;
  background: linear-gradient(90deg, #34d399, #10b981);
  transition: width 0.3s;
}

.prog-bad {
  height: 100%;
  background: linear-gradient(90deg, #fca5a5, #ef4444);
  transition: width 0.3s;
}

.prog-text {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  margin-top: 6px;
  font-size: 12px;
}

.prog-on {
  color: #16a34a;
  font-weight: 600;
}

.prog-off {
  color: #dc2626;
  font-weight: 600;
}

.prog-total {
  color: #9ca3af;
}

/* 改等级弹窗 */
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
  max-width: 400px;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  padding: 24px;
}

.modal-title {
  font-size: 17px;
  margin: 0 0 8px;
}

.modal-token {
  background: #f8fafc;
  border-radius: 8px;
  padding: 6px 8px;
  margin: 0 0 14px;
  word-break: break-all;
}

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

.form-error {
  color: #dc2626;
  font-size: 13px;
  margin: 12px 0 0;
}

.modal-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 18px;
}

.btn-ghost {
  padding: 10px 22px;
  background: #fff;
  color: #374151;
  border: 2px solid #e5e7eb;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
</style>
