/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// Cap.js 人机验证组件
declare namespace JSX {
  interface IntrinsicElements {
    'cap-widget': any
  }
}
