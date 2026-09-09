import { createRouter, createWebHashHistory } from 'vue-router'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('../views/HomePage.vue'),
    },
    {
      path: '/money',
      name: 'money',
      component: () => import('../views/MoneyPage.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginPage.vue'),
    },
    {
      path: '/user',
      name: 'user',
      component: () => import('../views/UserPage.vue'),
    },
    {
      path: '/developer',
      name: 'developer',
      component: () => import('../views/DeveloperPage.vue'),
    },
    {
      path: '/need_dev',
      name: 'need_dev',
      component: () => import('../views/NeedDevPage.vue'),
    },
    {
      path: '/bind_email',
      name: 'bind_email',
      component: () => import('../views/BindEmailPage.vue'),
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('../views/AdminPage.vue'),
    },
  ],
})

export default router
