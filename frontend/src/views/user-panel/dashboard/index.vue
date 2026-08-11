<template>
  <div class="user-dashboard">
    <!-- 欢迎卡片 -->
    <div class="art-card p-6 mb-5 welcome-card">
      <div class="welcome-content">
        <div class="welcome-text">
          <h2 class="welcome-title">欢迎回来，{{ nickname }}</h2>
          <p class="welcome-desc">您当前有 <strong>{{ stats.active }}</strong> 个有效授权，<strong>{{ stats.expiring }}</strong> 个即将到期</p>
        </div>
        <div class="welcome-icon">
          <IconifyIcon icon="ri:shield-star-line" width="64" color="var(--el-color-primary-light-5)" />
        </div>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <div class="art-card p-5 stat-card">
        <div class="stat-icon" style="background: var(--el-color-primary-light-9)">
          <IconifyIcon icon="ri:shield-check-line" width="24" color="var(--el-color-primary)" />
        </div>
        <div class="stat-info">
          <span class="stat-num">{{ stats.total }}</span>
          <span class="stat-label">总授权数</span>
        </div>
      </div>
      <div class="art-card p-5 stat-card">
        <div class="stat-icon" style="background: var(--el-color-success-light-9)">
          <IconifyIcon icon="ri:check-double-line" width="24" color="var(--el-color-success)" />
        </div>
        <div class="stat-info">
          <span class="stat-num">{{ stats.active }}</span>
          <span class="stat-label">有效授权</span>
        </div>
      </div>
      <div class="art-card p-5 stat-card">
        <div class="stat-icon" style="background: var(--el-color-warning-light-9)">
          <IconifyIcon icon="ri:alarm-warning-line" width="24" color="var(--el-color-warning)" />
        </div>
        <div class="stat-info">
          <span class="stat-num">{{ stats.expiring }}</span>
          <span class="stat-label">即将到期</span>
        </div>
      </div>
      <div class="art-card p-5 stat-card">
        <div class="stat-icon" style="background: var(--el-color-danger-light-9)">
          <IconifyIcon icon="ri:close-circle-line" width="24" color="var(--el-color-danger)" />
        </div>
        <div class="stat-info">
          <span class="stat-num">{{ stats.expired }}</span>
          <span class="stat-label">已过期</span>
        </div>
      </div>
    </div>

    <!-- 最近授权 -->
    <el-card shadow="hover">
      <template #header><span class="card-title">我的授权</span></template>
      <el-table :data="recentLicenses" stripe>
        <el-table-column prop="domain" label="域名/IP/密钥" min-width="180" show-overflow-tooltip />
        <el-table-column prop="appName" label="应用" width="120" />
        <el-table-column prop="statusLabel" label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : row.status === 'expiring' ? 'warning' : 'info'" size="small">
              {{ row.statusLabel }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="expireAt" label="到期时间" width="130" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Icon as IconifyIcon } from '@iconify/vue'
import axios from 'axios'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const stats = reactive({
  total: 0,
  active: 0,
  expiring: 0,
  expired: 0
})

const nickname = ref('用户')
const recentLicenses = ref<any[]>([])

function getToken() {
  return localStorage.getItem('user_panel_token') || ''
}

async function fetchDashboard() {
  try {
    const { data } = await axios.get('/api/user-panel/dashboard', {
      headers: { Authorization: `Bearer ${getToken()}` }
    })
    if (data.code === 200) {
      const d = data.data
      nickname.value = d.nickname || '用户'
      Object.assign(stats, d.stats)
      recentLicenses.value = d.recentLicenses || []
    }
  } catch {}
}

// 支付回跳：轮询充值订单状态直到入账
const rechargeOrderStorageKey = 'user_panel_recharge_order'
let rechargePollTimer: ReturnType<typeof setInterval> | null = null

function stopRechargePoll() {
  if (rechargePollTimer) {
    clearInterval(rechargePollTimer)
    rechargePollTimer = null
  }
}

async function checkRechargeReturn() {
  const queryOrder = typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
  const orderNo = queryOrder || sessionStorage.getItem(rechargeOrderStorageKey) || ''
  if (!orderNo) return

  sessionStorage.removeItem(rechargeOrderStorageKey)
  const nextQuery = { ...route.query }
  delete nextQuery.rechargeOrder
  delete nextQuery.rechargeReturn
  router.replace({ query: nextQuery })

  ElMessage.info('正在确认支付结果…')
  let attempts = 0
  rechargePollTimer = setInterval(async () => {
    attempts++
    try {
      const { data } = await axios.get(`/api/user-panel/recharge/orders/${orderNo}`, {
        headers: { Authorization: `Bearer ${getToken()}` }
      })
      const status = data.data?.status
      if (status === 'paid') {
        stopRechargePoll()
        ElMessage.success('充值成功，余额已到账')
      } else if (status === 'failed' || status === 'cancelled' || attempts >= 20) {
        stopRechargePoll()
        if (status !== 'pending') ElMessage.warning('充值未完成')
      }
    } catch {
      if (attempts >= 20) stopRechargePoll()
    }
  }, 1500)
}

onMounted(() => {
  fetchDashboard()
  checkRechargeReturn()
})
</script>

<style scoped lang="scss">
.user-dashboard {
  background: var(--el-bg-color);

  .art-card,
  :deep(.el-card) {
    overflow: hidden;
    background: var(--el-bg-color);
    border-radius: 12px !important;
  }
}

.mb-5 { margin-bottom: 20px; }
.p-5 { padding: 20px; }
.p-6 { padding: 24px; }
.card-title { font-weight: 600; }

.welcome-card {
  .welcome-content {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .welcome-title {
    font-size: 20px;
    font-weight: 600;
    margin-bottom: 8px;
    color: var(--el-text-color-primary);
  }

  .welcome-desc {
    font-size: 14px;
    color: var(--el-text-color-secondary);

    strong {
      color: var(--el-color-primary);
    }
  }
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;

  .stat-card {
    display: flex;
    align-items: center;
    gap: 14px;

    .stat-icon {
      width: 48px;
      height: 48px;
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
    }

    .stat-info {
      display: flex;
      flex-direction: column;
      gap: 2px;

      .stat-num {
        font-size: 24px;
        font-weight: 700;
        color: var(--el-text-color-primary);
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      }

      .stat-label {
        font-size: 12px;
        color: var(--el-text-color-secondary);
      }
    }
  }
}

@media (max-width: 768px) {
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>