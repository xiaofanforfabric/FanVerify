# FanVerify Web

FanVerify 项目官网 - 基于 Vue 3 + TypeScript + Vite 构建

## 🚀 项目简介

这是 FanVerify（微信小程序2FA鉴权）的官方网站，展示项目特性、技术架构和使用文档。

## 🛠️ 技术栈

- **Vue 3** - 渐进式 JavaScript 框架
- **TypeScript** - 类型安全
- **Vite** - 下一代前端构建工具
- **CSS3** - 现代化样式（支持深色主题）

## 📦 开发

### 安装依赖

```bash
npm install
```

### 启动开发服务器

```bash
npm run dev
```

访问 http://localhost:5173 查看效果

### 构建生产版本

```bash
npm run build
```

### 预览生产构建

```bash
npm run preview
```

## 🎨 特性

- ✨ **实时 OTP 演示** - 动态展示 TOTP 验证码生成
- 📱 **响应式设计** - 完美适配移动端和桌面端
- 🎭 **动态背景** - 渐变圆环脉动动画
- 🌓 **深色主题** - 科技感十足的视觉设计
- ⚡ **快速加载** - Vite HMR 热更新

## 📁 项目结构

```
GO/web/
├── src/
│   ├── components/
│   │   └── HelloWorld.vue    # 官网主组件
│   ├── assets/                # 静态资源
│   ├── App.vue                # 根组件
│   ├── main.ts                # 入口文件
│   └── style.css              # 全局样式
├── public/
│   ├── favicon.svg            # 网站图标
│   └── icons.svg              # SVG 图标集
├── index.html                 # HTML 模板
├── vite.config.ts             # Vite 配置
└── package.json               # 项目配置
```

## 🔗 相关链接

- [FanVerify 主项目](../)
- [Go 后端文档](../README.md)
- [微信小程序](../../README.md)

## 📝 开发状态

🚧 **项目开发中** - Alpha v0.1

当前为测试页面，正在持续完善中。

## 📄 许可证

© 2026 FanVerify. All rights reserved.
