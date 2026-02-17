<template>
  <div class="user-management">
    <div class="toolbar">
      <h2>B站账号管理</h2>
      <div class="actions">
        <el-button type="primary" @click="showLoginDialog = true">添加账号</el-button>
        <el-button @click="loadUsers">刷新</el-button>
      </div>
    </div>

    <el-table :data="users" style="width: 100%" v-loading="loading">
      <el-table-column prop="uid" label="UID" width="120" />
      <el-table-column label="头像" width="80">
        <template #default="{ row }">
          <el-avatar :src="row.face" size="small" />
        </template>
      </el-table-column>
      <el-table-column prop="uname" label="用户名" />
      <el-table-column label="登录状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.login ? 'success' : 'danger'">
            {{ row.login ? '已登录' : '未登录' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="level" label="等级" width="80" />
      <el-table-column label="登录时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.login_time) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 登录对话框 -->
    <el-dialog v-model="showLoginDialog" title="添加B站账号" width="500px">
      <el-tabs v-model="loginTab">
        <el-tab-pane label="扫码登录" name="qrcode">
          <div class="login-qrcode">
            <div v-if="qrcodeLoading" class="qrcode-placeholder">
              <div class="loading-spinner"></div>
              <p>正在生成二维码...</p>
            </div>
            <div v-else-if="!qrcodeUrl" class="qrcode-placeholder">
              <el-button type="primary" @click="generateQRCode">
                重新生成二维码
              </el-button>
            </div>
            <div v-else class="qrcode-content">
              <div class="qrcode-image">
                <img :src="'data:image/png;base64,' + qrcodeUrl" alt="登录二维码" />
              </div>
              <p class="login-status">{{ loginStatus }}</p>
              <el-button @click="cancelLogin" size="small">取消</el-button>
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Cookie登录" name="cookie">
          <el-form label-width="0">
            <el-form-item>
              <el-input
                v-model="cookieInput"
                type="textarea"
                :rows="6"
                placeholder="请粘贴完整的Cookie"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="cookieLoginLoading" @click="handleCookieLogin" style="width: 100%">
                登录
              </el-button>
            </el-form-item>
          </el-form>
          <div class="cookie-tips">
            <p>💡 Cookie获取方法：</p>
            <ol>
              <li>登录 bilibili.com</li>
              <li>按F12打开开发者工具</li>
              <li>在Network标签页找到任意请求的Cookie</li>
              <li>复制完整的Cookie内容粘贴到上方</li>
            </ol>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userAPI } from '@/api'

const users = ref([])
const loading = ref(false)
const showLoginDialog = ref(false)
const loginTab = ref('qrcode')

// 扫码登录
const qrcodeUrl = ref('')
const qrcodeLoading = ref(false)
const loginStatus = ref('等待扫码...')
let authKey = ''
let pollingTimer = null

// Cookie登录
const cookieInput = ref('')
const cookieLoginLoading = ref(false)

// 监听对话框打开，自动生成二维码
watch(showLoginDialog, (newVal) => {
  if (newVal && loginTab.value === 'qrcode' && !qrcodeUrl.value) {
    generateQRCode()
  } else if (!newVal) {
    // 对话框关闭，清理状态
    cancelLogin()
  }
})

// 监听标签切换
watch(loginTab, (newVal) => {
  if (newVal === 'qrcode' && showLoginDialog.value && !qrcodeUrl.value) {
    generateQRCode()
  }
})

const loadUsers = async () => {
  loading.value = true
  try {
    const data = await userAPI.list()
    users.value = data
  } catch (error) {
    ElMessage.error('加载用户列表失败')
  } finally {
    loading.value = false
  }
}

const generateQRCode = async () => {
  qrcodeLoading.value = true
  loginStatus.value = '等待扫码...'
  qrcodeUrl.value = ''
  
  try {
    const data = await userAPI.login()
    
    if (data.error) {
      ElMessage.error(data.error)
      loginStatus.value = data.error
      return
    }
    
    if (!data.image || !data.key) {
      ElMessage.error('二维码数据不完整')
      loginStatus.value = '二维码数据不完整'
      return
    }
    
    authKey = data.key
    qrcodeUrl.value = data.image
    
    startPolling()
  } catch (error) {
    console.error('生成二维码失败:', error)
    ElMessage.error('生成二维码失败')
    loginStatus.value = '生成二维码失败'
  } finally {
    qrcodeLoading.value = false
  }
}

const startPolling = () => {
  if (pollingTimer) return
  
  pollingTimer = setInterval(async () => {
    try {
      const result = await userAPI.loginCheck(authKey)
      
      if (result.status === 'success') {
        loginStatus.value = '登录成功！'
        ElMessage.success('登录成功')
        stopPolling()
        showLoginDialog.value = false
        qrcodeUrl.value = ''
        await loadUsers()
      } else if (result.status === 'expired') {
        loginStatus.value = '二维码已过期'
        ElMessage.warning('二维码已过期，请重新生成')
        stopPolling()
        qrcodeUrl.value = ''
      } else if (result.status === 'failed') {
        loginStatus.value = result.message || '登录失败'
        ElMessage.error(result.message || '登录失败')
        stopPolling()
        qrcodeUrl.value = ''
      } else if (result.status === 'scanned') {
        loginStatus.value = '已扫码，等待确认...'
      } else {
        loginStatus.value = result.message || '等待扫码...'
      }
    } catch (error) {
      console.error('轮询失败:', error)
    }
  }, 2000)
}

const stopPolling = () => {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

const cancelLogin = async () => {
  if (authKey) {
    await userAPI.loginCancel(authKey)
  }
  stopPolling()
  qrcodeUrl.value = ''
  authKey = ''
}

const handleCookieLogin = async () => {
  const cookies = cookieInput.value.trim()
  if (!cookies) {
    ElMessage.warning('请输入Cookie')
    return
  }

  cookieLoginLoading.value = true
  try {
    const result = await userAPI.loginByCookie(cookies)
    if (result.type === 'success') {
      ElMessage.success('登录成功')
      showLoginDialog.value = false
      cookieInput.value = ''
      await loadUsers()
    } else {
      ElMessage.error(result.msg || '登录失败')
    }
  } catch (error) {
    console.error('Cookie登录失败:', error)
    ElMessage.error('登录失败，请检查Cookie是否正确')
  } finally {
    cookieLoginLoading.value = false
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除账号 ${row.uname} 吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await userAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadUsers()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN')
}

onUnmounted(() => {
  stopPolling()
})

// 初始加载
loadUsers()
</script>

<style scoped>
.user-management {
  padding: 20px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.toolbar h2 {
  margin: 0;
  font-size: 18px;
}

.actions {
  display: flex;
  gap: 10px;
}

.login-qrcode {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  min-height: 300px;
}

.qrcode-placeholder {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #409eff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 10px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.qrcode-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 15px;
}

.qrcode-image {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.qrcode-image img {
  width: 256px;
  height: 256px;
  display: block;
  border-radius: 4px;
}

.login-status {
  margin: 0;
  color: #606266;
  font-size: 14px;
}

.cookie-tips {
  margin-top: 15px;
  padding: 10px;
  background: #f4f4f5;
  border-radius: 4px;
  font-size: 12px;
  color: #606266;
}

.cookie-tips p {
  margin: 0 0 5px 0;
  font-weight: 500;
}

.cookie-tips ol {
  margin: 0;
  padding-left: 20px;
}

.cookie-tips li {
  margin: 3px 0;
}
</style>
