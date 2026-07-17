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
    desc: '基于时间的一次性密码，150秒自动刷新'
  },
  {
    icon: '📱',
    title: '微信小程序',
    desc: '原生小程序开发，流畅的用户体验'
  },
  {
    icon: '⚡',
    title: '快速响应',
    desc: 'Go 后端高性能处理，毫秒级响应'
  },
  {
    icon: '🛡️',
    title: '安全可靠',
    desc: 'HMAC-SHA1 加密，多重安全防护'
  }
]

const techStack = [
  { name: '微信小程序', desc: '前端应用' },
  { name: 'Go 1.26', desc: '后端服务' },
  { name: 'Vue 3 + TS', desc: 'Web 管理' },
  { name: 'MySQL/SQLite', desc: '数据存储' }
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
      
      <div class="hero-content">
        <div class="logo-section">
          <img src="/logo.png" alt="FanVerify Logo" class="site-logo" />
          <h1 class="brand-name">FanVerify</h1>
        </div>
        
        <p class="tagline">微信小程序2FA鉴权</p>
        <p class="subtitle">基于微信小程序的双因素身份验证解决方案</p>
        
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
          <a class="btn btn-primary" href="#features">了解更多</a>
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
        </div>
      </div>
    </section>

    <!-- Architecture Section -->
    <section class="architecture">
      <h2 class="section-title">系统架构</h2>
      
      <div class="arch-diagram">
        <div class="arch-layer">
          <div class="arch-box frontend">
            <div class="box-title">前端层</div>
            <div class="box-content">微信小程序</div>
          </div>
        </div>
        
        <div class="arch-arrow">↓</div>
        
        <div class="arch-layer">
          <div class="arch-box backend">
            <div class="box-title">后端层</div>
            <div class="box-content">Go API Server</div>
          </div>
        </div>
        
        <div class="arch-arrow">↓</div>
        
        <div class="arch-layer">
          <div class="arch-box database">
            <div class="box-title">数据层</div>
            <div class="box-content">MySQL / SQLite</div>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section class="cta">
      <h2 class="cta-title">准备开始？</h2>
      <p class="cta-desc">查看文档了解如何部署和使用 FanVerify</p>
      <div class="cta-actions">
        <a class="btn btn-primary btn-lg" href="#" onclick="alert('文档开发中...')">查看文档</a>
        <a class="btn btn-outline btn-lg" href="#" onclick="alert('敬请期待...')">下载源码</a>
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

/* Architecture */
.architecture {
  padding: 6rem 2rem;
  max-width: 800px;
  margin: 0 auto;
}

.arch-diagram {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
}

.arch-layer {
  width: 100%;
  max-width: 400px;
}

.arch-box {
  padding: 2rem;
  border-radius: 12px;
  text-align: center;
  border: 2px solid;
}

.arch-box.frontend {
  background: #eef2ff;
  border-color: #6366f1;
}

.arch-box.backend {
  background: #f3e8ff;
  border-color: #a855f7;
}

.arch-box.database {
  background: #fce7f3;
  border-color: #ec4899;
}

.box-title {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 0.5rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.box-content {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
}

.arch-arrow {
  font-size: 2rem;
  color: #6366f1;
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
