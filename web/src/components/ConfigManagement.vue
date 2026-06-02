<template>
  <div class="config-management">
    <div class="toolbar">
      <h2>系统配置</h2>
      <div class="actions">
        <el-button @click="loadSettings">刷新</el-button>
        <el-button type="primary" :loading="saving" @click="saveSettings">保存</el-button>
      </div>
    </div>

    <el-alert
      v-if="runtime"
      type="info"
      :closable="false"
      class="runtime"
      :title="`运行端口 ${runtime.port}，数据库 ${runtime.db_path}，最大并发任务 ${runtime.max_concurrent_tasks}`"
    />

    <el-form :model="form" label-width="150px" class="settings-form" v-loading="loading">
      <el-divider content-position="left">默认监控参数</el-divider>
      <el-form-item label="默认视频数">
        <el-input-number v-model="form.default_video_count" :min="1" :max="50" />
      </el-form-item>
      <el-form-item label="默认评论数">
        <el-input-number v-model="form.default_comment_count" :min="1" :max="500" />
      </el-form-item>
      <el-form-item label="默认检查间隔">
        <el-input-number v-model="form.default_interval" :min="60" :max="86400" />
        <span class="unit">秒</span>
      </el-form-item>
      <el-form-item label="默认举报间隔">
        <el-input-number v-model="form.default_report_delay" :min="6" :max="600" />
        <span class="unit">秒</span>
      </el-form-item>
      <el-form-item label="默认最大重试">
        <el-input-number v-model="form.default_max_retries" :min="0" :max="10" />
      </el-form-item>
      <el-form-item label="默认重试间隔">
        <el-input-number v-model="form.default_retry_interval" :min="1" :max="60" />
        <span class="unit">秒</span>
      </el-form-item>
      <el-form-item label="Cookie检查间隔">
        <el-input-number v-model="form.cookie_check_interval" :min="60" :max="86400" />
        <span class="unit">秒</span>
      </el-form-item>

      <el-divider content-position="left">Webhook通知</el-divider>
      <el-form-item label="启用Webhook">
        <el-switch v-model="form.webhook_enabled" />
      </el-form-item>
      <el-form-item label="通知类型">
        <el-select v-model="form.webhook_type" style="width: 240px">
          <el-option label="不发送" value="none" />
          <el-option label="Telegram" value="telegram" />
          <el-option label="飞书" value="feishu" />
        </el-select>
      </el-form-item>
      <el-form-item label="Telegram Bot Token">
        <el-input v-model="form.telegram_bot_token" type="password" show-password />
      </el-form-item>
      <el-form-item label="Telegram Chat ID">
        <el-input v-model="form.telegram_chat_id" />
      </el-form-item>
      <el-form-item label="飞书Webhook URL">
        <el-input v-model="form.feishu_webhook_url" />
      </el-form-item>
      <el-form-item label="Webhook超时">
        <el-input-number v-model="form.webhook_timeout" :min="1" :max="60" />
        <span class="unit">秒</span>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { settingsAPI } from '@/api'

const loading = ref(false)
const saving = ref(false)
const runtime = ref(null)
const form = ref(defaultForm())

function defaultForm() {
  return {
    default_video_count: 5,
    default_comment_count: 50,
    default_interval: 300,
    default_report_delay: 6,
    default_max_retries: 3,
    default_retry_interval: 2,
    cookie_check_interval: 3600,
    webhook_enabled: false,
    webhook_type: 'none',
    telegram_bot_token: '',
    telegram_chat_id: '',
    feishu_webhook_url: '',
    webhook_timeout: 8
  }
}

const numericKeys = [
  'default_video_count',
  'default_comment_count',
  'default_interval',
  'default_report_delay',
  'default_max_retries',
  'default_retry_interval',
  'cookie_check_interval',
  'webhook_timeout'
]

const loadSettings = async () => {
  loading.value = true
  try {
    const data = await settingsAPI.get()
    runtime.value = data.runtime
    const settings = data.settings || {}
    const next = defaultForm()
    for (const key of Object.keys(next)) {
      if (settings[key] === undefined) continue
      if (numericKeys.includes(key)) {
        next[key] = Number(settings[key])
      } else if (key === 'webhook_enabled') {
        next[key] = settings[key] === 'true'
      } else {
        next[key] = settings[key]
      }
    }
    form.value = next
  } catch (error) {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    const payload = {}
    for (const [key, value] of Object.entries(form.value)) {
      payload[key] = String(value)
    }
    await settingsAPI.update(payload)
    ElMessage.success('保存成功')
    await loadSettings()
  } catch (error) {
    ElMessage.error('保存配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.config-management {
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

.runtime {
  margin-bottom: 16px;
}

.settings-form {
  max-width: 820px;
}

.unit {
  margin-left: 10px;
  color: #909399;
}
</style>
