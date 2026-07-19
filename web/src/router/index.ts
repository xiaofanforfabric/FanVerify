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
  ],
})

export default router
