<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'

const API_BASE = 'https://api.fanverify.cn'

const router = useRouter()
const loading = ref(true)
const errorMsg = ref('') // 未登录 / 网络错误
const levelOk = ref(false) // 验证等级 === 2 才有资格申请开发者
const applyClicked = ref(false) // 「立即申请」目前为占位按钮，点击后提示

// 进入页面再次请求 /api/me，按验证等级判断是否能申请开发者
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
    const u = data.userdata && data.userdata[0]
    if (!u) throw new Error('数据格式错误')
    levelOk.value = Number(u.level) === 2
    loading.value = false
  } catch (e: any) {
    console.error('获取用户信息失败:', e)
    errorMsg.value = e.message || '网络错误'
    loading.value = false
  }
}

function goLogin() {
  router.push('/login')
}

function applyPlaceholder() {
  applyClicked.value = true
}

onMounted(fetchMe)
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

      <!-- 验证等级不是 2：无申请资格 -->
      <div v-else-if="!levelOk" class="dev-card status-card">
        <div class="status-icon">📵</div>
        <h1>您不是开发者且帐号等级不足，请升级后再来申请开发者权限</h1>
        <p class="dev-desc">请先在微信小程序内完成「手机号二级认证」，将帐号升级至 Lv.2，再回来申请。</p>
      </div>

      <!-- 等级 2：尚未申请开发者 -->
      <div v-else class="dev-card status-card">
        <div class="status-icon">🛠️</div>
        <h1>您当前不是开发者，需要申请吗？</h1>
        <p class="dev-desc">成为开发者后即可接入 FanVerify OpenAPI，将微信动态验证码集成到你的应用（个人网站、论坛、Minecraft 服务器等）。</p>
        <button v-if="!applyClicked" class="btn-primary" @click="applyPlaceholder">立即申请</button>
        <p v-else class="dev-sub">🚧 申请功能正在开发中，敬请期待</p>
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
</style>
