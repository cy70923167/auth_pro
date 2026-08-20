<template>
  <div class="agent-upgrade-audit">
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card">
          <span>升级订单</span>
          <strong>{{ stats.totalOrders }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card warning">
          <span>待处理</span>
          <strong>{{ stats.pendingOrders }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card success">
          <span>完成转换</span>
          <strong>{{ stats.completedConversions }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card primary">
          <span>开通费合计</span>
          <strong>¥{{ money(stats.completedAmount) }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card info">
          <span>迁移余额</span>
          <strong>¥{{ money(stats.transferredBalance) }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="8" :lg="4">
        <el-card shadow="hover" class="stat-card danger">
          <span>失败订单</span>
          <strong>{{ stats.failedOrders }}</strong>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="audit-card">
      <template #header>
        <div class="card-header">
          <div>
            <h2>代理升级审计</h2>
            <p
              >追踪余额开通订单、账户主体转换、余额与授权迁移结果。当前页面仅提供查询，不会重新执行转换。</p
            >
          </div>
          <el-button :loading="refreshing" @click="refreshAll">刷新</el-button>
        </div>
      </template>

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="升级订单" name="orders">
          <el-form :model="orderSearch" inline class="search-bar">
            <el-form-item label="关键词">
              <el-input
                v-model.trim="orderSearch.keyword"
                clearable
                placeholder="订单号 / 用户 / 代理"
                class="keyword-input"
                @keyup.enter="searchOrders"
                @clear="searchOrders"
              />
            </el-form-item>
            <el-form-item label="状态">
              <el-select
                v-model="orderSearch.status"
                clearable
                placeholder="全部状态"
                class="search-select"
              >
                <el-option
                  v-for="item in orderStatusOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="支付渠道">
              <el-select
                v-model="orderSearch.payChannel"
                clearable
                placeholder="全部渠道"
                class="search-select"
              >
                <el-option label="用户余额" value="balance" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="searchOrders">查询</el-button>
              <el-button @click="resetOrders">重置</el-button>
            </el-form-item>
          </el-form>

          <el-table v-loading="orderLoading" :data="orders" border stripe>
            <el-table-column
              prop="orderNo"
              label="升级订单号"
              min-width="190"
              show-overflow-tooltip
            >
              <template #default="{ row }"
                ><span class="mono">{{ row.orderNo }}</span></template
              >
            </el-table-column>
            <el-table-column label="原用户" min-width="190" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="subject-cell">
                  <strong>{{ row.userName || row.userEmail }}</strong>
                  <span>#{{ row.userId }} · {{ row.userEmail }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="代理等级" min-width="130">
              <template #default="{ row }">
                <div class="subject-cell">
                  <strong>{{ row.levelName }}</strong>
                  <span>{{ row.discount }} 折</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="开通费" width="110" align="right">
              <template #default="{ row }"
                ><span class="amount">¥{{ money(row.amount) }}</span></template
              >
            </el-table-column>
            <el-table-column label="开通赠送" width="110" align="right">
              <template #default="{ row }">
                <span :class="{ amount: row.openingBonus > 0 }"
                  >+ ¥{{ money(row.openingBonus) }}</span
                >
              </template>
            </el-table-column>
            <el-table-column label="支付渠道" width="100" align="center">
              <template #default="{ row }">{{ payChannelLabel(row.payChannel) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="orderStatusType(row.status)" size="small">{{
                  orderStatusLabel(row.status)
                }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="目标代理" min-width="130" show-overflow-tooltip>
              <template #default="{ row }">{{
                row.agentId ? `${row.agentName || '代理'} #${row.agentId}` : '-'
              }}</template>
            </el-table-column>
            <el-table-column
              prop="errorMessage"
              label="失败原因"
              min-width="180"
              show-overflow-tooltip
            >
              <template #default="{ row }"
                ><span :class="{ 'error-text': row.errorMessage }">{{
                  row.errorMessage || '-'
                }}</span></template
              >
            </el-table-column>
            <el-table-column prop="createdAt" label="创建时间" width="165" />
            <el-table-column prop="completedAt" label="完成时间" width="165">
              <template #default="{ row }">{{ row.completedAt || '-' }}</template>
            </el-table-column>
          </el-table>

          <div class="pagination">
            <el-pagination
              v-model:current-page="orderSearch.page"
              v-model:page-size="orderSearch.pageSize"
              :total="orderTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadOrders"
              @current-change="loadOrders"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="转换记录" name="conversions">
          <el-form :model="conversionSearch" inline class="search-bar">
            <el-form-item label="关键词">
              <el-input
                v-model.trim="conversionSearch.keyword"
                clearable
                placeholder="转换号 / 订单号 / 账号"
                class="keyword-input"
                @keyup.enter="searchConversions"
                @clear="searchConversions"
              />
            </el-form-item>
            <el-form-item label="状态">
              <el-select
                v-model="conversionSearch.status"
                clearable
                placeholder="全部状态"
                class="search-select"
              >
                <el-option label="处理中" value="processing" />
                <el-option label="已完成" value="completed" />
                <el-option label="失败" value="failed" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="searchConversions">查询</el-button>
              <el-button @click="resetConversions">重置</el-button>
            </el-form-item>
          </el-form>

          <el-table v-loading="conversionLoading" :data="conversions" border stripe>
            <el-table-column
              prop="conversionNo"
              label="转换流水号"
              min-width="190"
              show-overflow-tooltip
            >
              <template #default="{ row }"
                ><span class="mono">{{ row.conversionNo }}</span></template
              >
            </el-table-column>
            <el-table-column
              prop="orderNo"
              label="升级订单号"
              min-width="190"
              show-overflow-tooltip
            >
              <template #default="{ row }"
                ><span class="mono">{{ row.orderNo }}</span></template
              >
            </el-table-column>
            <el-table-column label="主体迁移" min-width="240">
              <template #default="{ row }">
                <div class="migration-cell">
                  <span>用户 #{{ row.userId }} · {{ row.userEmail }}</span>
                  <iconify-icon icon="ri:arrow-right-line" />
                  <span>代理 #{{ row.agentId }} · {{ row.agentName || row.agentEmail }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="levelName" label="代理等级" width="120" />
            <el-table-column label="开通费" width="110" align="right">
              <template #default="{ row }">¥{{ money(row.openingFee) }}</template>
            </el-table-column>
            <el-table-column label="迁移余额" width="110" align="right">
              <template #default="{ row }"
                ><span class="amount">¥{{ money(row.transferredBalance) }}</span></template
              >
            </el-table-column>
            <el-table-column label="开通赠送" width="110" align="right">
              <template #default="{ row }">+ ¥{{ money(row.openingBonus) }}</template>
            </el-table-column>
            <el-table-column label="代理余额" width="110" align="right">
              <template #default="{ row }"
                ><span class="amount">¥{{ money(row.finalBalance) }}</span></template
              >
            </el-table-column>
            <el-table-column
              prop="migratedLicenseCount"
              label="迁移授权"
              width="100"
              align="center"
            />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="conversionStatusType(row.status)" size="small">{{
                  conversionStatusLabel(row.status)
                }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column
              prop="errorMessage"
              label="异常原因"
              min-width="170"
              show-overflow-tooltip
            >
              <template #default="{ row }"
                ><span :class="{ 'error-text': row.errorMessage }">{{
                  row.errorMessage || '-'
                }}</span></template
              >
            </el-table-column>
            <el-table-column prop="completedAt" label="完成时间" width="165">
              <template #default="{ row }">{{ row.completedAt || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openConversionDetail(row)">详情</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination">
            <el-pagination
              v-model:current-page="conversionSearch.page"
              v-model:page-size="conversionSearch.pageSize"
              :total="conversionTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadConversions"
              @current-change="loadConversions"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog v-model="detailVisible" title="账户转换审计快照" width="680px">
      <el-descriptions v-if="detail" :column="1" border>
        <el-descriptions-item label="转换流水号"
          ><span class="mono">{{ detail.conversionNo }}</span></el-descriptions-item
        >
        <el-descriptions-item label="升级订单号"
          ><span class="mono">{{ detail.orderNo }}</span></el-descriptions-item
        >
        <el-descriptions-item label="转换状态">{{
          conversionStatusLabel(detail.status)
        }}</el-descriptions-item>
        <el-descriptions-item label="异常原因">{{
          detail.errorMessage || '-'
        }}</el-descriptions-item>
        <el-descriptions-item label="原用户快照">
          <pre>{{ snapshotText(detail.sourceSnapshot) }}</pre>
        </el-descriptions-item>
        <el-descriptions-item label="转换结果快照">
          <pre>{{ snapshotText(detail.resultSnapshot) }}</pre>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { onMounted, reactive, ref } from 'vue'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import request from '@/utils/http'

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

  type UpgradeStats = {
    totalOrders: number
    pendingOrders: number
    completedOrders: number
    failedOrders: number
    completedAmount: number
    completedConversions: number
    transferredBalance: number
    openingBonus: number
    migratedLicenses: number
  }

  type UpgradeOrder = {
    id: number
    orderNo: string
    userId: number
    userEmail: string
    userName: string
    levelId: number
    levelCode: string
    levelName: string
    discount: number
    amount: number
    openingBonus: number
    paidAmount: number | null
    payChannel: string
    payMethod: string
    status: string
    agentId: number | null
    agentName: string
    gatewayTradeNo: string
    errorMessage: string
    createdAt: string
    paidAt: string
    completedAt: string
    updatedAt: string
  }

  type AccountConversion = {
    id: number
    conversionNo: string
    orderNo: string
    userId: number
    userEmail: string
    userName: string
    agentId: number | null
    agentEmail: string
    agentName: string
    levelId: number
    levelName: string
    status: string
    openingFee: number
    transferredBalance: number
    openingBonus: number
    finalBalance: number
    migratedLicenseCount: number
    errorMessage: string
    startedAt: string
    completedAt: string
    createdAt: string
    updatedAt: string
  }

  type ConversionDetail = {
    id: number
    conversionNo: string
    orderNo: string
    status: string
    errorMessage: string
    sourceSnapshot: unknown
    resultSnapshot: unknown
  }

  const emptyStats: UpgradeStats = {
    totalOrders: 0,
    pendingOrders: 0,
    completedOrders: 0,
    failedOrders: 0,
    completedAmount: 0,
    completedConversions: 0,
    transferredBalance: 0,
    openingBonus: 0,
    migratedLicenses: 0
  }

  const activeTab = ref('orders')
  const refreshing = ref(false)
  const orderLoading = ref(false)
  const conversionLoading = ref(false)
  const stats = reactive<UpgradeStats>({ ...emptyStats })
  const orders = ref<UpgradeOrder[]>([])
  const conversions = ref<AccountConversion[]>([])
  const orderTotal = ref(0)
  const conversionTotal = ref(0)
  const detailVisible = ref(false)
  const detail = ref<ConversionDetail>()

  const orderSearch = reactive({ page: 1, pageSize: 20, keyword: '', status: '', payChannel: '' })
  const conversionSearch = reactive({ page: 1, pageSize: 20, keyword: '', status: '' })

  const orderStatusOptions = [
    { label: '待支付', value: 'pending' },
    { label: '已支付', value: 'paid' },
    { label: '转换中', value: 'processing' },
    { label: '已完成', value: 'completed' },
    { label: '失败', value: 'failed' },
    { label: '已取消', value: 'cancelled' }
  ]

  function money(value: unknown) {
    return Number(value || 0).toFixed(2)
  }

  function orderStatusLabel(status: string) {
    return (
      (
        {
          pending: '待支付',
          paid: '已支付',
          processing: '转换中',
          completed: '已完成',
          failed: '失败',
          cancelled: '已取消'
        } as Record<string, string>
      )[status] || status
    )
  }

  function orderStatusType(status: string): TagType {
    return (
      (
        {
          pending: 'warning',
          paid: 'primary',
          processing: 'warning',
          completed: 'success',
          failed: 'danger',
          cancelled: 'info'
        } as Record<string, TagType>
      )[status] || 'info'
    )
  }

  function conversionStatusLabel(status: string) {
    return (
      ({ processing: '处理中', completed: '已完成', failed: '失败' } as Record<string, string>)[
        status
      ] || status
    )
  }

  function conversionStatusType(status: string): TagType {
    return (
      (
        { processing: 'warning', completed: 'success', failed: 'danger' } as Record<string, TagType>
      )[status] || 'info'
    )
  }

  function payChannelLabel(channel: string) {
    if (channel === 'balance') return '用户余额'
    return channel || '-'
  }

  function snapshotText(value: unknown) {
    if (value === null || value === undefined) return '-'
    if (typeof value === 'string') return value
    return JSON.stringify(value, null, 2)
  }

  async function loadStats() {
    const data = await request.get<UpgradeStats>({ url: '/api/admin/agent-upgrade/stats' })
    Object.assign(stats, emptyStats, data || {})
  }

  async function loadOrders() {
    orderLoading.value = true
    try {
      const data = await request.get<{ list: UpgradeOrder[]; total: number }>({
        url: '/api/admin/agent-upgrade/orders',
        params: orderSearch
      })
      orders.value = data.list || []
      orderTotal.value = Number(data.total || 0)
    } finally {
      orderLoading.value = false
    }
  }

  async function loadConversions() {
    conversionLoading.value = true
    try {
      const data = await request.get<{ list: AccountConversion[]; total: number }>({
        url: '/api/admin/agent-upgrade/conversions',
        params: conversionSearch
      })
      conversions.value = data.list || []
      conversionTotal.value = Number(data.total || 0)
    } finally {
      conversionLoading.value = false
    }
  }

  function searchOrders() {
    orderSearch.page = 1
    loadOrders()
  }

  function resetOrders() {
    Object.assign(orderSearch, { page: 1, keyword: '', status: '', payChannel: '' })
    loadOrders()
  }

  function searchConversions() {
    conversionSearch.page = 1
    loadConversions()
  }

  function resetConversions() {
    Object.assign(conversionSearch, { page: 1, keyword: '', status: '' })
    loadConversions()
  }

  function handleTabChange(name: string | number) {
    if (name === 'conversions' && conversions.value.length === 0) loadConversions()
  }

  async function openConversionDetail(row: AccountConversion) {
    detail.value = await request.get<ConversionDetail>({
      url: `/api/agent-upgrade/conversions/${row.id}`
    })
    detailVisible.value = true
  }

  async function refreshAll() {
    refreshing.value = true
    try {
      await Promise.all([
        loadStats(),
        activeTab.value === 'orders' ? loadOrders() : loadConversions()
      ])
    } finally {
      refreshing.value = false
    }
  }

  onMounted(() => {
    Promise.all([loadStats(), loadOrders()])
  })
</script>

<style scoped lang="scss">
  .agent-upgrade-audit {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .stats-row {
    row-gap: 16px;
  }

  .stat-card {
    border-left: 3px solid var(--el-border-color);

    :deep(.el-card__body) {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    span {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }

    strong {
      font-size: 24px;
      line-height: 1;
    }

    &.primary {
      border-left-color: var(--el-color-primary);
    }

    &.success {
      border-left-color: var(--el-color-success);
    }

    &.warning {
      border-left-color: var(--el-color-warning);
    }

    &.danger {
      border-left-color: var(--el-color-danger);
    }

    &.info {
      border-left-color: var(--el-color-info);
    }
  }

  .card-header {
    display: flex;
    gap: 20px;
    align-items: center;
    justify-content: space-between;

    h2 {
      margin: 0 0 6px;
      font-size: 19px;
    }

    p {
      margin: 0;
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }
  }

  .search-bar {
    margin-bottom: 4px;
  }

  .keyword-input {
    width: 240px;
  }

  .search-select {
    width: 140px;
  }

  .subject-cell,
  .migration-cell {
    display: flex;
    flex-direction: column;
    gap: 3px;

    span {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .migration-cell svg {
    color: var(--el-color-primary);
  }

  .mono {
    font-family: 'Roboto Mono', monospace;
    font-size: 12px;
  }

  .amount {
    font-weight: 600;
    color: var(--el-color-success);
  }

  .error-text {
    color: var(--el-color-danger);
  }

  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 18px;
  }

  pre {
    max-height: 260px;
    padding: 12px;
    margin: 0;
    overflow: auto;
    font-family: 'Roboto Mono', monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    background: var(--el-fill-color-light);
    border-radius: 6px;
  }

  @media (width <= 768px) {
    .card-header {
      align-items: flex-start;
    }

    .keyword-input,
    .search-select {
      width: 100%;
    }
  }
</style>
