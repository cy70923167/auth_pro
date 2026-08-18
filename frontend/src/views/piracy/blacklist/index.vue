<template>
  <div class="blacklist-manage">
    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="域名/IP">
          <el-input v-model="searchForm.keyword" placeholder="请输入" clearable style="width: 180px" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="searchForm.type" placeholder="全部" clearable style="width: 120px">
            <el-option label="域名" value="domain" />
            <el-option label="IP" value="ip" />
            <el-option label="IP段" value="cidr" />
          </el-select>
        </el-form-item>
        <el-form-item label="来源">
          <el-select v-model="searchForm.source" placeholder="全部" clearable style="width: 130px">
            <el-option label="盗版入库" value="piracy" />
            <el-option label="手动添加" value="manual" />
            <el-option label="自动规则" value="auto" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">黑名单列表（共 {{ pagination.total }} 条）</span>
          <div class="table-actions">
            <el-button type="danger" plain :disabled="!selectedRows.length" @click="handleBatchRemove">
              批量移除 ({{ selectedRows.length }})
            </el-button>
            <el-button type="primary" @click="handleAdd">添加黑名单</el-button>
          </div>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading" @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="45" />
        <el-table-column prop="value" label="域名/IP" min-width="200" show-overflow-tooltip />
        <el-table-column prop="typeLabel" label="类型" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small">{{ row.typeLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sourceLabel" label="来源" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="sourceTagMap[row.source]" size="small" effect="plain">{{ row.sourceLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="hitCount" label="命中次数" width="100" align="center">
          <template #default="{ row }">
            <span class="hit-count">{{ row.hitCount }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="150" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="添加时间" width="160" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="handleRemove(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 添加/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="500px" destroy-on-close>
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="80px">
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="formData.type">
            <el-radio value="domain">域名</el-radio>
            <el-radio value="ip">IP</el-radio>
            <el-radio value="cidr">IP段</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="valueLabel" prop="value">
          <el-input v-model="formData.value" :placeholder="valuePlaceholder" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.remark" type="textarea" :rows="2" placeholder="可选备注" />
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
const selectedRows = ref<any[]>([])

const dialogTitle = computed(() => (isEdit.value ? '编辑黑名单' : '添加黑名单'))

const searchForm = reactive({ keyword: '', type: '', source: '' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })

const typeTagMap: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined> = { domain: undefined, ip: 'warning', cidr: 'info' } as const
const sourceTagMap: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined> = { piracy: 'danger', manual: undefined, auto: 'warning' } as const

const tableData = ref<any[]>([])

const formRef = ref()
const formData = reactive({ id: 0, type: 'domain', value: '', remark: '' })
const formRules = {
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  value: [{ required: true, message: '请输入值', trigger: 'blur' }]
}

const valueLabel = computed(() => {
  const map: Record<string, string> = { domain: '域名', ip: 'IP地址', cidr: 'IP段' }
  return map[formData.type] || '值'
})

const valuePlaceholder = computed(() => {
  const map: Record<string, string> = { domain: 'example.com', ip: '192.168.1.1', cidr: '10.0.0.0/24' }
  return map[formData.type] || ''
})

async function handleSearch() {
  loading.value = true
  try {
    const params: any = { page: pagination.page, pageSize: pagination.pageSize }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.type) params.type = searchForm.type
    if (searchForm.source) params.source = searchForm.source
    const data = await request.get<any>({ url: '/api/piracy/blacklist/list', params })
    tableData.value = data.list || []
    pagination.total = data.total || 0
  } finally {
    loading.value = false
  }
}

function handleReset() {
  searchForm.keyword = ''; searchForm.type = ''; searchForm.source = ''
  pagination.page = 1
  handleSearch()
}

function handleSelectionChange(rows: any[]) { selectedRows.value = rows }

function handleAdd() {
  isEdit.value = false
  Object.assign(formData, { id: 0, type: 'domain', value: '', remark: '' })
  dialogVisible.value = true
}

function handleEdit(row: any) {
  isEdit.value = true
  Object.assign(formData, { id: row.id, type: row.type, value: row.value, remark: row.remark })
  dialogVisible.value = true
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isEdit.value) {
    await request.put<any>({
      url: `/api/piracy/blacklist/${formData.id}`,
      data: { type: formData.type, value: formData.value, remark: formData.remark },
      showSuccessMessage: true
    })
  } else {
    await request.post<any>({
      url: '/api/piracy/blacklist/create',
      data: { type: formData.type, value: formData.value, remark: formData.remark },
      showSuccessMessage: true
    })
  }
  dialogVisible.value = false
  handleSearch()
}

function handleRemove(row: any) {
  ElMessageBox.confirm(`确定从黑名单移除「${row.value}」？移除后该域名/IP的请求将不再自动阻断`, '移除确认', { type: 'warning' }).then(async () => {
    await request.del<any>({ url: `/api/piracy/blacklist/${row.id}` })
    ElMessage.success('已移除')
    handleSearch()
  })
}

function handleBatchRemove() {
  ElMessageBox.confirm(`确定移除选中的 ${selectedRows.value.length} 条黑名单？`, '批量移除', { type: 'warning' }).then(async () => {
    const ids = selectedRows.value.map((r) => r.id)
    await request.post<any>({ url: '/api/piracy/blacklist/batch-delete', data: { ids } })
    ElMessage.success(`已移除 ${ids.length} 条`)
    handleSearch()
  })
}

onMounted(() => {
  handleSearch()
})
</script>

<style scoped lang="scss">
.mb-4 { margin-bottom: 16px; }
.table-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.table-actions { display: flex; gap: 8px; }
.card-title { font-weight: 600; }
.pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
.hit-count { font-weight: 600; color: #f56c6c; }
</style>