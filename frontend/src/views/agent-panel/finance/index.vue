<template>
  <div class="panel-finance">
    <!-- 统计卡片 -->
    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-icon balance">
          <iconify-icon icon="ri:wallet-3-line" width="24" />
        </div>
        <div class="stat-info">
          <span class="stat-label">账户余额</span>
          <div class="stat-value">
            <span class="currency">¥</span>
            <span class="num">{{ balanceParts.int }}</span>
            <span class="dec">.{{ balanceParts.dec }}</span>
          </div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon recharge">
          <iconify-icon icon="ri:arrow-down-circle-line" width="24" />
        </div>
        <div class="stat-info">
          <span class="stat-label">累计充值</span>
          <div class="stat-value">
            <span class="currency">¥</span>
            <span class="num">{{ formatInt(overview.totalRecharge) }}</span>
            <span class="dec">.{{ formatDec(overview.totalRecharge) }}</span>
          </div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon consume">
          <iconify-icon icon="ri:arrow-up-circle-line" width="24" />
        </div>
        <div class="stat-info">
          <span class="stat-label">累计消费</span>
          <div class="stat-value">
            <span class="currency">¥</span>
            <span class="num">{{ formatInt(overview.totalConsume) }}</span>
            <span class="dec">.{{ formatDec(overview.totalConsume) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 充值 + 配额 -->
    <div class="panel-row">
      <div class="art-card panel-col">
        <div class="panel-header">
          <iconify-icon icon="ri:flashlight-line" width="18" class="panel-icon" />
          <span class="panel-title">快捷充值</span>
        </div>
        <div class="recharge-presets">
          <span
            v-for="amt in presetAmounts"
            :key="amt"
            class="preset-chip"
            :class="{ active: rechargeAmount === amt }"
            @click="rechargeAmount = amt"
          >¥{{ amt }}</span>
        </div>
        <div class="recharge-input-row">
          <el-input-number
            v-model="rechargeAmount"
            :min="1"
            :max="1000000"
            :precision="2"
            :controls="false"
            placeholder="自定义金额"
            class="recharge-input"
          />
          <el-button type="primary" class="recharge-btn" @click="handleRecharge">
            <iconify-icon icon="ri:flashlight-line" width="15" />
            <span>立即充值</span>
          </el-button>
        </div>
      </div>

      <div class="art-card panel-col">
        <div class="panel-header">
          <iconify-icon icon="ri:gift-line" width="18" class="panel-icon" />
          <span class="panel-title">当前配额</span>
        </div>
        <div v-if="quotaInfo.length" class="quota-items">
          <div v-for="(q, idx) in quotaInfo" :key="idx" class="quota-item">
            <div class="quota-header">
              <span class="quota-name">{{ q.appName }}</span>
              <span class="quota-nums">{{ q.used }}/{{ q.total === -1 ? '∞' : q.total }}</span>
            </div>
            <el-progress
              v-if="q.total !== -1"
              :percentage="Math.round((q.used / q.total) * 100)"
              :color="getProgressColor(q.used / q.total)"
              :stroke-width="6"
              :show-text="false"
            />
            <el-progress v-else :percentage="30" color="#67c23a" :stroke-width="6" :show-text="false" />
          </div>
        </div>
        <div v-else class="quota-empty">暂无配额，可联系管理员开通</div>
      </div>
    </div>

    <!-- 收支明细 -->
    <div class="art-card">
      <div class="panel-header with-filter">
        <div class="panel-header-left">
          <iconify-icon icon="ri:file-list-3-line" width="18" class="panel-icon" />
          <span class="panel-title">收支明细</span>
        </div>
        <div class="filter-bar">
          <el-select v-model="searchForm.type" placeholder="全部类型" clearable style="width: 120px">
            <el-option label="充值" value="recharge" />
            <el-option label="消费" value="consume" />
            <el-option label="退款" value="refund" />
          </el-select>
          <el-date-picker
            v-model="searchForm.dateRange"
            type="daterange"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 240px"
          />
          <el-button type="primary" plain @click="handleSearch">查询</el-button>
          <el-button text @click="handleReset">重置</el-button>
        </div>
      </div>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="orderNo" label="流水号" min-width="180" show-overflow-tooltip />
        <el-table-column prop="typeLabel" label="类型" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small" effect="light">{{ row.typeLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="130" align="right">
          <template #default="{ row }">
            <span :class="amountClass(row)">{{ formatAmount(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="balanceAfter" label="余额" width="120" align="right">
          <template #default="{ row }">¥{{ row.balanceAfter.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="220" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="时间" width="170" />
        <template #empty>
          <el-empty description="暂无流水记录" :image-size="80" />
        </template>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handlePageChange"
          @size-change="handleSearch"
        />
      </div>
    </div>

    <!-- 充值方式选择 -->
    <el-dialog v-model="rechargeDialog.visible" title="选择支付方式" width="420px">
      <div class="recharge-dialog-body">
        <div class="recharge-amount-preview">
          充值金额 <em>¥{{ Number(rechargeAmount).toFixed(2) }}</em>
        </div>
        <el-radio-group v-model="rechargeDialog.payType" class="pay-type-group">
          <el-radio-button v-for="pt in rechargeOptions.payTypes" :key="pt" :value="pt">
            {{ payTypeLabels[pt] || pt }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <template #footer>
        <el-button @click="rechargeDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="rechargeDialog.submitting" @click="submitRecharge">
          去支付
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Icon as IconifyIcon } from '@iconify/vue'
import axios from 'axios'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

function getToken() {
  return localStorage.getItem('agent_panel_token') || ''
}
const authHeaders = computed(() => ({ Authorization: `Bearer ${getToken()}` }))

// ========== 余额概览 ==========
const overview = reactive({ balance: 0, totalRecharge: 0, totalConsume: 0 })
const balanceParts = computed(() => {
  const [intPart, decPart] = Math.abs(overview.balance).toFixed(2).split('.')
  return { int: Number(intPart).toLocaleString(), dec: decPart }
})
function formatInt(v: number) {
  return Number(Math.abs(v).toFixed(2).split('.')[0]).toLocaleString()
}
function formatDec(v: number) {
  return Math.abs(v).toFixed(2).split('.')[1]
}

async function fetchOverview() {
  try {
    const { data } = await axios.get('/api/agent-panel/finance/overview', { headers: authHeaders.value })
    if (data.code === 200) {
      overview.balance = Number(data.data?.balance || 0)
      overview.totalRecharge = Number(data.data?.totalRecharge || 0)
      overview.totalConsume = Number(data.data?.totalConsume || 0)
    }
  } catch {}
}

// ========== 配额 ==========
const quotaInfo = ref<any[]>([])

async function fetchQuotas() {
  try {
    const { data } = await axios.get('/api/agent-panel/finance/quotas', { headers: authHeaders.value })
    if (data.code === 200) {
      quotaInfo.value = Array.isArray(data.data) ? data.data : []
    }
  } catch {}
}

function getProgressColor(ratio: number) {
  if (ratio >= 0.9) return '#f56c6c'
  if (ratio >= 0.7) return '#e6a23c'
  return '#409eff'
}

// ========== 流水列表 ==========
const loading = ref(false)
const searchForm = reactive({ type: '', dateRange: [] as string[] })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const tableData = ref<any[]>([])

const typeTagMap: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined> = { recharge: 'success', consume: 'danger', refund: 'warning' } as const

// 金额按符号渲染：消费为负（红），充值/退款为正（绿），0 元中性灰
function amountClass(row: any) {
  if (row.amount < 0) return 'amount-negative'
  if (row.amount > 0) return 'amount-positive'
  return 'amount-zero'
}
function formatAmount(row: any) {
  if (row.amount < 0) return `-¥${Math.abs(row.amount).toFixed(2)}`
  if (row.amount > 0) return `+¥${row.amount.toFixed(2)}`
  return `¥0.00`
}

async function fetchTransactions() {
  loading.value = true
  try {
    const params: any = { page: pagination.page, pageSize: pagination.pageSize }
    if (searchForm.type) params.type = searchForm.type
    if (searchForm.dateRange?.length === 2) {
      params.startDate = searchForm.dateRange[0]
      params.endDate = searchForm.dateRange[1]
    }
    const { data } = await axios.get('/api/agent-panel/finance/transactions', { headers: authHeaders.value, params })
    if (data.code === 200) {
      tableData.value = data.data?.list || []
      pagination.total = Number(data.data?.total || 0)
    }
  } catch {
    ElMessage.error('加载流水失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  fetchTransactions()
}
function handleReset() {
  searchForm.type = ''
  searchForm.dateRange = []
  pagination.page = 1
  fetchTransactions()
}
function handlePageChange(page: number) {
  pagination.page = page
  fetchTransactions()
}

// ========== 充值 ==========
const rechargeAmount = ref(500)
const presetAmounts = [100, 300, 500, 1000, 2000, 5000]
const rechargeOptions = reactive({ enabled: false, payTypes: [] as string[], defaultType: 'alipay' })
const rechargeDialog = reactive({ visible: false, payType: 'alipay', submitting: false })
let rechargePollTimer: ReturnType<typeof setInterval> | null = null

const payTypeLabels: Record<string, string> = { alipay: '支付宝', wxpay: '微信支付', qqpay: 'QQ钱包', bank: '网银' }

async function fetchRechargeOptions() {
  try {
    const { data } = await axios.get('/api/agent-panel/recharge/options', { headers: authHeaders.value })
    if (data.code === 200) {
      rechargeOptions.enabled = !!data.data?.enabled
      rechargeOptions.payTypes = Array.isArray(data.data?.payTypes) ? data.data.payTypes : []
      rechargeOptions.defaultType = data.data?.defaultType || 'alipay'
    }
  } catch {}
}

function handleRecharge() {
  if (!rechargeOptions.enabled || rechargeOptions.payTypes.length === 0) {
    ElMessage.warning('线上支付未开启，请联系管理员')
    return
  }
  const amount = Number(rechargeAmount.value)
  if (!amount || amount <= 0) {
    ElMessage.warning('请输入充值金额')
    return
  }
  rechargeDialog.payType = rechargeOptions.payTypes.includes(rechargeOptions.defaultType)
    ? rechargeOptions.defaultType
    : rechargeOptions.payTypes[0]
  rechargeDialog.visible = true
}

async function submitRecharge() {
  if (rechargeDialog.submitting) return
  rechargeDialog.submitting = true
  try {
    const { data } = await axios.post('/api/agent-panel/recharge/orders', {
      amount: Number(rechargeAmount.value),
      payType: rechargeDialog.payType
    }, { headers: authHeaders.value })
    if (data.code === 200 && data.data?.payUrl) {
      sessionStorage.setItem('agent_panel_recharge_order', data.data.orderNo)
      ElMessage.success('充值订单已创建，正在跳转收银台')
      window.location.href = data.data.payUrl
    } else {
      ElMessage.error(data.msg || '创建充值订单失败')
    }
  } catch {
    ElMessage.error('请求失败，请重试')
  } finally {
    rechargeDialog.submitting = false
  }
}

// 支付回跳：轮询订单状态直到入账
async function checkRechargeReturn() {
  const queryOrder = typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
  const orderNo = queryOrder || sessionStorage.getItem('agent_panel_recharge_order') || ''
  if (!orderNo) return

  sessionStorage.removeItem('agent_panel_recharge_order')
  const nextQuery = { ...route.query }
  delete nextQuery.rechargeOrder
  delete nextQuery.rechargeReturn
  router.replace({ query: nextQuery })

  ElMessage.info('正在确认支付结果…')
  let attempts = 0
  rechargePollTimer = setInterval(async () => {
    attempts++
    try {
      const { data } = await axios.get(`/api/agent-panel/recharge/orders/${orderNo}`, { headers: authHeaders.value })
      const status = data.data?.status
      if (status === 'paid') {
        stopRechargePoll()
        ElMessage.success('充值成功，余额已到账')
        fetchOverview()
        fetchTransactions()
      } else if (status === 'failed' || status === 'cancelled' || attempts >= 20) {
        stopRechargePoll()
        if (status !== 'pending') ElMessage.warning('充值未完成')
      }
    } catch {
      if (attempts >= 20) stopRechargePoll()
    }
  }, 1500)
}

function stopRechargePoll() {
  if (rechargePollTimer) {
    clearInterval(rechargePollTimer)
    rechargePollTimer = null
  }
}

onMounted(() => {
  fetchOverview()
  fetchQuotas()
  fetchTransactions()
  fetchRechargeOptions()
  checkRechargeReturn()
})
</script>

<style scoped lang="scss">
.panel-finance {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 顶部统计卡片 */
.stat-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;

  .stat-card {
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    padding: 20px 24px;
    display: flex;
    align-items: center;
    gap: 16px;
    transition: box-shadow 0.25s ease;

    &:hover {
      box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
    }

    .stat-icon {
      width: 48px;
      height: 48px;
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;

      &.balance {
        background: var(--el-color-primary-light-9);
        color: var(--el-color-primary);
      }
      &.recharge {
        background: var(--el-color-success-light-9);
        color: var(--el-color-success);
      }
      &.consume {
        background: var(--el-color-danger-light-9);
        color: var(--el-color-danger);
      }
    }

    .stat-info {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
    }

    .stat-label {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }

    .stat-value {
      display: flex;
      align-items: baseline;
      gap: 2px;
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;

      .currency {
        font-size: 15px;
        font-weight: 600;
        color: var(--el-text-color-regular);
      }
      .num {
        font-size: 26px;
        font-weight: 700;
        color: var(--el-text-color-primary);
      }
      .dec {
        font-size: 15px;
        font-weight: 600;
        color: var(--el-text-color-secondary);
      }
    }
  }
}

/* 充值 + 配额双栏 */
.panel-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;

  .panel-col {
    padding: 20px 24px;
  }
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;

  &.with-filter {
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
    padding: 20px 24px 0;
    margin-bottom: 0;
  }

  .panel-header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .panel-icon {
    color: var(--el-color-primary);
  }

  .panel-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

/* 充值 */
.recharge-presets {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 14px;

  .preset-chip {
    padding: 5px 14px;
    border-radius: 16px;
    font-size: 13px;
    font-weight: 500;
    background: var(--el-fill-color-light);
    color: var(--el-text-color-regular);
    cursor: pointer;
    transition: all 0.2s ease;
    border: 1px solid transparent;
    user-select: none;

    &:hover {
      background: var(--el-color-primary-light-9);
      color: var(--el-color-primary);
    }

    &.active {
      background: var(--el-color-primary-light-9);
      color: var(--el-color-primary);
      border-color: var(--el-color-primary-light-5);
      font-weight: 600;
    }
  }
}

.recharge-input-row {
  display: flex;
  align-items: center;
  gap: 10px;

  .recharge-input {
    width: 180px;
  }

  .recharge-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    font-weight: 600;
  }
}

/* 配额 */
.quota-items {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;

  .quota-item {
    min-width: 160px;
    flex: 1;

    .quota-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;
    }

    .quota-name {
      font-size: 13px;
      font-weight: 500;
    }

    .quota-nums {
      font-size: 12px;
      color: var(--el-text-color-secondary);
      font-family: 'DIN Alternate', monospace;
    }
  }
}

.quota-empty {
  font-size: 13px;
  color: var(--el-text-color-placeholder);
  padding: 12px 0;
}

/* 明细表格 */
.art-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;

  .el-table {
    --el-table-header-bg-color: var(--el-fill-color-light);
  }
}

.amount-negative { color: var(--el-color-danger); font-weight: 600; font-family: 'DIN Alternate', monospace; }
.amount-positive { color: var(--el-color-success); font-weight: 600; font-family: 'DIN Alternate', monospace; }
.amount-zero { color: var(--el-text-color-secondary); font-family: 'DIN Alternate', monospace; }

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 16px 24px;
}

/* 充值弹窗 */
.recharge-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 16px;

  .recharge-amount-preview {
    font-size: 14px;
    color: var(--el-text-color-secondary);

    em {
      font-style: normal;
      font-size: 18px;
      font-weight: 700;
      color: var(--el-color-primary);
      margin-left: 4px;
    }
  }

  .pay-type-group {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
}

@media (max-width: 900px) {
  .stat-cards { grid-template-columns: 1fr; }
  .panel-row { grid-template-columns: 1fr; }
}
</style>
