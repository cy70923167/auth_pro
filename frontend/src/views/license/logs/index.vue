<template>
  <div class="license-logs">
    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="域名/IP/密钥">
          <el-input v-model="searchForm.keyword" placeholder="请输入" clearable style="width: 180px" />
        </el-form-item>
        <el-form-item label="应用">
          <el-select v-model="searchForm.appId" placeholder="全部" clearable style="width: 130px">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="验证结果">
          <el-select v-model="searchForm.result" placeholder="全部" clearable style="width: 110px">
            <el-option label="通过" value="pass" />
            <el-option label="拒绝" value="reject" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="searchForm.dateRange"
            type="daterange"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="width: 240px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 日志表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">验证日志</span>
          <el-button type="danger" plain @click="handleClear">清空日志</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="requestDomain" label="请求域名/IP" min-width="180" show-overflow-tooltip />
        <el-table-column prop="appName" label="应用" width="110" />
        <el-table-column prop="result" label="结果" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.result === 'pass' ? 'success' : 'danger'" size="small">
              {{ row.result === 'pass' ? '通过' : '拒绝' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="150" show-overflow-tooltip />
        <el-table-column prop="clientIp" label="来源IP" width="140" />
        <el-table-column prop="serverIp" label="本机IP" width="140" />
        <el-table-column prop="responseTime" label="响应(ms)" width="90" align="center" />
        <el-table-column prop="createdAt" label="时间" width="170" />
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/http'

const loading = ref(false)

const searchForm = reactive({
  keyword: '',
  appId: '',
  result: '',
  dateRange: [] as any[]
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const appList = ref<{ id: string; name: string }[]>([])
const tableData = ref<any[]>([])

async function fetchAppList() {
  try {
    const data = await request.get<any[]>({ url: '/api/license/apps' })
    appList.value = data || []
  } catch {
    appList.value = []
  }
}

async function handleSearch() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.appId) params.appId = searchForm.appId
    if (searchForm.result) params.result = searchForm.result
    if (searchForm.dateRange && searchForm.dateRange.length === 2) {
      const fmt = (d: Date) => d.toISOString().slice(0, 10)
      params.startDate = fmt(searchForm.dateRange[0])
      params.endDate = fmt(searchForm.dateRange[1])
    }

    const data = await request.get<{ list: any[]; total: number }>({
      url: '/api/verify-log/list',
      params
    })
    tableData.value = data.list || []
    pagination.total = data.total || 0
  } catch (e) {
    console.error('[VerifyLog] 查询失败:', e)
  } finally {
    loading.value = false
  }
}

function handleReset() {
  searchForm.keyword = ''
  searchForm.appId = ''
  searchForm.result = ''
  searchForm.dateRange = []
  pagination.page = 1
  handleSearch()
}

async function handleClear() {
  try {
    await ElMessageBox.confirm('确定清空所有验证日志？此操作不可撤销', '警告', { type: 'error' })
    await request.del({ url: '/api/verify-log/clear' })
    ElMessage.success('日志已清空')
    handleSearch()
  } catch {}
}

onMounted(() => {
  fetchAppList()
  handleSearch()
})
</script>

<style scoped lang="scss">
.mb-4 {
  margin-bottom: 16px;
}

.table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  font-weight: 600;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>