<template>
  <div class="task-management">
    <div class="toolbar">
      <h2>监控任务管理</h2>
      <div class="actions">
        <el-button type="primary" @click="showCreateDialog = true">创建任务</el-button>
        <el-button @click="loadTasks">刷新</el-button>
      </div>
    </div>

    <el-table :data="tasks" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="使用账号" width="150">
        <template #default="{ row }">
          {{ row.user?.uname || '-' }}
        </template>
      </el-table-column>
      <el-table-column label="监控UP主" width="150">
        <template #default="{ row }">
          {{ row.target_uname }}
        </template>
      </el-table-column>
      <el-table-column prop="keywords" label="关键字" />
      <el-table-column label="配置" width="220">
        <template #default="{ row }">
          <div style="font-size: 12px;">
            <div>视频数: {{ row.video_count }} | 评论数: {{ row.comment_count }}</div>
            <div>检查间隔: {{ row.interval }}秒</div>
            <div>举报间隔: {{ row.report_delay || 6 }}秒</div>
            <div v-if="row.proxy_url">
              <el-tag size="small" type="success">代理</el-tag>
            </div>
            <div v-if="row.max_retries > 0">
              <el-tag size="small" type="info">重试×{{ row.max_retries }}</el-tag>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">
            {{ row.enabled ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后检查" width="180">
        <template #default="{ row }">
          {{ formatTime(row.last_check) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="handleEdit(row)">编辑</el-button>
          <el-button size="small" @click="handleTest(row)" :loading="testingId === row.id">测试</el-button>
          <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建/编辑任务对话框 -->
    <el-dialog v-model="showCreateDialog" :title="editingTask ? '编辑任务' : '创建任务'" width="600px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="使用账号" v-if="!editingTask">
          <el-select v-model="form.user_id" placeholder="请选择B站账号" style="width: 100%">
            <el-option
              v-for="user in users"
              :key="user.id"
              :label="user.uname"
              :value="user.id"
              :disabled="!user.login"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="UP主UID" v-if="!editingTask">
          <el-input v-model="form.target_uid" placeholder="输入要监控的UP主UID" />
        </el-form-item>
        <el-form-item label="监控视频数">
          <el-input-number v-model="form.video_count" :min="1" :max="20" />
          <span style="margin-left: 10px; color: #909399;">监控最新多少条视频</span>
        </el-form-item>
        <el-form-item label="评论数">
          <el-input-number v-model="form.comment_count" :min="10" :max="200" />
          <span style="margin-left: 10px; color: #909399;">每个视频检查多少条评论</span>
        </el-form-item>
        <el-form-item label="关键字">
          <el-input
            v-model="form.keywords"
            type="textarea"
            :rows="3"
            placeholder="输入关键字，多个关键字用逗号分隔"
          />
        </el-form-item>
        <el-form-item label="检查间隔">
          <el-input-number v-model="form.interval" :min="60" :max="3600" />
          <span style="margin-left: 10px; color: #909399;">秒</span>
        </el-form-item>
        <el-form-item label="代理地址">
          <el-input v-model="form.proxy_url" placeholder="http://proxy:port 或 socks5://proxy:port （可选）" />
          <div style="font-size: 12px; color: #909399; margin-top: 5px;">
            单IP举报限制：1分钟10条，建议配置代理
          </div>
        </el-form-item>
        <el-form-item label="举报间隔">
          <el-input-number v-model="form.report_delay" :min="6" :max="60" />
          <span style="margin-left: 10px; color: #909399;">秒（默认6秒，确保不超过1分钟10次限制）</span>
        </el-form-item>
        <el-form-item label="最大重试">
          <el-input-number v-model="form.max_retries" :min="0" :max="10" />
          <span style="margin-left: 10px; color: #909399;">API调用失败后的最大重试次数（默认3次）</span>
        </el-form-item>
        <el-form-item label="重试间隔">
          <el-input-number v-model="form.retry_interval" :min="1" :max="30" />
          <span style="margin-left: 10px; color: #909399;">秒（基础间隔，使用指数退避策略，默认2秒）</span>
        </el-form-item>
        <el-form-item label="启用" v-if="editingTask">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">
          {{ editingTask ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 测试结果对话框 -->
    <el-dialog v-model="showTestResult" title="测试结果" width="700px">
      <div v-if="testResult">
        <div v-for="(video, index) in testResult.result" :key="index" class="test-result-item">
          <h4>{{ video.title }}</h4>
          <p>BVID: {{ video.bvid }} | 评论数: {{ video.comments }}</p>
          <div v-if="video.matches && video.matches.length > 0">
            <el-tag type="warning" size="small" style="margin: 2px" v-for="(match, idx) in video.matches" :key="idx">
              {{ match }}
            </el-tag>
          </div>
          <div v-else>
            <el-tag type="success" size="small">未发现匹配关键字的评论</el-tag>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { taskAPI, userAPI } from '@/api'

const tasks = ref([])
const users = ref([])
const loading = ref(false)
const showCreateDialog = ref(false)
const showTestResult = ref(false)
const editingTask = ref(null)
const submitting = ref(false)
const testingId = ref(null)
const testResult = ref(null)

const form = ref({
  user_id: null,
  target_uid: '',
  video_count: 5,
  comment_count: 50,
  keywords: '',
  interval: 300,
  proxy_url: '',
  report_delay: 6,
  max_retries: 3,
  retry_interval: 2,
  enabled: true
})

const loadTasks = async () => {
  loading.value = true
  try {
    const data = await taskAPI.list()
    tasks.value = data
  } catch (error) {
    ElMessage.error('加载任务列表失败')
  } finally {
    loading.value = false
  }
}

const loadUsers = async () => {
  try {
    const data = await userAPI.list()
    users.value = data
  } catch (error) {
    ElMessage.error('加载用户列表失败')
  }
}

const handleEdit = (row) => {
  editingTask.value = row
  form.value = {
    video_count: row.video_count,
    comment_count: row.comment_count,
    keywords: row.keywords,
    interval: row.interval,
    proxy_url: row.proxy_url || '',
    report_delay: row.report_delay || 6,
    max_retries: row.max_retries || 3,
    retry_interval: row.retry_interval || 2,
    enabled: row.enabled
  }
  showCreateDialog.value = true
}

const handleSubmit = async () => {
  if (editingTask.value) {
    // 编辑
    submitting.value = true
    try {
      await taskAPI.update(editingTask.value.id, form.value)
      ElMessage.success('更新成功')
      showCreateDialog.value = false
      editingTask.value = null
      await loadTasks()
    } catch (error) {
      ElMessage.error('更新失败')
    } finally {
      submitting.value = false
    }
  } else {
    // 创建
    if (!form.value.user_id || !form.value.target_uid || !form.value.keywords) {
      ElMessage.warning('请填写所有必填项')
      return
    }

    submitting.value = true
    try {
      await taskAPI.create(form.value)
      ElMessage.success('创建成功')
      showCreateDialog.value = false
      form.value = {
        user_id: null,
        target_uid: '',
        video_count: 5,
        comment_count: 50,
        keywords: '',
        interval: 300
      }
      await loadTasks()
    } catch (error) {
      ElMessage.error('创建失败')
    } finally {
      submitting.value = false
    }
  }
}

const handleTest = async (row) => {
  testingId.value = row.id
  try {
    const result = await taskAPI.test(row.id)
    testResult.value = result
    showTestResult.value = true
  } catch (error) {
    ElMessage.error('测试失败')
  } finally {
    testingId.value = null
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除任务吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await taskAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadTasks()
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

onMounted(() => {
  loadTasks()
  loadUsers()
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

.test-result-item {
  margin-bottom: 20px;
  padding: 15px;
  background: #f5f7fa;
  border-radius: 4px;
}

.test-result-item h4 {
  margin: 0 0 5px 0;
}

.test-result-item p {
  margin: 0 0 10px 0;
  color: #909399;
  font-size: 12px;
}
</style>
