import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  config => {
    const username = localStorage.getItem('username')
    const password = localStorage.getItem('password')
    if (username && password) {
      config.headers.Authorization = 'Basic ' + btoa(username + ':' + password)
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 响应拦截器
request.interceptors.response.use(
  response => {
    return response.data
  },
  error => {
    if (error.response?.status === 401) {
      ElMessage.error('认证失败，请重新登录')
      localStorage.removeItem('username')
      localStorage.removeItem('password')
      router.push('/login')
      return Promise.reject(error)
    }
    
    ElMessage.error(error.response?.data?.error || '请求失败')
    return Promise.reject(error)
  }
)

export default request

// API模块
export const userAPI = {
  list: () => request.get('/users/list'),
  login: () => request.get('/users/login'),
  loginCheck: (key) => request.get('/users/loginCheck', { params: { key } }),
  loginCancel: (key) => request.get('/users/loginCancel', { params: { key } }),
  loginByCookie: (cookies) => request.post('/users/loginByCookie', { cookies }),
  delete: (id) => request.delete(`/users/${id}`)
}

export const taskAPI = {
  list: () => request.get('/tasks/list'),
  create: (data) => request.post('/tasks/create', data),
  update: (id, data) => request.put(`/tasks/${id}`, data),
  delete: (id) => request.delete(`/tasks/${id}`),
  test: (id) => request.get(`/tasks/${id}/test`)
}

export const logAPI = {
  monitor: (params) => request.get('/logs/monitor', { params }),
  report: (params) => request.get('/logs/report', { params })
}
