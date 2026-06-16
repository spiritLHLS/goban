import { ElMessageBox } from 'element-plus'

export async function buildDeleteConfirmation(row, label, expectedText) {
  const name = expectedText || row?.name || row?.uname || String(row?.uid || row?.id || '')
  await ElMessageBox.confirm(`确定删除${label} ${name || row?.id} 吗？`, '删除确认', {
    confirmButtonText: '继续',
    cancelButtonText: '取消',
    type: 'warning'
  })
  const result = await ElMessageBox.prompt(`请输入 ${name || 'DELETE'} 确认删除`, '二次确认', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    inputPattern: /.+/,
    inputErrorMessage: '请输入确认内容',
    type: 'warning'
  })
  return {
    confirm_id: row.id,
    confirm_text: result.value
  }
}
