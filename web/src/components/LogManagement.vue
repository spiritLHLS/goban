<template>
  <div class="log-management">
    <div class="toolbar">
      <h2>监控日志</h2>
      <div class="actions">
        <el-button @click="loadLogs">刷新</el-button>
      </div>
    </div>

    <el-table :data="logs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="任务" width="200">
        <template #default="{ row }">
          <div v-if="row.task">
            <div>{{ row.task.target_uname }}</div>
            <div style="font-size: 12px; color: #909399;">
              使用: {{ row.task.user?.uname }}
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="等级" width="80">
        <template #default="{ row }">
          <el-tag
            :type="row.level === 'error' ? 'danger' : row.level === 'warning' ? 'warning' : 'info'"
            size="small"
          >
            {{ row.level }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="消息" />
      <el-table-column label="时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @size-change="loadLogs"
        @current-change="loadLogs"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { logAPI } from '@/api'

const logs = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)

const loadLogs = async () => {
  loading.value = true
  try {
    const data = await logAPI.monitor({
      page: page.value,
      page_size: pageSize.value
    })
    
    logs.value = data.data || []
    total.value = data.total || 0
  } catch (error) {
    ElMessage.error('加载日志失败')
  } finally {
    loading.value = false
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN')
}

onMounted(() => {
  loadLogs()
})
</script>

<style scoped>
.log-management {
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
