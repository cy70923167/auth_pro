<template>
  <div class="agent-recharge">
    <!-- 统计卡片 -->
    <el-row :gutter="16" class="mb-4">
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">总充值</div>
          <div class="stats-value text-primary">¥{{ stats.totalRecharge.toFixed(2) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">总消费</div>
          <div class="stats-value text-danger">¥{{ stats.totalConsume.toFixed(2) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">本月充值</div>
          <div class="stats-value text-success">¥{{ stats.monthRecharge.toFixed(2) }}</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-title">本月消费</div>
          <div class="stats-value text-warning">¥{{ stats.monthConsume.toFixed(2) }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="代理商">
          <el-select v-model="searchForm.agentId" placeholder="全部" clearable style="width: 150px">
            <el-option v-for="a in agentList" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="searchForm.type" placeholder="全部" clearable style="width: 120px">
            <el-option label="充值" value="recharge" />
            <el-option label="消费" value="consume" />
            <el-option label="退款" value="refund" />
            <el-option label="开通赠送" value="bonus" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
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

    <!-- 流水表格 -->
    <el-card shadow="hover">
      <template #header>
        <span class="card-title">财务流水</span>
      </template>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="orderNo" label="流水号" min-width="180" show-overflow-tooltip />
        <el-table-column prop="agentName" label="代理商" width="120" />
        <el-table-column prop="typeLabel" label="类型" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small">{{ row.typeLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额(元)" width="120" align="right">
          <template #default="{ row }">
            <span :class="row.type === 'consume' ? 'text-danger' : 'text-success'">
              {{ row.type === 'consume' ? '-' : '+' }}¥{{ row.amount.toFixed(2) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="balanceAfter" label="余额(元)" width="110" align="right">
          <template #default="{ row }"> ¥{{ row.balanceAfter.toFixed(2) }} </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="150" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="时间" width="170" />
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
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, onMounted } from 'vue'
  import request from '@/utils/http'

  const loading = ref(false)

  const stats = reactive({
    totalRecharge: 0,
    totalConsume: 0,
    monthRecharge: 0,
    monthConsume: 0
  })

  const searchForm = reactive({ agentId: '', type: '', dateRange: [] as any[] })
  const pagination = reactive({ page: 1, pageSize: 20, total: 0 })

  const agentList = ref<{ id: string; name: string }[]>([])
  const typeTagMap: Record<
    string,
    'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined
  > = {
    recharge: 'success',
    consume: 'danger',
    refund: 'warning',
    transfer: 'info',
    bonus: 'success'
  } as const
  const tableData = ref<any[]>([])

  async function fetchStats() {
    try {
      const data = await request.get<any>({ url: '/api/transaction/stats' })
      Object.assign(stats, data)
    } catch {
      return
    }
  }

  async function fetchAgentList() {
    try {
      const data = await request.get<any[]>({ url: '/api/agent/select-list' })
      agentList.value = (data || []).map((a: any) => ({ id: String(a.id), name: a.name }))
    } catch {
      return
    }
  }

  async function handleSearch() {
    loading.value = true
    try {
      const params: Record<string, any> = { page: pagination.page, pageSize: pagination.pageSize }
      if (searchForm.agentId) params.agentId = searchForm.agentId
      if (searchForm.type) params.type = searchForm.type
      if (searchForm.dateRange && searchForm.dateRange.length === 2) {
        const fmt = (d: Date) => d.toISOString().slice(0, 10)
        params.startDate = fmt(searchForm.dateRange[0])
        params.endDate = fmt(searchForm.dateRange[1])
      }

      const data = await request.get<{ list: any[]; total: number }>({
        url: '/api/transaction/list',
        params
      })
      tableData.value = data.list || []
      pagination.total = data.total || 0
    } catch (e) {
      console.error('[Transaction] 查询失败:', e)
    } finally {
      loading.value = false
    }
  }

  function handleReset() {
    searchForm.agentId = ''
    searchForm.type = ''
    searchForm.dateRange = []
    pagination.page = 1
    handleSearch()
  }

  onMounted(() => {
    fetchStats()
    fetchAgentList()
    handleSearch()
  })
</script>

<style scoped lang="scss">
  .mb-4 {
    margin-bottom: 16px;
  }
  .card-title {
    font-weight: 600;
  }
  .pagination-wrapper {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .stats-card {
    text-align: center;
    .stats-title {
      font-size: 13px;
      color: var(--el-text-color-secondary);
      margin-bottom: 8px;
    }
    .stats-value {
      font-size: 22px;
      font-weight: 600;
    }
  }

  .text-primary {
    color: var(--el-color-primary);
  }
  .text-success {
    color: #67c23a;
  }
  .text-danger {
    color: #f56c6c;
  }
  .text-warning {
    color: #e6a23c;
  }
</style>
