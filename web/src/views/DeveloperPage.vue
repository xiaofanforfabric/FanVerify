<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="dev-page">
    <nav class="dev-nav">
      <router-link to="/user" class="nav-back">← 返回用户中心</router-link>
      <span class="nav-title">开发者页面</span>
      <span class="nav-placeholder"></span>
    </nav>

    <div class="dev-wrap">
      <div class="dev-card">
        <div class="dev-icon">🚧</div>
        <h1>开发者页面</h1>
        <p class="dev-desc">面向开发者的 API 接入功能正在开发中，敬请期待。</p>
        <p class="dev-sub">开发者接入后，可申请 FanVerify OpenAPI 密钥，将微信动态验证码集成到你的应用（个人网站、论坛、Minecraft 服务器等）。</p>
        <button class="btn-back" @click="router.push('/user')">返回用户中心</button>
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
  max-width: 460px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
  padding: 48px 32px;
  text-align: center;
}

.dev-icon {
  font-size: 52px;
  margin-bottom: 16px;
}

.dev-card h1 {
  font-size: 22px;
  margin: 0 0 12px;
}

.dev-desc {
  color: #6b7280;
  font-size: 14px;
  margin: 0 0 8px;
}

.dev-sub {
  color: #9ca3af;
  font-size: 13px;
  line-height: 1.7;
  margin: 0 0 24px;
}

.btn-back {
  padding: 10px 28px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
</style>
