<template>
  <div class="keyword-management">
    <div class="toolbar">
      <h2>关键字规则</h2>
      <div class="actions">
        <el-button type="primary" @click="openCreate">新增规则</el-button>
        <el-button @click="loadRules">刷新</el-button>
      </div>
    </div>

    <div class="preview-panel">
      <el-input
        v-model="previewText"
        type="textarea"
        :rows="3"
        placeholder="输入一段评论内容，启用规则会自动预览匹配结果"
      />
      <div class="preview-result">
        <el-tag v-for="match in previewMatches" :key="`${match.rule_id}-${match.matched}`" type="warning" size="small">
          {{ match.rule_name }}：{{ match.matched }}
        </el-tag>
        <el-tag v-if="previewText && previewMatches.length === 0" type="success" size="small">未匹配</el-tag>
      </div>
    </div>

    <el-table :data="rules" style="width: 100%" v-loading="loading" :empty-text="loading ? '加载中' : '暂无关键字规则'">
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column prop="pattern" label="匹配内容" min-width="220" show-overflow-tooltip />
      <el-table-column label="类型" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="row.match_type === 'regex' ? 'warning' : 'info'">
            {{ row.match_type === 'regex' ? '正则' : '普通' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="关系" width="90">
        <template #default="{ row }">{{ matchLogicLabel(row.match_logic) }}</template>
      </el-table-column>
      <el-table-column label="大小写" width="90">
        <template #default="{ row }">{{ row.case_sensitive ? '敏感' : '不敏感' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近命中" width="180">
        <template #default="{ row }">{{ formatTime(row.last_matched_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingRule ? '编辑规则' : '新增规则'" width="560px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="留空时使用匹配内容" />
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="form.match_type">
            <el-radio-button v-for="option in matchTypeOptions" :key="option.value" :label="option.value">
              {{ option.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="匹配内容">
          <el-input v-model="form.pattern" type="textarea" :rows="3" placeholder="普通关键词或正则表达式" />
        </el-form-item>
        <el-form-item label="条件关系">
          <el-radio-group v-model="form.match_logic">
            <el-radio-button v-for="option in matchLogicOptions" :key="option.value" :label="option.value">
              {{ option.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="大小写敏感">
          <el-switch v-model="form.case_sensitive" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { keywordAPI } from '@/api'
import { buildDeleteConfirmation } from '@/utils/deleteConfirm'

const rules = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editingRule = ref(null)
const submitting = ref(false)
const previewText = ref('')
const previewMatches = ref([])
let previewTimer = null

const matchTypeOptions = [
  { label: '普通', value: 'plain' },
  { label: '正则', value: 'regex' }
]

const matchLogicOptions = [
  { label: '单条', value: 'single' },
  { label: '任一', value: 'or' },
  { label: '全部', value: 'and' }
]

const form = ref(defaultForm())

function defaultForm() {
  return {
    name: '',
    pattern: '',
    match_type: 'plain',
    match_logic: 'single',
    case_sensitive: false,
    enabled: true,
    description: ''
  }
}

const loadRules = async () => {
  loading.value = true
  try {
    rules.value = await keywordAPI.list()
    schedulePreview()
  } catch (error) {
    ElMessage.error('加载关键字规则失败')
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editingRule.value = null
  form.value = defaultForm()
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingRule.value = row
  form.value = { ...row, match_logic: row.match_logic || 'single' }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!form.value.pattern.trim()) {
    ElMessage.warning('请填写匹配内容')
    return
  }
  submitting.value = true
  try {
    if (editingRule.value) {
      await keywordAPI.update(editingRule.value.id, form.value)
    } else {
      await keywordAPI.create(form.value)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadRules()
  } catch (error) {
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row) => {
  try {
    const params = await buildDeleteConfirmation(row, '规则', row.name || String(row.id))
    await keywordAPI.delete(row.id, params)
    ElMessage.success('删除成功')
    await loadRules()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const schedulePreview = () => {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(loadPreview, 250)
}

const loadPreview = async () => {
  if (!previewText.value.trim()) {
    previewMatches.value = []
    return
  }
  try {
    const data = await keywordAPI.preview(buildPreviewPayload())
    if (data.error) {
      ElMessage.error(data.error)
      return
    }
    previewMatches.value = data.matches || []
  } catch (error) {
    previewMatches.value = []
  }
}

const buildPreviewPayload = () => {
  if (dialogVisible.value && form.value.pattern.trim()) {
    return {
      text: previewText.value,
      name: form.value.name,
      pattern: form.value.pattern,
      match_type: form.value.match_type,
      match_logic: form.value.match_logic,
      case_sensitive: form.value.case_sensitive,
      use_enabled: false
    }
  }
  return {
    text: previewText.value,
    use_enabled: true
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const matchLogicLabel = (value) => {
  const option = matchLogicOptions.find(item => item.value === value)
  return option ? option.label : '单条'
}

watch([
  previewText,
  dialogVisible,
  () => form.value.pattern,
  () => form.value.match_type,
  () => form.value.match_logic,
  () => form.value.case_sensitive
], schedulePreview)

onMounted(loadRules)
</script>

<style scoped>
.keyword-management {
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

.preview-panel {
  margin-bottom: 16px;
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.preview-result {
  min-height: 28px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  align-items: center;
  margin-top: 10px;
}
</style>
