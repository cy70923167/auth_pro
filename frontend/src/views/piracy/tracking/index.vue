<template>
  <div class="piracy-tracking">
    <!-- 统计概览 -->
    <el-row :gutter="16" class="mb-4">
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">盗版案例总数</div>
          <div class="stats-value text-danger">{{ stats.total }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">待处理</div>
          <div class="stats-value text-warning">{{ stats.pending }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">已拉黑</div>
          <div class="stats-value text-primary">{{ stats.blocked }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">今日新增</div>
          <div class="stats-value">{{ stats.todayNew }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="域名/IP">
          <el-input v-model="searchForm.keyword" placeholder="请输入" clearable style="width: 180px" />
        </el-form-item>
        <el-form-item label="应用">
          <el-select v-model="searchForm.appId" placeholder="全部" clearable style="width: 130px">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 120px">
            <el-option label="发现" value="discovered" />
            <el-option label="已拉黑" value="blocked" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间">
          <el-date-picker
            v-model="searchForm.dateRange"
            type="daterange"
            start-placeholder="开始"
            end-placeholder="结束"
            style="width: 240px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 操作栏 + 表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">盗版案例</span>
          <div class="table-actions">
            <el-button type="warning" plain :disabled="!selectedRows.length" @click="handleBatchBlock">
              批量拉黑 ({{ selectedRows.length }})
            </el-button>
            <el-button plain :disabled="!selectedRows.length" @click="handleExportEvidence">
              导出证据
            </el-button>
            <el-button type="primary" @click="handleAdd">手动入库</el-button>
          </div>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading" @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="45" />
        <el-table-column prop="domain" label="盗版域名/IP" min-width="160" show-overflow-tooltip />
        <el-table-column prop="appName" label="被盗应用" min-width="100" />
        <el-table-column prop="hitCount" label="累计请求" width="90" align="center">
          <template #default="{ row }">
            <span class="hit-count">{{ row.hitCount }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="statusLabel" label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagMap[row.status]" size="small">{{ row.statusLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="region" label="地区" min-width="90" />
        <el-table-column prop="firstSeenAt" label="首次发现" min-width="140" />
        <el-table-column prop="lastSeenAt" label="最近活跃" min-width="140" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleDetail(row)">详情</el-button>
            <el-button link type="danger" size="small" @click="handleBlock(row)" v-if="row.status !== 'blocked'">
              拉黑
            </el-button>
            <el-button link type="success" size="small" @click="handleUnblock(row)" v-if="row.status === 'blocked'">
              解黑
            </el-button>
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

    <!-- 手动入库弹窗 -->
    <el-dialog v-model="addDialogVisible" title="手动入库盗版记录" width="520px" destroy-on-close>
      <el-form :model="addForm" :rules="addFormRules" ref="addFormRef" label-width="90px">
        <el-form-item label="盗版域名" prop="domain">
          <el-input v-model="addForm.domain" placeholder="例: pirate-site.com" />
        </el-form-item>
        <el-form-item label="被盗应用" prop="appId">
          <el-select v-model="addForm.appId" placeholder="请选择" style="width: 100%">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="来源IP">
          <el-input v-model="addForm.sourceIp" placeholder="可选" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="addForm.remark" type="textarea" :rows="2" placeholder="补充说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleAddSubmit">确认入库</el-button>
      </template>
    </el-dialog>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailDialogVisible" title="盗版案例详情" width="640px" destroy-on-close>
      <template v-if="currentDetail">
        <el-descriptions :column="2" border class="mb-4">
          <el-descriptions-item label="盗版域名">{{ currentDetail.domain }}</el-descriptions-item>
          <el-descriptions-item label="被盗应用">{{ currentDetail.appName }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="statusTagMap[currentDetail.status]" size="small">{{ currentDetail.statusLabel }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="累计请求">{{ currentDetail.hitCount }} 次</el-descriptions-item>
          <el-descriptions-item label="首次发现">{{ currentDetail.firstSeenAt }}</el-descriptions-item>
          <el-descriptions-item label="最近活跃">{{ currentDetail.lastSeenAt }}</el-descriptions-item>
          <el-descriptions-item label="地区">{{ currentDetail.region }}</el-descriptions-item>
          <el-descriptions-item label="来源IP">{{ currentDetail.sourceIps }}</el-descriptions-item>
        </el-descriptions>

        <!-- 时间线 -->
        <div class="detail-section-title">处理记录</div>
        <el-timeline>
          <el-timeline-item
            v-for="log in currentDetail.timeline"
            :key="log.id"
            :timestamp="log.time"
            :type="log.type"
            placement="top"
          >
            {{ log.content }}
          </el-timeline-item>
        </el-timeline>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/http'

const loading = ref(false)
const addDialogVisible = ref(false)
const detailDialogVisible = ref(false)
const selectedRows = ref<any[]>([])

const stats = reactive({ total: 0, pending: 0, blocked: 0, todayNew: 0 })

const searchForm = reactive({ keyword: '', appId: '', status: '', dateRange: [] as any[] })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })

const appList = ref<any[]>([])

const statusTagMap: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined> = {
  discovered: 'warning',
  blocked: 'danger'
} as const

const tableData = ref<any[]>([])
const currentDetail = ref<any>(null)

const addFormRef = ref()
const addForm = reactive({ domain: '', appId: '', sourceIp: '', remark: '' })
const addFormRules = {
  domain: [{ required: true, message: '请输入盗版域名', trigger: 'blur' }],
  appId: [{ required: true, message: '请选择被盗应用', trigger: 'change' }]
}

async function loadStats() {
  const data = await request.get<any>({ url: '/api/piracy/tracking/stats' })
  Object.assign(stats, data)
}

async function loadAppList() {
  const data = await request.get<any>({ url: '/api/license/apps' })
  appList.value = (data || []).map((a: any) => ({ id: String(a.id), name: a.name }))
}

async function handleSearch() {
  loading.value = true
  try {
    const params: any = { page: pagination.page, pageSize: pagination.pageSize }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.appId) params.appId = searchForm.appId
    if (searchForm.status) params.status = searchForm.status
    if (searchForm.dateRange?.length === 2) {
      params.startDate = searchForm.dateRange[0]
      params.endDate = searchForm.dateRange[1]
    }
    const data = await request.get<any>({ url: '/api/piracy/tracking/list', params })
    tableData.value = data.list || []
    pagination.total = data.total || 0
  } finally {
    loading.value = false
  }
}

function handleReset() {
  searchForm.keyword = ''; searchForm.appId = ''; searchForm.status = ''; searchForm.dateRange = []
  pagination.page = 1
  handleSearch()
}

function handleSelectionChange(rows: any[]) { selectedRows.value = rows }

function handleAdd() {
  Object.assign(addForm, { domain: '', appId: '', sourceIp: '', remark: '' })
  addDialogVisible.value = true
}

async function handleAddSubmit() {
  const valid = await addFormRef.value?.validate().catch(() => false)
  if (!valid) return
  await request.post<any>({
    url: '/api/piracy/tracking/create',
    data: { domain: addForm.domain, appId: Number(addForm.appId), sourceIp: addForm.sourceIp, remark: addForm.remark },
    showSuccessMessage: true
  })
  addDialogVisible.value = false
  handleSearch()
  loadStats()
}

async function handleDetail(row: any) {
  const data = await request.get<any>({ url: `/api/piracy/tracking/${row.id}` })
  currentDetail.value = data
  detailDialogVisible.value = true
}

function handleBlock(row: any) {
  ElMessageBox.confirm(`确定将「${row.domain}」加入黑名单？将立即阻断所有来自该域名的请求`, '拉黑确认', { type: 'warning' }).then(async () => {
    await request.put<any>({ url: `/api/piracy/tracking/${row.id}/block` })
    ElMessage.success(`已拉黑 ${row.domain}`)
    handleSearch()
    loadStats()
  })
}

function handleUnblock(row: any) {
  ElMessageBox.confirm(`确定将「${row.domain}」从黑名单移除？`, '解黑确认', { type: 'info' }).then(async () => {
    await request.put<any>({ url: `/api/piracy/tracking/${row.id}/unblock` })
    ElMessage.success(`已解黑 ${row.domain}`)
    handleSearch()
    loadStats()
  })
}

function handleBatchBlock() {
  ElMessageBox.confirm(`确定批量拉黑选中的 ${selectedRows.value.length} 条记录？`, '批量拉黑', { type: 'warning' }).then(async () => {
    const ids = selectedRows.value.map((r) => r.id)
    await request.post<any>({ url: '/api/piracy/tracking/batch-block', data: { ids } })
    ElMessage.success(`已批量拉黑 ${ids.length} 条`)
    handleSearch()
    loadStats()
  })
}

function handleExportEvidence() {
  ElMessage.success(`正在导出 ${selectedRows.value.length} 条记录的证据...`)
}

onMounted(() => {
  loadStats()
  loadAppList()
  handleSearch()
})
</script>

<style scoped lang="scss">
.mb-4 { margin-bottom: 16px; }
.table-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.table-actions { display: flex; gap: 8px; }
.card-title { font-weight: 600; }
.pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }

.stats-card {
  text-align: center;
  .stats-title { font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 8px; }
  .stats-value { font-size: 24px; font-weight: 700; }
}

.text-danger { color: #f56c6c; }
.text-warning { color: #e6a23c; }
.text-primary { color: var(--el-color-primary); }
.hit-count { font-weight: 600; color: #f56c6c; }

.detail-section-title {
  font-size: 14px;
  font-weight: 600;
  margin: 16px 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
</style>