import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Dashboard from '@/views/Dashboard.vue'
import { getCredentials } from '@/utils/authStorage'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login
  },
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard,
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const { username, password } = getCredentials()
  
  if (to.meta.requiresAuth && (!username || !password)) {
    next('/login')
  } else if (to.path === '/login' && username && password) {
    next('/')
  } else {
    next()
  }
})

export default router
