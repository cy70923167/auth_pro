<template>
  <div class="agent-quota">
    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="代理商">
          <el-select v-model="searchForm.agentId" placeholder="全部" clearable style="width: 150px">
            <el-option v-for="a in agentList" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="应用">
          <el-select v-model="searchForm.appId" placeholder="全部" clearable style="width: 140px">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 配额表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">开码配额</span>
          <el-button type="primary" @click="handleAdd">分配配额</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="agentName" label="代理商" min-width="120" />
        <el-table-column prop="appName" label="应用" width="120" />
        <el-table-column prop="totalQuota" label="总配额" width="100" align="center" />
        <el-table-column prop="usedQuota" label="已使用" width="100" align="center" />
        <el-table-column label="剩余" width="100" align="center">
          <template #default="{ row }">
            <span :class="row.totalQuota - row.usedQuota <= 5 ? 'text-danger' : ''">
              {{ row.totalQuota === -1 ? '无限' : row.totalQuota - row.usedQuota }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="使用率" width="160">
          <template #default="{ row }">
            <el-progress
              v-if="row.totalQuota !== -1"
              :percentage="Math.round((row.usedQuota / row.totalQuota) * 100)"
              :color="getProgressColor(row.usedQuota / row.totalQuota)"
              :stroke-width="8"
            />
            <el-tag v-else type="success" size="small">不限</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="price" label="单价(元)" width="100" align="right">
          <template #default="{ row }">
            ¥{{ row.price.toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column prop="updatedAt" label="更新时间" width="160" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleEdit(row)">调整</el-button>
            <el-button link type="danger" size="small" @click="handleDelete(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 分配/调整弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="480px" destroy-on-close>
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="90px">
        <el-form-item label="代理商" prop="agentId">
          <el-select
            v-model="formData.agentId"
            placeholder="请输入代理商名称搜索"
            filterable
            style="width: 100%"
            :disabled="isEdit"
          >
            <el-option v-for="a in agentList" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="应用" prop="appId">
          <el-select v-model="formData.appId" placeholder="请选择" style="width: 100%" :disabled="isEdit">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="总配额" prop="totalQuota">
          <el-input-number v-model="formData.totalQuota" :min="0" :max="999999" :step="10" style="width: 200px" />
        </el-form-item>
        <el-form-item label="单价(元)" prop="price">
          <el-input-number v-model="formData.price" :min="0" :max="999999" :step="10" :precision="2" style="width: 200px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/http'

const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const dialogTitle = computed(() => (isEdit.value ? '调整配额' : '分配配额'))

const searchForm = reactive({ agentId: '', appId: '' })
const pagination = reactive({ page: 1, pageSize: 10, total: 0 })

const agentList = ref<{ id: string; name: string }[]>([])
const appList = ref<{ id: string; name: string }[]>([])
const tableData = ref<any[]>([])

const formRef = ref()
const formData = reactive({ id: 0, agentId: '', appId: '', totalQuota: 0, price: 0.00 })
const formRules = {
  agentId: [{ required: true, message: '请选择代理商', trigger: 'change' }],
  appId: [{ required: true, message: '请选择应用', trigger: 'change' }],
  totalQuota: [{ required: true, message: '请输入配额', trigger: 'blur' }],
  price: [{ required: true, message: '请输入单价', trigger: 'blur' }]
}

function getProgressColor(ratio: number) {
  if (ratio >= 0.9) return '#f56c6c'
  if (ratio >= 0.7) return '#e6a23c'
  return '#67c23a'
}

async function fetchAgentList() {
  try {
    const data = await request.get<any[]>({ url: '/api/agent/select-list' })
    agentList.value = (data || []).map((a: any) => ({ id: String(a.id), name: a.name }))
  } catch {}
}

async function fetchAppList() {
  try {
    const data = await request.get<any[]>({ url: '/api/license/apps' })
    appList.value = (data || []).map((a: any) => ({ id: String(a.id), name: a.name }))
  } catch {}
}

async function handleSearch() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.page, pageSize: pagination.pageSize }
    if (searchForm.agentId) params.agentId = searchForm.agentId
    if (searchForm.appId) params.appId = searchForm.appId

    const data = await request.get<{ list: any[]; total: number }>({ url: '/api/quota/list', params })
    tableData.value = data.list || []
    pagination.total = data.total || 0
  } catch (e) {
    console.error('[Quota] 查询失败:', e)
  } finally {
    loading.value = false
  }
}

function handleReset() {
  searchForm.agentId = ''
  searchForm.appId = ''
  pagination.page = 1
  handleSearch()
}

function handleAdd() {
  isEdit.value = false
  Object.assign(formData, { id: 0, agentId: '', appId: '', totalQuota: 0, price: 0.00 })
  dialogVisible.value = true
}

function handleEdit(row: any) {
  isEdit.value = true
  Object.assign(formData, { id: row.id, agentId: String(row.agentId), appId: String(row.appId), totalQuota: row.totalQuota, price: row.price })
  dialogVisible.value = true
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`移除「${row.agentName}」的「${row.appName}」配额？`, '提示', { type: 'warning' })
    await request.del({ url: `/api/quota/${row.id}` })
    ElMessage.success('已移除')
    handleSearch()
  } catch {}
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  try {
    if (isEdit.value) {
      await request.put({
        url: `/api/quota/${formData.id}`,
        params: { totalQuota: formData.totalQuota, price: formData.price }
      })
      ElMessage.success('调整成功')
    } else {
      await request.post({
        url: '/api/quota/create',
        params: { agentId: Number(formData.agentId), appId: Number(formData.appId), totalQuota: formData.totalQuota, price: formData.price }
      })
      ElMessage.success('分配成功')
    }
    dialogVisible.value = false
    handleSearch()
  } catch (e) {
    console.error('[Quota] 提交失败:', e)
  }
}

onMounted(() => {
  fetchAgentList()
  fetchAppList()
  handleSearch()
})
</script>

<style scoped lang="scss">
.mb-4 { margin-bottom: 16px; }
.table-header { display: flex; align-items: center; justify-content: space-between; }
.card-title { font-weight: 600; }
.pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
.text-danger { color: #f56c6c; font-weight: 600; }
.hint-text { margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary); }
</style>