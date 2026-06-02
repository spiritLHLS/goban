<template>
  <div class="status-overview">
    <div class="toolbar">
      <h2>监控状态</h2>
      <div class="actions">
        <el-button @click="loadStatus">刷新</el-button>
      </div>
    </div>

    <el-row :gutter="12" class="stats">
      <el-col :span="6" v-for="item in statItems" :key="item.label">
        <el-card shadow="never" class="stat-card">
          <div class="stat-value">{{ item.value }}</div>
          <div class="stat-label">{{ item.label }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-table :data="recentTasks" style="width: 100%" v-loading="loading">
      <el-table-column prop="name" label="任务" min-width="160" />
      <el-table-column label="UP主" min-width="180">
        <template #default="{ row }">
          <el-tag v-for="target in row.targets" :key="target.id" size="small" style="margin: 2px">
            {{ target.uname || target.uid }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.last_status)" size="small">{{ row.last_status || 'created' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="checked_comments" label="已检测" width="100" />
      <el-table-column prop="matched_comments" label="已匹配" width="100" />
      <el-table-column prop="report_count" label="已举报" width="100" />
      <el-table-column label="最后检查" width="180">
        <template #default="{ row }">{{ formatTime(row.last_check) }}</template>
      </el-table-column>
      <el-table-column prop="last_error" label="最近异常" min-width="180" />
    </el-table>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { statusAPI } from '@/api'

const loading = ref(false)
const status = ref({})
let timer = null

const recentTasks = computed(() => status.value.recent_tasks || [])
const statItems = computed(() => [
  { label: '启用任务', value: `${status.value.enabled_tasks || 0}/${status.value.total_tasks || 0}` },
  { label: 'B站账号', value: `${status.value.total_users || 0}` },
  { label: '异常账号', value: `${status.value.invalid_users || 0}` },
  { label: '已检测评论', value: status.value.checked_comments || 0 },
  { label: '已匹配评论', value: status.value.matched_comments || 0 },
  { label: '成功举报', value: status.value.report_success || 0 },
  { label: '总举报数', value: status.value.report_count || 0 },
  { label: '更新时间', value: formatTime(status.value.now) }
])

const loadStatus = async () => {
  loading.value = true
  try {
    status.value = await statusAPI.get()
  } catch (error) {
    ElMessage.error('加载监控状态失败')
  } finally {
    loading.value = false
  }
}

const statusType = (value) => {
  if (value === 'success') return 'success'
  if (value === 'warning') return 'warning'
  if (value === 'error') return 'danger'
  if (value === 'running') return 'primary'
  return 'info'
}

const formatTime = (time) => {
  if (!time || time.startsWith?.('0001-')) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

onMounted(() => {
  loadStatus()
  timer = setInterval(loadStatus, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.status-overview {
  padding: 20px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.toolbar h2 {
  margin: 0;
  font-size: 18px;
}

.stats {
  margin-bottom: 16px;
}

.stat-card {
  border-radius: 6px;
}

.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  margin-top: 6px;
  color: #909399;
  font-size: 12px;
}
</style>
