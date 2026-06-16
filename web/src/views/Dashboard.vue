<template>
  <div class="dashboard">
    <el-container>
      <el-header class="header">
        <div class="header-content">
          <h1>Goban 评论监控系统</h1>
          <el-button type="danger" @click="handleLogout">退出登录</el-button>
        </div>
      </el-header>

      <el-container>
        <el-aside width="200px" class="sidebar">
          <el-menu :default-active="activeMenu" @select="handleMenuSelect">
            <el-menu-item index="status">
              <span>监控状态</span>
            </el-menu-item>
            <el-menu-item index="users">
              <span>B站账号</span>
            </el-menu-item>
            <el-menu-item index="keywords">
              <span>关键字规则</span>
            </el-menu-item>
            <el-menu-item index="whitelist">
              <span>白名单</span>
            </el-menu-item>
            <el-menu-item index="tasks">
              <span>监控任务</span>
            </el-menu-item>
            <el-menu-item index="logs">
              <span>监控日志</span>
            </el-menu-item>
            <el-menu-item index="reports">
              <span>举报记录</span>
            </el-menu-item>
            <el-menu-item index="settings">
              <span>系统配置</span>
            </el-menu-item>
          </el-menu>
        </el-aside>

        <el-main class="main">
          <component :is="currentComponent" />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import UserManagement from '@/components/UserManagement.vue'
import TaskManagement from '@/components/TaskManagement.vue'
import LogManagement from '@/components/LogManagement.vue'
import ReportManagement from '@/components/ReportManagement.vue'
import KeywordManagement from '@/components/KeywordManagement.vue'
import WhitelistManagement from '@/components/WhitelistManagement.vue'
import ConfigManagement from '@/components/ConfigManagement.vue'
import StatusOverview from '@/components/StatusOverview.vue'
import { clearCredentials } from '@/utils/authStorage'

const router = useRouter()
const activeMenu = ref('status')

const components = {
  status: StatusOverview,
  users: UserManagement,
  keywords: KeywordManagement,
  whitelist: WhitelistManagement,
  tasks: TaskManagement,
  logs: LogManagement,
  reports: ReportManagement,
  settings: ConfigManagement
}

const currentComponent = computed(() => components[activeMenu.value])

const handleMenuSelect = (index) => {
  activeMenu.value = index
}

const handleLogout = async () => {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    clearCredentials()
    router.push('/login')
  } catch {
    // 取消
  }
}
</script>

<style scoped>
.dashboard {
  height: 100vh;
}

.header {
  background: #409eff;
  color: white;
  display: flex;
  align-items: center;
  padding: 0 20px;
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 500;
}

.sidebar {
  background: #f5f7fa;
  border-right: 1px solid #e4e7ed;
}

.main {
  background: white;
  padding: 20px;
}

:deep(.el-menu) {
  border: none;
}
</style>
