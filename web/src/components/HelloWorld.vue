<script setup lang="ts">
import { ref, onMounted } from 'vue'

const countdown = ref(150)
const demoCode = ref('------')
const isGenerating = ref(false)

// 模拟验证码生成
const generateDemoCode = () => {
  isGenerating.value = true
  setTimeout(() => {
    demoCode.value = Math.random().toString().slice(2, 8)
    countdown.value = 150
    isGenerating.value = false
    startCountdown()
  }, 500)
}

// 倒计时
const startCountdown = () => {
  const timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
      generateDemoCode()
    }
  }, 1000)
}

onMounted(() => {
  generateDemoCode()
})

const features = [
  {
    icon: '🔐',
    title: 'TOTP 动态验证',
    desc: '基于时间的一次性密码算法，150秒自动刷新，支持多级安全策略',
    points: ['HMAC-SHA1 加密算法', '150 秒时间窗口', '3 级验证强度可调']
  },
  {
    icon: '📱',
    title: '微信小程序',
    desc: '原生小程序开发，即开即用，无需下载安装，扫码即可完成鉴权',
    points: ['微信原生框架', '扫码即鉴权', '零安装成本']
  },
  {
    icon: '🔌',
    title: 'OpenAPI 接口',
    desc: '类似 authMC 插件，但用微信小程序动态验证码替代密码登录，安全便捷',
    points: ['博客/论坛快速接入', 'MC 服务端插件支持', 'Token + IP 双重鉴权']
  },
  {
    icon: '🛡️',
    title: '多重安全防护',
    desc: '从传输到存储全方位安全保障，限流防刷、过期失效、权限分级',
    points: ['接口限流防刷', '验证码过期失效', '用户数据权限隔离']
  }
]

const techStack = [
  { name: '微信小程序', desc: '前端应用', detail: '原生框架，即开即用，零安装成本' },
  { name: 'Go 1.26', desc: '后端服务', detail: '高性能编译型语言，毫秒级并发处理' },
  { name: 'Vue 3 + TS', desc: 'Web 管理', detail: '渐进式框架，类型安全，开发体验优秀' },
  { name: 'MySQL/SQLite', desc: '数据存储', detail: '双数据库支持，开发测试无缝切换' }
]

const apiDocs = [
  { icon: '🔑', name: 'OTP申请接口', desc: '生成一次性OTP令牌，用于扫码鉴权场景', doc: 'https://doc.fanverify.cn/485863981e0.md' },
  { icon: '🖼️', name: 'OTP二维码包装接口', desc: '将OTP包装为标准二维码，方便展示和传播', doc: 'https://doc.fanverify.cn/485907612e0.md' },
  { icon: '🔄', name: 'OTP轮询接口', desc: '轮询OTP验证状态，5秒间隔防刷限制', doc: 'https://doc.fanverify.cn/485907646e0.md', tag: '5s限频' },
  { icon: '✅', name: '用户验证接口', desc: '验证用户动态验证码，完成身份确认', doc: 'https://doc.fanverify.cn/486499010e0.md' },
  { icon: '📋', name: '获取用户数据', desc: '获取已验证用户的信息，权限隔离防滥用', doc: 'https://doc.fanverify.cn/487111702e0.md' },
  { icon: 'ℹ️', name: '开发者令牌信息', desc: '查询开发者自身令牌的统计和状态信息', doc: 'https://doc.fanverify.cn/487762584e0.md' },
  { icon: '🏷️', name: '用户风险标签接口', desc: '给用户打风险标签，需要消耗有效验证次数', doc: 'https://doc.fanverify.cn/488453042e0.md' }
]

const useCases = [
  { icon: '📝', title: '个人博客用户登录', desc: '替代传统密码登录，用户用微信小程序获取动态验证码即可登录你的博客' },
  { icon: '💬', title: '论坛会员登录', desc: '论坛接入 FanVerify，会员凭微信 2FA 动态码登录，告别密码泄露风险' },
  { icon: '⛏️', title: 'Minecraft 服务器登录', desc: '类似 authMC 插件，但用微信小程序动态验证码代替密码输入，防破解防盗号' },
  { icon: '🔐', title: '任何网站/应用的登录认证', desc: '通用的 2FA 登录方案，只需对接 OpenAPI 即可用微信动态码替代密码' }
]

const flowSteps = [
  { step: '1', title: '用户请求登录', desc: '用户在博客/论坛/MC 服务器发起登录请求' },
  { step: '2', title: '生成 OTP 二维码', desc: '服务端调用 OpenAPI 生成一次性 OTP 令牌并展示二维码' },
  { step: '3', title: '微信扫码获取验证码', desc: '用户打开微信小程序，扫码后获取 150 秒有效的动态验证码' },
  { step: '4', title: '输入动态码完成登录', desc: '用户将验证码提交到网站/MC 服务器，完成身份验证并登录' }
]
</script>

<template>
  <div class="fanverify-site">
    <!-- Hero Section -->
    <section class="hero">
      <div class="hero-bg">
        <div class="ring ring-1"></div>
        <div class="ring ring-2"></div>
        <div class="ring ring-3"></div>
      </div>

      <!-- 右上角登录入口 -->
      <div class="hero-login">
        <router-link to="/login" class="login-btn">登录</router-link>
      </div>

      <div class="hero-content">
        <div class="logo-section">
          <img src="/logo.png" alt="FanVerify Logo" class="site-logo" />
          <h1 class="brand-name">FanVerify</h1>
        </div>
        
        <p class="tagline">个人博客 · 论坛 · Minecraft 服务器</p>
        <p class="subtitle">用微信小程序动态验证码替代密码登录——像 authMC 一样简单，比密码更安全</p>
        
        <!-- Demo Card -->
        <div class="demo-card">
          <div class="demo-header">
            <span class="demo-label">实时演示</span>
            <span class="demo-status">● LIVE</span>
          </div>
          
          <div class="demo-code-display">
            <div class="code-label">OTP TOKEN</div>
            <div class="code-value" :class="{ generating: isGenerating }">
              {{ demoCode }}
            </div>
          </div>
          
          <div class="demo-progress">
            <div class="progress-bar">
              <div class="progress-fill" :style="{ width: (countdown / 150 * 100) + '%' }"></div>
            </div>
            <div class="countdown-text">{{ Math.floor(countdown / 60) }}:{{ String(countdown % 60).padStart(2, '0') }}</div>
          </div>
        </div>
        
        <div class="hero-actions">
          <a class="btn btn-primary" href="#/" onclick="document.getElementById('features')?.scrollIntoView({behavior:'smooth'});return false">了解更多</a>
          <a class="btn btn-outline" href="#/money">
            <span class="btn-icon-text">📊</span>
            运营成本
          </a>
          <a class="btn btn-secondary" href="https://github.com" target="_blank">
            <svg class="icon" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
            </svg>
            GitHub
          </a>
        </div>
      </div>
    </section>

    <!-- Status Banner -->
    <div class="status-banner">
      <div class="status-content">
        <span class="status-icon">🚧</span>
        <span class="status-text">项目开发中</span>
        <span class="status-badge">Alpha v0.1</span>
      </div>
    </div>

    <!-- Features Section -->
    <section id="features" class="features">
      <h2 class="section-title">核心特性</h2>
      
      <div class="features-grid">
        <div v-for="feature in features" :key="feature.title" class="feature-card">
          <div class="feature-icon">{{ feature.icon }}</div>
          <h3 class="feature-title">{{ feature.title }}</h3>
          <p class="feature-desc">{{ feature.desc }}</p>
          <ul class="feature-points">
            <li v-for="point in feature.points" :key="point">{{ point }}</li>
          </ul>
        </div>
      </div>
    </section>

    <!-- Tech Stack Section -->
    <section class="tech-stack">
      <h2 class="section-title">技术栈</h2>
      
      <div class="tech-grid">
        <div v-for="tech in techStack" :key="tech.name" class="tech-item">
          <div class="tech-name">{{ tech.name }}</div>
          <div class="tech-desc">{{ tech.desc }}</div>
          <div class="tech-detail">{{ tech.detail }}</div>
        </div>
      </div>
    </section>

    <!-- API Overview Section -->
    <section class="api-overview">
      <h2 class="section-title">API 概览</h2>
      <p class="section-subtitle">FanVerify 提供以下 OpenAPI 接口，方便第三方开发者集成</p>
      
      <div class="api-table">
        <a v-for="api in apiDocs" :key="api.name" :href="api.doc" target="_blank" rel="noopener" class="api-row">
          <span class="api-row-icon">{{ api.icon }}</span>
          <div class="api-row-body">
            <div class="api-row-header">
              <span class="api-row-name">{{ api.name }}</span>
              <span v-if="api.tag" class="api-row-tag">{{ api.tag }}</span>
            </div>
            <span class="api-row-desc">{{ api.desc }}</span>
          </div>
          <span class="api-row-arrow">→</span>
        </a>
      </div>
    </section>

    <!-- Flow Section -->
    <section class="flow-section">
      <h2 class="section-title">验证流程</h2>
      <p class="section-subtitle">一次完整的双因素身份验证过程</p>
      
      <div class="flow-steps">
        <div v-for="(item, idx) in flowSteps" :key="item.step" class="flow-step">
          <div class="flow-step-number">{{ item.step }}</div>
          <div class="flow-step-body">
            <h3 class="flow-step-title">{{ item.title }}</h3>
            <p class="flow-step-desc">{{ item.desc }}</p>
          </div>
          <div v-if="idx < flowSteps.length - 1" class="flow-step-line"></div>
        </div>
      </div>
    </section>

    <!-- Use Cases Section -->
    <section class="use-cases">
      <h2 class="section-title">适用场景</h2>
      
      <div class="cases-grid">
        <div v-for="item in useCases" :key="item.title" class="case-card">
          <div class="case-icon">{{ item.icon }}</div>
          <h3 class="case-title">{{ item.title }}</h3>
          <p class="case-desc">{{ item.desc }}</p>
        </div>
      </div>
    </section>

    <!-- Architecture Section -->
    <section class="architecture">
      <h2 class="section-title">系统架构</h2>

      <div class="arch-flow">
        <!-- 前端层 -->
        <div class="arch-node frontend-node">
          <div class="node-icon">📱</div>
          <div class="node-title">前端层</div>
          <div class="node-desc">微信小程序</div>
        </div>

        <div class="arch-connector">
          <div class="connector-line"></div>
          <div class="connector-arrow">▼</div>
        </div>

        <!-- 后端 OpenAPI 分组 -->
        <div class="arch-group">
          <div class="group-label">后端 OpenAPI</div>
          <div class="group-cards">
            <div class="group-card">
              <div class="card-header">
                <span class="card-icon">🔐</span>
                <span class="card-title">OTP 服务</span>
              </div>
              <div class="card-items">
                <div class="card-item">OTP申请接口</div>
                <div class="card-item">OTP二维码包装接口</div>
                <div class="card-item">OTP轮询接口 <span class="item-tag">5s</span></div>
              </div>
            </div>
            <div class="group-card">
              <div class="card-header">
                <span class="card-icon">✅</span>
                <span class="card-title">验证服务</span>
              </div>
              <div class="card-items">
                <div class="card-item">用户验证接口</div>
                <div class="card-item">获取用户数据</div>
              </div>
            </div>
            <div class="group-card">
              <div class="card-header">
                <span class="card-icon">⚙️</span>
                <span class="card-title">管理服务</span>
              </div>
              <div class="card-items">
                <div class="card-item">开发者令牌信息</div>
                <div class="card-item">用户风险标签接口</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section class="cta">
      <h2 class="cta-title">准备开始？</h2>
      <p class="cta-desc">查看文档了解如何部署和使用 FanVerify</p>
      <div class="cta-actions">
        <a class="btn btn-primary btn-lg" href="/" onclick="alert('文档开发中...');return false">查看文档</a>
        <a class="btn btn-outline btn-lg" href="/" onclick="alert('敬请期待...');return false">下载源码</a>
      </div>
    </section>

    <!-- Sponsor Section -->
    <section class="sponsor">
      <div class="sponsor-card">
        <div class="sponsor-icon">❤️</div>
        <h2 class="sponsor-title">为爱发电</h2>
        <p class="sponsor-desc">
          FanVerify 是个人纯免费项目，服务器、域名等运营成本均由我一人承担。
          如果你觉得这个项目对你有帮助，可以自愿赞助支持我，帮助减轻运营压力，
          让项目能持续稳定地为大家服务。
        </p>
        <div class="sponsor-actions">
          <a class="btn btn-sponsor" href="https://afdian.com/a/xiaofanforfabric" target="_blank" rel="noopener">
            <span class="sponsor-btn-icon">☕</span>
            请我喝杯咖啡
          </a>
        </div>
        <p class="sponsor-footnote">赞助纯属自愿，不赞助也不影响任何功能使用 ❤️</p>
      </div>
    </section>

    <!-- Footer -->
    <footer class="footer">
      <div class="footer-content">
        <div class="footer-brand">
          <div class="footer-logo">FanVerify</div>
          <p class="footer-tagline">安全 · 可靠 · 高效</p>
        </div>
        
        <div class="footer-links">
          <div class="footer-column">
            <h4>产品</h4>
            <a href="#">小程序</a>
            <a href="#">OpenAPI</a>
            <a href="#">文档</a>
          </div>
          
          <div class="footer-column">
            <h4>开发</h4>
            <a href="#">GitHub</a>
            <a href="#">API 参考</a>
            <a href="#">更新日志</a>
          </div>
          
          <div class="footer-column">
            <h4>关于</h4>
            <a href="#">团队</a>
            <a href="#">联系我们</a>
            <a href="#">许可证</a>
          </div>
        </div>
      </div>
      
      <div class="footer-bottom">
        <p>© 2026 FanVerify. All rights reserved.</p>
        <p class="footer-tech">Powered by Vue 3 + TypeScript + Vite</p>
        <div class="footer-icp">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">陕ICP备2026018433号</a>
          <span class="icp-sep">|</span>
          <a href="http://www.beian.gov.cn/portal/registerSystemInfo" target="_blank" rel="noopener">
            <img src="/110.png" alt="公安备案图标" class="beian-icon"/>
            陕公网安备61100202000217号
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.fanverify-site {
  min-height: 100vh;
  background: #ffffff;
  color: #1f2937;
  overflow-x: hidden;
}

/* Hero Section */
.hero {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2rem;
  overflow: hidden;
}

/* 右上角登录按钮 */
.hero-login {
  position: absolute;
  top: 24px;
  right: 28px;
  z-index: 20;
}

.login-btn {
  display: inline-block;
  padding: 8px 22px;
  border: 1px solid rgba(79, 70, 229, 0.4);
  border-radius: 999px;
  color: #4f46e5;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(8px);
  transition: all 0.2s;
}

.login-btn:hover {
  background: #4f46e5;
  color: #fff;
}

.hero-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
}

.ring {
  position: absolute;
  border: 2px solid rgba(79, 70, 229, 0.12);
  border-radius: 50%;
  animation: pulse 8s ease-in-out infinite;
}

.ring-1 {
  width: 300px;
  height: 300px;
  top: 20%;
  left: 10%;
  animation-delay: 0s;
}

.ring-2 {
  width: 400px;
  height: 400px;
  top: 40%;
  right: 15%;
  animation-delay: 2s;
}

.ring-3 {
  width: 250px;
  height: 250px;
  bottom: 20%;
  left: 50%;
  animation-delay: 4s;
}

@keyframes pulse {
  0%, 100% { transform: scale(1); opacity: 0.3; }
  50% { transform: scale(1.1); opacity: 0.6; }
}

.hero-content {
  position: relative;
  z-index: 1;
  text-align: center;
  max-width: 800px;
}

.logo-section {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.site-logo {
  width: 56px;
  height: 56px;
  object-fit: contain;
}

.brand-name {
  font-size: 3.5rem;
  font-weight: 700;
  background: linear-gradient(135deg, #4f46e5 0%, #6366f1 50%, #818cf8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
}

.tagline {
  font-size: 1.5rem;
  color: #374151;
  margin: 0.5rem 0;
  font-weight: 500;
}

.subtitle {
  font-size: 1rem;
  color: #6b7280;
  margin-bottom: 2rem;
}

/* Demo Card */
.demo-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 16px;
  padding: 2rem;
  margin: 2rem auto;
  max-width: 400px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
}

.demo-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.demo-label {
  font-size: 0.875rem;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.demo-status {
  color: #059669;
  font-size: 0.875rem;
  font-weight: 600;
}

.demo-code-display {
  text-align: center;
  margin-bottom: 1.5rem;
}

.code-label {
  font-size: 0.75rem;
  color: #6b7280;
  margin-bottom: 0.5rem;
  letter-spacing: 0.1em;
}

.code-value {
  font-size: 2.5rem;
  font-weight: 700;
  font-family: 'Courier New', monospace;
  color: #4f46e5;
  letter-spacing: 0.2em;
  transition: all 0.3s ease;
}

.code-value.generating {
  opacity: 0.5;
  transform: scale(0.95);
}

.demo-progress {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.progress-bar {
  flex: 1;
  height: 6px;
  background: #e5e7eb;
  border-radius: 3px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #4f46e5, #6366f1);
  transition: width 1s linear;
  border-radius: 3px;
}

.countdown-text {
  font-family: 'Courier New', monospace;
  font-size: 0.875rem;
  color: #6b7280;
  min-width: 40px;
}

/* Buttons */
.hero-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  margin-top: 2rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border-radius: 8px;
  font-weight: 600;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.3s ease;
  font-size: 0.875rem;
}

.btn-primary {
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  color: white;
  border: none;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 30px rgba(79, 70, 229, 0.3);
}

.btn-secondary {
  background: #f3f4f6;
  color: #4f46e5;
  border: 1px solid #e5e7eb;
}

.btn-secondary:hover {
  background: #e5e7eb;
  border-color: #d1d5db;
}

.btn-outline {
  background: transparent;
  color: #4f46e5;
  border: 2px solid #c7d2fe;
}

.btn-outline:hover {
  background: #eef2ff;
}

.btn-lg {
  padding: 1rem 2rem;
  font-size: 1rem;
}

.icon {
  width: 20px;
  height: 20px;
}

/* Status Banner */
.status-banner {
  background: #f0fdf4;
  border-top: 1px solid #bbf7d0;
  border-bottom: 1px solid #bbf7d0;
  padding: 1rem;
}

.status-content {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
}

.status-icon {
  font-size: 1.25rem;
}

.status-text {
  color: #16a34a;
  font-weight: 600;
}

.status-badge {
  background: #dcfce7;
  color: #16a34a;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
}

/* Features Section */
.features {
  padding: 6rem 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.section-title {
  text-align: center;
  font-size: 2.5rem;
  font-weight: 700;
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 3rem;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 2rem;
}

.feature-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 2rem;
  text-align: center;
  transition: all 0.3s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
}

.feature-card:hover {
  transform: translateY(-5px);
  border-color: #c7d2fe;
  box-shadow: 0 10px 30px rgba(79, 70, 229, 0.1);
}

.feature-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.feature-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 0.5rem;
}

.feature-desc {
  color: #6b7280;
  font-size: 0.875rem;
  line-height: 1.6;
}

/* Tech Stack */
.tech-stack {
  padding: 6rem 2rem;
  background: #f9fafb;
  max-width: 1200px;
  margin: 0 auto;
}

.tech-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
}

.tech-item {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 1.5rem;
  text-align: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
}

.tech-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: #4f46e5;
  margin-bottom: 0.5rem;
}

.tech-desc {
  font-size: 0.875rem;
  color: #6b7280;
}

.tech-detail {
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 0.5rem;
}

/* Feature Points */
.feature-points {
  list-style: none;
  margin-top: 1rem;
  padding: 0;
  text-align: left;
}

.feature-points li {
  position: relative;
  padding-left: 1.25rem;
  font-size: 0.8125rem;
  color: #6b7280;
  line-height: 1.8;
}

.feature-points li::before {
  content: '▸';
  position: absolute;
  left: 0;
  color: #6366f1;
}

/* Section Subtitle */
.section-subtitle {
  text-align: center;
  font-size: 1.0625rem;
  color: #6b7280;
  margin-top: -2rem;
  margin-bottom: 3rem;
}

/* API Overview */
.api-overview {
  padding: 6rem 2rem;
  max-width: 760px;
  margin: 0 auto;
}

.api-table {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.api-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.25rem;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  text-decoration: none;
  transition: all 0.2s ease;
}

.api-row:hover {
  border-color: #c7d2fe;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.08);
  transform: translateX(4px);
}

.api-row-icon {
  font-size: 1.25rem;
  flex-shrink: 0;
}

.api-row-body {
  flex: 1;
  min-width: 0;
}

.api-row-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.125rem;
}

.api-row-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #1f2937;
}

.api-row-tag {
  font-size: 0.625rem;
  font-weight: 600;
  padding: 0.0625rem 0.5rem;
  border-radius: 6px;
  background: #fef3c7;
  color: #b45309;
}

.api-row-desc {
  font-size: 0.8125rem;
  color: #6b7280;
  display: block;
}

.api-row-arrow {
  font-size: 1rem;
  color: #c7d2fe;
  flex-shrink: 0;
  transition: transform 0.2s;
}

.api-row:hover .api-row-arrow {
  transform: translateX(3px);
  color: #6366f1;
}

/* Flow Section */
.flow-section {
  padding: 6rem 2rem;
  background: #f9fafb;
}

.flow-steps {
  max-width: 600px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.flow-step {
  display: flex;
  gap: 1.25rem;
  position: relative;
  padding-bottom: 2rem;
}

.flow-step:last-child {
  padding-bottom: 0;
}

.flow-step-number {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  color: #fff;
  font-size: 0.875rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.flow-step-body {
  flex: 1;
  padding-top: 0.375rem;
}

.flow-step-title {
  font-size: 1rem;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 0.25rem;
}

.flow-step-desc {
  font-size: 0.8125rem;
  color: #6b7280;
  line-height: 1.5;
}

.flow-step-line {
  position: absolute;
  left: 17px;
  top: 36px;
  bottom: 0;
  width: 2px;
  background: linear-gradient(to bottom, #c7d2fe, #e5e7eb);
}

/* Use Cases */
.use-cases {
  padding: 6rem 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.cases-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1.5rem;
}

.case-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 2rem;
  text-align: center;
  transition: all 0.3s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
}

.case-card:hover {
  transform: translateY(-4px);
  border-color: #c7d2fe;
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.1);
}

.case-icon {
  font-size: 2.5rem;
  margin-bottom: 1rem;
}

.case-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 0.5rem;
}

.case-desc {
  font-size: 0.875rem;
  color: #6b7280;
  line-height: 1.6;
}

/* Architecture */
.architecture {
  padding: 6rem 2rem;
  max-width: 800px;
  margin: 0 auto;
}

.arch-flow {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

/* 前端节点 */
.arch-node {
  text-align: center;
  padding: 1.5rem 3rem;
  border-radius: 14px;
  border: 2px solid;
  min-width: 200px;
}

.frontend-node {
  background: #eef2ff;
  border-color: #6366f1;
}

.node-icon {
  font-size: 2rem;
  margin-bottom: 0.5rem;
}

.node-title {
  font-size: 0.75rem;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.25rem;
}

.node-desc {
  font-size: 1.125rem;
  font-weight: 600;
  color: #1f2937;
}

/* 连接线 */
.arch-connector {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 2.5rem;
}

.connector-line {
  width: 2px;
  flex: 1;
  background: linear-gradient(to bottom, #6366f1, #a855f7);
}

.connector-arrow {
  font-size: 0.75rem;
  color: #a855f7;
  line-height: 1;
}

/* OpenAPI 分组 */
.arch-group {
  width: 100%;
  max-width: 680px;
}

.group-label {
  text-align: center;
  font-size: 0.75rem;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 1rem;
}

.group-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
}

.group-card {
  background: #faf5ff;
  border: 1.5px solid #e9d5ff;
  border-radius: 12px;
  padding: 1.25rem;
  transition: box-shadow 0.2s;
}

.group-card:hover {
  box-shadow: 0 4px 12px rgba(168, 85, 247, 0.12);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid #e9d5ff;
}

.card-icon {
  font-size: 1.125rem;
}

.card-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: #7c3aed;
}

.card-items {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.card-item {
  font-size: 0.8125rem;
  color: #4b5563;
  padding: 0.375rem 0.5rem;
  background: rgba(255, 255, 255, 0.7);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.item-tag {
  font-size: 0.625rem;
  font-weight: 600;
  padding: 0.0625rem 0.375rem;
  border-radius: 4px;
  background: #fef3c7;
  color: #b45309;
}

/* CTA */
.cta {
  padding: 6rem 2rem;
  text-align: center;
  background: #f9fafb;
}

.cta-title {
  font-size: 2.5rem;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 1rem;
}

.cta-desc {
  font-size: 1.125rem;
  color: #6b7280;
  margin-bottom: 2rem;
}

.cta-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

/* Footer */
.footer {
  background: #f8fafc;
  border-top: 1px solid #e5e7eb;
  padding: 4rem 2rem 2rem;
}

.footer-content {
  max-width: 1200px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 4rem;
  margin-bottom: 3rem;
}

.footer-brand {
  
}

.footer-logo {
  font-size: 1.5rem;
  font-weight: 700;
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 0.5rem;
}

.footer-tagline {
  color: #6b7280;
  font-size: 0.875rem;
}

.footer-links {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2rem;
}

.footer-column h4 {
  color: #374151;
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 1rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.footer-column a {
  display: block;
  color: #6b7280;
  text-decoration: none;
  font-size: 0.875rem;
  margin-bottom: 0.5rem;
  transition: color 0.2s;
}

.footer-column a:hover {
  color: #4f46e5;
}

.footer-bottom {
  text-align: center;
  padding-top: 2rem;
  border-top: 1px solid #e5e7eb;
  color: #6b7280;
  font-size: 0.875rem;
}

.footer-tech {
  margin-top: 0.5rem;
  font-size: 0.75rem;
}

.footer-icp {
  margin-top: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  font-size: 0.75rem;
}

.footer-icp a {
  color: #6b7280;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  transition: color 0.2s;
}

.footer-icp a:hover {
  color: #4f46e5;
}

.beian-icon {
  width: 16px;
  height: 16px;
  vertical-align: middle;
}

.icp-sep {
  color: #4b5563;
}

/* Sponsor Section */
.sponsor {
  padding: 4rem 2rem;
  background: linear-gradient(135deg, #faf5ff 0%, #fdf2f8 100%);
}

.sponsor-card {
  max-width: 560px;
  margin: 0 auto;
  text-align: center;
  background: #ffffff;
  border: 1px solid #f3e8ff;
  border-radius: 20px;
  padding: 3rem 2.5rem;
  box-shadow: 0 4px 24px rgba(168, 85, 247, 0.08);
}

.sponsor-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  animation: heartbeat 1.5s ease-in-out infinite;
}

@keyframes heartbeat {
  0%, 100% { transform: scale(1); }
  25% { transform: scale(1.1); }
  50% { transform: scale(1); }
  75% { transform: scale(1.05); }
}

.sponsor-title {
  font-size: 2rem;
  font-weight: 700;
  background: linear-gradient(135deg, #ec4899, #a855f7);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 1rem;
}

.sponsor-desc {
  font-size: 0.9375rem;
  color: #6b7280;
  line-height: 1.8;
  margin-bottom: 1.5rem;
}

.sponsor-actions {
  margin-bottom: 1rem;
}

.btn-sponsor {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 2rem;
  background: linear-gradient(135deg, #ec4899, #a855f7);
  color: #fff;
  border: none;
  border-radius: 50px;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.3s ease;
  cursor: pointer;
}

.btn-sponsor:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(168, 85, 247, 0.35);
}

.sponsor-btn-icon {
  font-size: 1.25rem;
}

.sponsor-footnote {
  font-size: 0.75rem;
  color: #9ca3af;
  font-style: italic;
}

/* Responsive */
@media (max-width: 768px) {
  .brand-name {
    font-size: 2.5rem;
  }
  
  .tagline {
    font-size: 1.25rem;
  }
  
  .hero-actions {
    flex-direction: column;
  }
  
  .section-title {
    font-size: 2rem;
  }
  
  .features-grid {
    grid-template-columns: 1fr;
  }
  
  .group-cards {
    grid-template-columns: 1fr;
  }
  
  .cases-grid {
    grid-template-columns: 1fr;
  }
  
  .section-subtitle {
    font-size: 0.9375rem;
    padding: 0 1rem;
  }
  
  .footer-content {
    grid-template-columns: 1fr;
    gap: 2rem;
  }
  
  .footer-links {
    grid-template-columns: 1fr;
  }
  
  .cta-actions {
    flex-direction: column;
    align-items: stretch;
  }
  
  .btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
