<template>
  <div class="task-management">
    <div class="toolbar">
      <h2>监控任务管理</h2>
      <div class="actions">
        <el-button type="primary" @click="openCreate">创建任务</el-button>
        <el-button @click="loadAll">刷新</el-button>
      </div>
    </div>

    <el-table :data="tasks" style="width: 100%" v-loading="loading" :empty-text="loading ? '加载中' : '暂无监控任务'">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="任务" min-width="150" />
      <el-table-column label="使用账号" width="140">
        <template #default="{ row }">{{ row.user?.uname || '-' }}</template>
      </el-table-column>
      <el-table-column label="监控UP主" min-width="180">
        <template #default="{ row }">
          <div class="target-list">
            <el-tag
              v-for="target in row.targets"
              :key="target.id"
              size="small"
              :type="statusType(target.last_status)"
              :title="target.last_error || target.last_status || 'created'"
            >
              {{ target.uname || target.uid }} · {{ target.last_status || 'created' }}
              <span v-if="target.checked_comments">({{ target.checked_comments }}/{{ target.matched_comments || 0 }}/{{ target.report_count || 0 }})</span>
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="规则" min-width="160">
        <template #default="{ row }">
          <span>{{ ruleSummary(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="统计" width="160">
        <template #default="{ row }">
          <div class="mini">检测 {{ row.checked_comments || 0 }}</div>
          <div class="mini">匹配 {{ row.matched_comments || 0 }} / 举报 {{ row.report_count || 0 }}</div>
        </template>
      </el-table-column>
      <el-table-column label="进度" min-width="240">
        <template #default="{ row }">
          <el-progress :percentage="progressPercent(row)" :status="progressStatus(row)" :stroke-width="8" />
          <div class="mini progress-message">{{ row.progress_message || row.last_error || '-' }}</div>
          <div class="mini">下次 {{ formatTime(row.next_run_at) }}</div>
          <div class="recent-log" v-for="log in recentLogs(row)" :key="log.id">
            <el-tag size="small" :type="statusType(log.level)">{{ log.level }}</el-tag>
            <span>{{ log.message }}</span>
            <span v-if="log.repeat_count > 1" class="muted">x{{ log.repeat_count }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="配置" width="210">
        <template #default="{ row }">
          <div class="mini">视频 {{ row.video_count }} | 评论 {{ row.comment_count }}</div>
          <div class="mini">检查 {{ row.interval }}秒 | 举报 {{ row.report_delay || 30 }}秒</div>
          <div class="mini">每日上限 {{ row.daily_report_limit || 100 }}</div>
          <div class="mini">重试 {{ row.max_retries }}次，间隔 {{ row.retry_interval }}秒</div>
          <el-tag v-if="row.proxy_url" size="small" type="success">代理</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? statusType(row.last_status) : 'info'" size="small">
            {{ row.enabled ? (row.last_status || 'created') : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后检查" width="180">
        <template #default="{ row }">{{ formatTime(row.last_check) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="handleTest(row)" :loading="testingId === row.id">测试</el-button>
          <el-button v-if="row.enabled" size="small" @click="handleStatus(row, 'disable')">暂停</el-button>
          <el-button v-else size="small" type="success" @click="handleStatus(row, 'enable')">启用</el-button>
          <el-button size="small" @click="handleStatus(row, 'retry_now')">重试</el-button>
          <el-button size="small" @click="handleStatus(row, 'reset_stats')">重置</el-button>
          <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingTask ? '编辑任务' : '创建任务'" width="720px">
      <el-form :model="form" label-width="120px">
        <el-form-item label="任务名称">
          <el-input v-model="form.name" placeholder="留空时自动使用UP主名称" />
        </el-form-item>
        <el-form-item label="使用账号" v-if="!editingTask">
          <el-select v-model="form.user_id" placeholder="请选择B站账号" style="width: 100%">
            <el-option
              v-for="user in users"
              :key="user.id"
              :label="`${user.uname} (${user.uid})`"
              :value="user.id"
              :disabled="!user.login"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="UP主UID">
          <el-input
            v-model="form.target_uids_text"
            type="textarea"
            :rows="3"
            placeholder="每行或用逗号填写一个UID"
          />
        </el-form-item>
        <el-form-item label="关键字规则">
          <el-select v-model="form.keyword_rule_ids" multiple clearable placeholder="留空时使用所有启用规则" style="width: 100%">
            <el-option
              v-for="rule in keywordRules"
              :key="rule.id"
              :label="`${rule.name} (${rule.match_type === 'regex' ? '正则' : '普通'} · ${matchLogicLabel(rule.match_logic)})`"
              :value="rule.id"
              :disabled="!rule.enabled"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="临时关键字">
          <el-input
            v-model="form.keywords"
            type="textarea"
            :rows="2"
            placeholder="可选，逗号或换行分隔；会与规则一起生效"
          />
        </el-form-item>
        <el-form-item label="监控视频数">
          <el-input-number v-model="form.video_count" :min="1" :max="50" />
        </el-form-item>
        <el-form-item label="评论数">
          <el-input-number v-model="form.comment_count" :min="1" :max="500" />
        </el-form-item>
        <el-form-item label="检查间隔">
          <el-input-number v-model="form.interval" :min="60" :max="86400" />
          <span class="unit">秒</span>
        </el-form-item>
        <el-form-item label="举报间隔">
          <el-input-number v-model="form.report_delay" :min="30" :max="600" />
          <span class="unit">秒</span>
        </el-form-item>
        <el-form-item label="每日举报上限">
          <el-input-number v-model="form.daily_report_limit" :min="1" :max="5000" />
        </el-form-item>
        <el-form-item label="最大重试">
          <el-input-number v-model="form.max_retries" :min="0" :max="10" />
        </el-form-item>
        <el-form-item label="重试间隔">
          <el-input-number v-model="form.retry_interval" :min="1" :max="60" />
          <span class="unit">秒</span>
        </el-form-item>
        <el-form-item label="代理地址">
          <el-input v-model="form.proxy_url" placeholder="http://proxy:port 或 socks5://proxy:port" />
        </el-form-item>
        <el-form-item label="启用" v-if="editingTask">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">
          {{ editingTask ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showTestResult" title="测试结果" width="780px">
      <div v-if="testResult">
        <el-alert v-if="testResult.compile_errors?.length" type="warning" :closable="false" class="test-alert">
          <template #title>{{ testResult.compile_errors.join('；') }}</template>
        </el-alert>
        <div v-for="(video, index) in testResult.result" :key="index" class="test-result-item">
          <h4>{{ video.target_uname }} - {{ video.title || video.error }}</h4>
          <p v-if="video.bvid">BVID: {{ video.bvid }} | 评论数: {{ video.comments }}</p>
          <div v-if="video.matches && video.matches.length > 0" class="match-list">
            <el-tag type="warning" size="small" v-for="(match, idx) in video.matches" :key="idx">
              {{ match.rule_name }}：{{ match.matched }}
            </el-tag>
          </div>
          <el-tag v-else-if="!video.error" type="success" size="small">未发现匹配评论</el-tag>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { keywordAPI, taskAPI, userAPI } from '@/api'
import { buildDeleteConfirmation } from '@/utils/deleteConfirm'

const tasks = ref([])
const taskProgress = ref(new Map())
const users = ref([])
const keywordRules = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const showTestResult = ref(false)
const editingTask = ref(null)
const submitting = ref(false)
const testingId = ref(null)
const testResult = ref(null)
const form = ref(defaultForm())
let refreshTimer = null

function defaultForm() {
  return {
    name: '',
    user_id: null,
    target_uids_text: '',
    video_count: 5,
    comment_count: 50,
    keywords: '',
    keyword_rule_ids: [],
    interval: 300,
    proxy_url: '',
    report_delay: 30,
    daily_report_limit: 100,
    max_retries: 3,
    retry_interval: 2,
    enabled: true
  }
}

const loadTasks = async () => {
  const data = await taskAPI.progress()
  taskProgress.value = new Map((data || []).map(item => [item.task.id, item]))
  tasks.value = (data || []).map(item => item.task)
}

const loadUsers = async () => {
  users.value = await userAPI.list()
}

const loadRules = async () => {
  keywordRules.value = await keywordAPI.list()
}

const loadAll = async () => {
  loading.value = true
  try {
    await Promise.all([loadTasks(), loadUsers(), loadRules()])
  } catch (error) {
    ElMessage.error('加载数据失败')
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editingTask.value = null
  form.value = defaultForm()
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingTask.value = row
  form.value = {
    name: row.name || '',
    target_uids_text: (row.targets || []).map(target => target.uid).join('\n'),
    video_count: row.video_count,
    comment_count: row.comment_count,
    keywords: row.keywords || '',
    keyword_rule_ids: parseRuleIDs(row.keyword_rule_ids),
    interval: row.interval,
    proxy_url: row.proxy_url || '',
    report_delay: row.report_delay || 30,
    daily_report_limit: row.daily_report_limit || 100,
    max_retries: row.max_retries ?? 3,
    retry_interval: row.retry_interval || 2,
    enabled: row.enabled
  }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  const targetUids = parseTargetUIDs(form.value.target_uids_text)
  if (!editingTask.value && !form.value.user_id) {
    ElMessage.warning('请选择B站账号')
    return
  }
  if (targetUids.length === 0) {
    ElMessage.warning('请填写至少一个UP主UID')
    return
  }

  const payload = {
    name: form.value.name,
    user_id: form.value.user_id,
    target_uids: targetUids,
    video_count: form.value.video_count,
    comment_count: form.value.comment_count,
    keywords: form.value.keywords,
    keyword_rule_ids: form.value.keyword_rule_ids,
    interval: form.value.interval,
    proxy_url: form.value.proxy_url,
    report_delay: form.value.report_delay,
    daily_report_limit: form.value.daily_report_limit,
    max_retries: form.value.max_retries,
    retry_interval: form.value.retry_interval,
    enabled: form.value.enabled
  }

  submitting.value = true
  try {
    if (editingTask.value) {
      await taskAPI.update(editingTask.value.id, payload)
    } else {
      await taskAPI.create(payload)
    }
    ElMessage.success(editingTask.value ? '更新成功' : '创建成功')
    dialogVisible.value = false
    await loadTasks()
  } catch (error) {
    ElMessage.error(editingTask.value ? '更新失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

const handleTest = async (row) => {
  testingId.value = row.id
  try {
    const result = await taskAPI.test(row.id)
    if (result.error) {
      ElMessage.error(result.error)
      return
    }
    testResult.value = result
    showTestResult.value = true
  } catch (error) {
    ElMessage.error('测试失败')
  } finally {
    testingId.value = null
  }
}

const handleStatus = async (row, action) => {
  try {
    await taskAPI.updateStatus(row.id, { action })
    ElMessage.success('状态已更新')
    await loadTasks()
  } catch (error) {
    ElMessage.error('状态更新失败')
  }
}

const handleDelete = async (row) => {
  try {
    const params = await buildDeleteConfirmation(row, '任务', row.name || String(row.id))
    await taskAPI.delete(row.id, params)
    ElMessage.success('删除成功')
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const parseTargetUIDs = (value) => {
  return value
    .split(/[\s,;，]+/)
    .map(item => Number(item.trim()))
    .filter(item => Number.isInteger(item) && item > 0)
    .filter((item, index, arr) => arr.indexOf(item) === index)
}

const parseRuleIDs = (value) => {
  if (!value) return []
  return String(value)
    .split(',')
    .map(item => Number(item.trim()))
    .filter(item => Number.isInteger(item) && item > 0)
}

const ruleSummary = (row) => {
  const ids = parseRuleIDs(row.keyword_rule_ids)
  if (ids.length === 0 && !row.keywords) return '所有启用规则'
  const names = ids
    .map(id => keywordRules.value.find(rule => rule.id === id)?.name)
    .filter(Boolean)
  if (row.keywords) names.push('临时关键字')
  return names.length ? names.join('、') : '所有启用规则'
}

const statusType = (value) => {
  if (value === 'success') return 'success'
  if (value === 'warning') return 'warning'
  if (value === 'error') return 'danger'
  if (value === 'running') return 'primary'
  if (value === 'paused') return 'info'
  if (value === 'backoff') return 'warning'
  return 'info'
}

const progressPercent = (row) => {
  const item = taskProgress.value.get(row.id)
  if (item?.progress_percent !== undefined) return item.progress_percent
  if (!row.progress_total) return row.last_status === 'success' ? 100 : 0
  return Math.min(100, Math.max(0, Math.floor((row.progress_done || 0) * 100 / row.progress_total)))
}

const progressStatus = (row) => {
  if (row.last_status === 'success') return 'success'
  if (row.last_status === 'error') return 'exception'
  if (row.last_status === 'warning' || row.last_status === 'backoff') return 'warning'
  return undefined
}

const recentLogs = (row) => taskProgress.value.get(row.id)?.recent_logs || []

const matchLogicLabel = (value) => {
  const labels = {
    single: '单条',
    or: '任一',
    and: '全部'
  }
  return labels[value] || '单条'
}

const formatTime = (time) => {
  if (!time || time.startsWith?.('0001-')) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

onMounted(() => {
  loadAll()
  refreshTimer = setInterval(loadTasks, 8000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.task-management {
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

.actions,
.match-list,
.target-list {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.target-list {
  gap: 4px;
}

.mini {
  font-size: 12px;
  color: #606266;
  line-height: 1.6;
}

.progress-message,
.recent-log {
  overflow-wrap: anywhere;
}

.recent-log {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 4px;
  font-size: 12px;
  color: #606266;
}

.muted {
  color: #909399;
}

.unit {
  margin-left: 10px;
  color: #909399;
}

.test-alert {
  margin-bottom: 12px;
}

.test-result-item {
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.test-result-item h4 {
  margin: 0 0 6px 0;
}

.test-result-item p {
  margin: 0 0 10px 0;
  color: #909399;
  font-size: 12px;
}
</style>
