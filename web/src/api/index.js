import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { clearCredentials, getCredentials } from '@/utils/authStorage'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  config => {
    const { username, password } = getCredentials()
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
    const data = response.data
    if (isBlob(data)) {
      return data
    }
    if (data && typeof data === 'object' && Object.prototype.hasOwnProperty.call(data, 'code')) {
      if (data.code !== 0) {
        const message = data.error || data.message || '请求失败'
        const error = new Error(message)
        error.businessMessage = message
        error.response = response
        return Promise.reject(error)
      }
      return Object.prototype.hasOwnProperty.call(data, 'data') ? data.data : data
    }
    const message = businessErrorMessage(data)
    if (message) {
      const error = new Error(message)
      error.businessMessage = message
      error.response = response
      return Promise.reject(error)
    }
    return data
  }
)

request.interceptors.response.use(
  response => {
    return response
  },
  error => {
    if (error.response?.status === 401) {
      ElMessage.error('认证失败，请重新登录')
      clearCredentials()
      router.push('/login')
      return Promise.reject(error)
    }
    
    ElMessage.error(normalizeErrorMessage(error))
    return Promise.reject(error)
  }
)

function businessErrorMessage(data) {
  if (!data || typeof data !== 'object' || isBlob(data)) {
    return ''
  }
  if (data.error) {
    return data.error
  }
  if (data.type === 'error' && data.msg) {
    return data.msg
  }
  return ''
}

function normalizeErrorMessage(error) {
  const data = error.response?.data
  if (error.businessMessage) return error.businessMessage
  if (data && typeof data === 'object' && !isBlob(data)) {
    return data.error || data.message || data.msg || '请求失败'
  }
  return error.message || '请求失败'
}

function isBlob(value) {
  return typeof Blob !== 'undefined' && value instanceof Blob
}

export default request

// API模块
export const userAPI = {
  list: () => request.get('/users/list'),
  login: () => request.get('/users/login'),
  loginCheck: (key) => request.get('/users/loginCheck', { params: { key } }),
  loginCancel: (key) => request.get('/users/loginCancel', { params: { key } }),
  loginByCookie: (cookies) => request.post('/users/loginByCookie', { cookies }),
  check: (id) => request.post(`/users/${id}/check`),
  delete: (id, params) => request.delete(`/users/${id}`, { params })
}

export const taskAPI = {
  list: () => request.get('/tasks/list'),
  progress: () => request.get('/tasks/progress'),
  getProgress: (id) => request.get(`/tasks/${id}/progress`),
  updateStatus: (id, data) => request.post(`/tasks/${id}/status`, data),
  create: (data) => request.post('/tasks/create', data),
  update: (id, data) => request.put(`/tasks/${id}`, data),
  delete: (id, params) => request.delete(`/tasks/${id}`, { params }),
  test: (id) => request.get(`/tasks/${id}/test`)
}

export const logAPI = {
  monitor: (params) => request.get('/logs/monitor', { params }),
  report: (params) => request.get('/logs/report', { params }),
  exportReport: (params) => request.get('/logs/report/export', { params, responseType: 'blob' })
}

export const keywordAPI = {
  list: () => request.get('/keywords/list'),
  create: (data) => request.post('/keywords/create', data),
  update: (id, data) => request.put(`/keywords/${id}`, data),
  delete: (id, params) => request.delete(`/keywords/${id}`, { params }),
  preview: (data) => request.post('/keywords/preview', data)
}

export const whitelistAPI = {
  list: () => request.get('/whitelist/list'),
  create: (data) => request.post('/whitelist/create', data),
  update: (id, data) => request.put(`/whitelist/${id}`, data),
  delete: (id, params) => request.delete(`/whitelist/${id}`, { params })
}

export const settingsAPI = {
  get: () => request.get('/settings'),
  update: (data) => request.put('/settings', data)
}

export const statusAPI = {
  get: () => request.get('/status')
}
