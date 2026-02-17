<template>
  <div class="report-management">
    <div class="toolbar">
      <h2>举报记录</h2>
      <div class="actions">
        <el-button @click="loadReports">刷新</el-button>
      </div>
    </div>

    <el-table :data="reports" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="任务" width="150">
        <template #default="{ row }">
          <div v-if="row.task">
            {{ row.task.target_uname }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="视频" width="200">
        <template #default="{ row }">
          <div>{{ row.video_title }}</div>
          <div style="font-size: 12px; color: #909399;">{{ row.bvid }}</div>
        </template>
      </el-table-column>
      <el-table-column label="评论" width="250">
        <template #default="{ row }">
          <div style="font-size: 12px;">
            <div>用户: {{ row.comment_user }}</div>
            <div style="color: #606266; margin-top: 5px;">{{ truncate(row.comment_content, 50) }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="匹配关键字" width="120">
        <template #default="{ row }">
          <el-tag type="warning" size="small">{{ row.matched_keyword }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" size="small">
            {{ row.success ? '成功' : '失败' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="消息" width="150">
        <template #default="{ row }">
          {{ row.message }}
        </template>
      </el-table-column>
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
        @size-change="loadReports"
        @current-change="loadReports"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { logAPI } from '@/api'

const reports = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)

const loadReports = async () => {
  loading.value = true
  try {
    const data = await logAPI.report({
      page: page.value,
      page_size: pageSize.value
    })
    
    reports.value = data.data || []
    total.value = data.total || 0
  } catch (error) {
    ElMessage.error('加载举报记录失败')
  } finally {
    loading.value = false
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN')
}

const truncate = (str, len) => {
  if (!str) return ''
  if (str.length <= len) return str
  return str.substring(0, len) + '...'
}

onMounted(() => {
  loadReports()
})
</script>

<style scoped>
.report-management {
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
