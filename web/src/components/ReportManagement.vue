<template>
  <div class="report-management">
    <div class="toolbar">
      <h2>举报记录</h2>
      <div class="actions">
        <el-button @click="loadReports">刷新</el-button>
        <el-button type="primary" :loading="exporting" @click="exportReports">导出CSV</el-button>
      </div>
    </div>

    <el-form :inline="true" :model="filters" class="filters">
      <el-form-item label="任务">
        <el-select v-model="filters.task_id" clearable placeholder="全部任务" style="width: 180px">
          <el-option v-for="task in tasks" :key="task.id" :label="task.name || task.id" :value="task.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="UP主UID">
        <el-input v-model="filters.target_uid" clearable style="width: 150px" />
      </el-form-item>
      <el-form-item label="关键字">
        <el-input v-model="filters.keyword" clearable style="width: 160px" />
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="filters.success" clearable placeholder="全部" style="width: 110px">
          <el-option label="成功" value="true" />
          <el-option label="失败" value="false" />
        </el-select>
      </el-form-item>
      <el-form-item label="时间">
        <el-date-picker
          v-model="filters.time_range"
          type="datetimerange"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          style="width: 360px"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="applyFilters">筛选</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="reports" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="任务" width="150">
        <template #default="{ row }">{{ row.task?.name || row.task_id }}</template>
      </el-table-column>
      <el-table-column label="UP主" width="160">
        <template #default="{ row }">
          <div>{{ row.target_uname || '-' }}</div>
          <div class="muted">{{ row.target_uid || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="视频" min-width="220">
        <template #default="{ row }">
          <div>{{ row.video_title }}</div>
          <div class="muted">{{ row.bvid }}</div>
        </template>
      </el-table-column>
      <el-table-column label="评论" min-width="260">
        <template #default="{ row }">
          <div class="muted">用户: {{ row.comment_user }} ({{ row.comment_user_id || '-' }})</div>
          <div>{{ truncate(row.comment_content, 70) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="匹配规则" width="150">
        <template #default="{ row }">
          <el-tag type="warning" size="small">{{ row.keyword_rule_name || row.matched_keyword }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" size="small">
            {{ row.success ? '成功' : '失败' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="消息" width="160" show-overflow-tooltip />
      <el-table-column label="时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
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
import { logAPI, taskAPI } from '@/api'

const reports = ref([])
const tasks = ref([])
const loading = ref(false)
const exporting = ref(false)
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const filters = ref(defaultFilters())

function defaultFilters() {
  return {
    task_id: '',
    target_uid: '',
    keyword: '',
    success: '',
    time_range: []
  }
}

const queryParams = () => {
  const params = {
    page: page.value,
    page_size: pageSize.value
  }
  for (const key of ['task_id', 'target_uid', 'keyword', 'success']) {
    if (filters.value[key] !== '' && filters.value[key] !== null) {
      params[key] = filters.value[key]
    }
  }
  if (filters.value.time_range?.length === 2) {
    params.start_time = filters.value.time_range[0].toISOString()
    params.end_time = filters.value.time_range[1].toISOString()
  }
  return params
}

const loadReports = async () => {
  loading.value = true
  try {
    const data = await logAPI.report(queryParams())
    reports.value = data.data || []
    total.value = data.total || 0
  } catch (error) {
    ElMessage.error('加载举报记录失败')
  } finally {
    loading.value = false
  }
}

const loadTasks = async () => {
  try {
    tasks.value = await taskAPI.list()
  } catch (error) {
    tasks.value = []
  }
}

const applyFilters = () => {
  page.value = 1
  loadReports()
}

const resetFilters = () => {
  filters.value = defaultFilters()
  page.value = 1
  loadReports()
}

const exportReports = async () => {
  exporting.value = true
  try {
    const params = queryParams()
    delete params.page
    delete params.page_size
    const blob = await logAPI.exportReport(params)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'goban-report-records.csv'
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    ElMessage.error('导出失败')
  } finally {
    exporting.value = false
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const truncate = (str, len) => {
  if (!str) return ''
  if (str.length <= len) return str
  return str.substring(0, len) + '...'
}

onMounted(() => {
  loadTasks()
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
  margin-bottom: 16px;
}

.toolbar h2 {
  margin: 0;
  font-size: 18px;
}

.actions {
  display: flex;
  gap: 10px;
}

.filters {
  margin-bottom: 12px;
}

.muted {
  color: #909399;
  font-size: 12px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
