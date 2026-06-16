<template>
  <div class="whitelist-management">
    <div class="toolbar">
      <h2>白名单</h2>
      <div class="actions">
        <el-button type="primary" @click="openCreate">新增白名单</el-button>
        <el-button @click="loadUsers">刷新</el-button>
      </div>
    </div>

    <el-table :data="users" style="width: 100%" v-loading="loading" :empty-text="loading ? '加载中' : '暂无白名单用户'">
      <el-table-column prop="uid" label="UID" width="140" />
      <el-table-column prop="uname" label="用户名" min-width="160" />
      <el-table-column prop="remark" label="备注" min-width="220" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingUser ? '编辑白名单' : '新增白名单'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="UID">
          <el-input v-model="form.uid" placeholder="可选，优先按UID匹配" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.uname" placeholder="可选，按用户名精确匹配" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
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
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { whitelistAPI } from '@/api'
import { buildDeleteConfirmation } from '@/utils/deleteConfirm'

const users = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editingUser = ref(null)
const submitting = ref(false)
const form = ref(defaultForm())

function defaultForm() {
  return {
    uid: '',
    uname: '',
    remark: '',
    enabled: true
  }
}

const loadUsers = async () => {
  loading.value = true
  try {
    users.value = await whitelistAPI.list()
  } catch (error) {
    ElMessage.error('加载白名单失败')
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editingUser.value = null
  form.value = defaultForm()
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingUser.value = row
  form.value = { ...row }
  dialogVisible.value = true
}

const payload = () => ({
  uid: Number(form.value.uid) || 0,
  uname: form.value.uname.trim(),
  remark: form.value.remark.trim(),
  enabled: form.value.enabled
})

const handleSubmit = async () => {
  const data = payload()
  if (!data.uid && !data.uname) {
    ElMessage.warning('UID 和用户名至少填写一个')
    return
  }
  submitting.value = true
  try {
    if (editingUser.value) {
      await whitelistAPI.update(editingUser.value.id, data)
    } else {
      await whitelistAPI.create(data)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadUsers()
  } catch (error) {
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row) => {
  try {
    const params = await buildDeleteConfirmation(row, '白名单', row.uname || String(row.uid || row.id))
    await whitelistAPI.delete(row.id, params)
    ElMessage.success('删除成功')
    await loadUsers()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

onMounted(loadUsers)
</script>

<style scoped>
.whitelist-management {
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
</style>
