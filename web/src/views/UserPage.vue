<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()
const loading = ref(true)
const errorMsg = ref('')
const user = ref<{
  uid: number
  level: number
  code_level: number
  reg_time: string
  tag: string
  status: string
  openid: string
} | null>(null)
const ip = ref('')
const lostTime = ref('')
const email = ref('') // 已绑定联系邮箱
const qq = ref('') // 已绑定 QQ(预留)

async function fetchMe() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch(API_BASE + '/api/me', { credentials: 'include' })
    if (res.status === 401) {
      errorMsg.value = '未登录或登录已过期'
      loading.value = false
      return
    }
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    const data = await res.json()
    if (data.userdata && data.userdata[0]) {
      user.value = data.userdata[0]
      ip.value = data.ip || ''
      lostTime.value = data.lost_time || ''
      email.value = data.email || ''
      qq.value = data.qq || ''
      loading.value = false
    } else {
      throw new Error('数据格式错误')
    }
  } catch (e: any) {
    console.error('获取用户信息失败:', e)
    errorMsg.value = e.message || '网络错误'
    loading.value = false
  }
}

function formatTime(ms: string): string {
  const t = new Date(Number(ms))
  if (isNaN(t.getTime())) return ms || '--'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}`
}

function goLogin() {
  router.push('/login')
}

// 前往绑定/更换邮箱页
function goBindEmail() {
  router.push('/bind_email')
}

// 汉堡菜单
const menuOpen = ref(false)
function toggleMenu() {
  menuOpen.value = !menuOpen.value
}
function goMenu(path: string) {
  menuOpen.value = false
  router.push(path)
}

// 退出登录：HttpOnly Cookie 前端删不掉，须调用后端 /api/logout 清除
async function goLogout() {
  menuOpen.value = false
  try {
    await fetch(API_BASE + '/api/logout', { method: 'POST', credentials: 'include' })
  } catch (e) {
    console.error('退出登录请求失败:', e)
  }
  router.push('/')
}

onMounted(fetchMe)
</script>

<template>
  <div class="user-page">
    <nav class="user-nav">
      <router-link to="/" class="nav-back">← 返回首页</router-link>
      <span class="nav-title">我的信息</span>
      <div class="nav-right">
        <button class="hamburger" :class="{ active: menuOpen }" @click="toggleMenu" aria-label="菜单">
          <span class="bar"></span>
          <span class="bar"></span>
          <span class="bar"></span>
        </button>

        <div v-if="menuOpen" class="menu-backdrop" @click="menuOpen = false"></div>

        <div v-if="menuOpen" class="menu-dropdown">
          <div class="menu-item" @click="goMenu('/user')">👤 用户中心</div>
          <div class="menu-item" @click="goMenu('/developer')">🛠️ 开发者页面</div>
          <div v-if="user && user.level === 0" class="menu-item admin" @click="goMenu('/admin')">🛡️ 管理员后台</div>
          <div class="menu-item danger" @click="goLogout">🚪 退出登录</div>
        </div>
      </div>
    </nav>

    <div class="user-wrap">
      <!-- 加载中 -->
      <div v-if="loading" class="user-card status-card">
        <div class="status-icon">⏳</div>
        <h2>正在加载…</h2>
      </div>

      <!-- 未登录 / 出错 -->
      <div v-else-if="errorMsg" class="user-card status-card error">
        <div class="status-icon">🔒</div>
        <h2>无法访问</h2>
        <p class="status-desc">{{ errorMsg }}</p>
        <button class="btn-primary" @click="goLogin">去登录</button>
      </div>

      <!-- 用户信息 -->
      <div v-else-if="user" class="user-card">
        <div class="user-head">
          <div class="user-avatar">#{{ user.uid }}</div>
          <div class="user-meta">
            <h2>FanVerify 用户</h2>
            <p class="user-status" :class="user.status === 'ok' ? 'ok' : 'bad'">{{ user.status }}</p>
          </div>
        </div>

        <div class="info-list">
          <div class="info-row">
            <span class="info-key">UID</span>
            <span class="info-val">#{{ user.uid }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">联系邮箱</span>
            <span v-if="email" class="info-val">
              {{ email }}
              <a class="mini-link" @click="goBindEmail">更换</a>
            </span>
            <span v-else class="info-val">
              <span class="unbound">未绑定</span>
              <a class="mini-link" @click="goBindEmail">去绑定</a>
            </span>
          </div>
          <div class="info-row">
            <span class="info-key">等级</span>
            <span class="info-val">Lv.{{ user.level }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">验证码等级</span>
            <span class="info-val">{{ user.code_level === 2 ? '简易 · 4位数字' : user.code_level === 3 ? '高级 · 8位混合' : '普通 · 6位数字' }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">标签</span>
            <span class="info-val">{{ user.tag || '普通用户' }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">注册时间</span>
            <span class="info-val">{{ user.reg_time }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">登录 IP</span>
            <span class="info-val mono">{{ ip }}</span>
          </div>
          <div class="info-row">
            <span class="info-key">登录过期</span>
            <span class="info-val">{{ formatTime(lostTime) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.user-page {
  min-height: 100vh;
  background: #f5f5f7;
  color: #1f2937;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  display: flex;
  flex-direction: column;
}

.user-nav {
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

.nav-right {
  position: relative;
  width: 120px;
  display: flex;
  justify-content: flex-end;
}

.hamburger {
  display: flex;
  flex-direction: column;
  gap: 5px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 10px;
}

.hamburger .bar {
  width: 22px;
  height: 2px;
  background: #1f2937;
  border-radius: 2px;
  transition: transform 0.2s;
}

.menu-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 20;
}

.menu-dropdown {
  position: absolute;
  top: 48px;
  right: 0;
  z-index: 30;
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.12);
  min-width: 168px;
  overflow: hidden;
  border: 1px solid #f3f4f6;
}

.menu-item {
  padding: 13px 18px;
  font-size: 14px;
  color: #1f2937;
  cursor: pointer;
  white-space: nowrap;
}

.menu-item:hover {
  background: #f3f4f6;
}

.menu-item.danger {
  color: #dc2626;
}

.menu-item.danger:hover {
  background: #fef2f2;
}

.menu-item.admin {
  color: #6d28d9;
  font-weight: 600;
}

.menu-item.admin:hover {
  background: #f5f3ff;
}

.menu-item + .menu-item {
  border-top: 1px solid #f3f4f6;
}

.user-wrap {
  flex: 1;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 40px 16px;
}

.user-card {
  width: 100%;
  max-width: 460px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 28px;
}

.status-card {
  text-align: center;
  padding: 48px 28px;
}

.status-icon {
  font-size: 44px;
  margin-bottom: 12px;
}

.status-card h2 {
  font-size: 20px;
  margin: 0 0 10px;
}

.status-desc {
  color: #6b7280;
  font-size: 14px;
  margin: 0 0 20px;
}

.user-head {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.user-avatar {
  width: 60px;
  height: 60px;
  border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-weight: 700;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.user-meta h2 {
  font-size: 18px;
  margin: 0 0 4px;
}

.user-status {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.user-status.ok {
  background: #dcfce7;
  color: #16a34a;
}

.user-status.bad {
  background: #fee2e2;
  color: #dc2626;
}

.info-list {
  border-top: 1px solid #f3f4f6;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 0;
  border-bottom: 1px solid #f3f4f6;
  font-size: 14px;
}

.info-key {
  color: #6b7280;
}

.info-val {
  font-weight: 500;
  word-break: break-all;
  text-align: right;
}

.mono {
  font-family: 'SF Mono', 'Courier New', monospace;
  font-size: 12px;
  color: #4b5563;
}

.btn-primary {
  padding: 10px 28px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.mini-link {
  color: #4f46e5;
  font-size: 13px;
  cursor: pointer;
  margin-left: 8px;
  text-decoration: underline;
}

.unbound {
  color: #9ca3af;
}
</style>
