<template>
  <div class="admin-dashboard">
    <ElRow :gutter="16" class="card-row">
      <ElCol v-for="item in overview.cards" :key="item.title" :xs="12" :sm="8" :lg="4">
        <div class="art-card stat-card">
          <div class="stat-main">
            <div>
              <p class="stat-title">{{ item.title }}</p>
              <p class="stat-value">
                <span v-if="item.prefix">{{ item.prefix }}</span>{{ formatValue(item.value) }}
                <span class="stat-unit">{{ item.unit }}</span>
              </p>
            </div>
            <div class="stat-icon">
              <ArtSvgIcon :icon="item.icon" />
            </div>
          </div>
          <div class="stat-footer">
            <span :class="item.trend >= 0 ? 'text-success' : 'text-danger'">
              {{ item.trend >= 0 ? '+' : '' }}{{ item.trend }}%
            </span>
            <span>较昨日</span>
          </div>
        </div>
      </ElCol>
    </ElRow>

    <ElRow :gutter="16" class="trend-status-row">
      <ElCol :xs="24" :lg="16" class="trend-status-col">
        <div class="art-card panel-card trend-panel">
          <div class="panel-header trend-header">
            <div>
              <h3>经营趋势</h3>
              <p>最近 7 天收入、订单和新增授权</p>
            </div>
            <div class="trend-summary">
              <div class="summary-item revenue">
                <span>7日收入</span>
                <strong>¥{{ formatValue(trendTotals.revenue) }}</strong>
              </div>
              <div class="summary-item">
                <span>订单</span>
                <strong>{{ trendTotals.orders }}</strong>
              </div>
              <div class="summary-item">
                <span>新增授权</span>
                <strong>{{ trendTotals.licenses }}</strong>
              </div>
            </div>
          </div>
          <ArtLineChart
            height="300px"
            :data="revenueTrend"
            :xAxisData="trendDates"
            :showAreaColor="true"
            :showAxisLine="false"
          />
          <div class="trend-extra">
            <div v-for="item in overview.trend" :key="item.date" class="trend-extra-item">
              <span class="trend-date">{{ item.date }}</span>
              <strong class="trend-revenue">¥{{ formatValue(item.revenue) }}</strong>
              <span class="trend-meta">
                <em>{{ item.orders }} 订单</em>
                <i></i>
                <em>{{ item.licenses }} 授权</em>
              </span>
            </div>
          </div>
        </div>
      </ElCol>

      <ElCol :xs="24" :lg="8" class="trend-status-col">
        <div class="art-card panel-card status-panel">
          <div class="panel-header">
            <div>
              <h3>授权状态</h3>
              <p>当前授权健康度</p>
            </div>
          </div>
          <div class="status-list">
            <div v-for="item in overview.licenseStatus" :key="item.type" class="status-item">
              <div>
                <span class="status-dot" :class="`status-${item.type}`"></span>
                <span>{{ item.name }}</span>
              </div>
              <b>{{ item.value }}</b>
            </div>
          </div>

          <div class="payment-box">
            <div class="panel-header compact">
              <div>
                <h3>今日支付方式</h3>
                <p>当前实际以余额支付为主</p>
              </div>
            </div>
            <div v-for="item in overview.paymentMethods" :key="item.name" class="payment-item">
              <span>{{ item.name }}</span>
              <strong>¥{{ formatValue(item.revenue) }}</strong>
            </div>
          </div>
        </div>
      </ElCol>
    </ElRow>

    <ElRow :gutter="16" class="metric-row">
      <ElCol :xs="24" :lg="8">
        <div class="art-card panel-card metric-panel">
          <div class="panel-header">
            <div>
              <h3>代理商运营</h3>
              <p>代理数量、余额和今日消费</p>
            </div>
          </div>
          <div class="metric-grid">
            <div v-for="item in overview.agentMetrics" :key="item.label" class="metric-item">
              <div>
                <p>{{ item.label }}</p>
                <strong>
                  <span v-if="item.prefix">{{ item.prefix }}</span>{{ formatValue(item.value) }}
                  <small>{{ item.unit }}</small>
                </strong>
                <em>{{ item.desc }}</em>
              </div>
            </div>
          </div>
        </div>
      </ElCol>

      <ElCol :xs="24" :lg="8">
        <div class="art-card panel-card metric-panel">
          <div class="panel-header">
            <div>
              <h3>用户数据</h3>
              <p>用户增长、余额和消费</p>
            </div>
          </div>
          <div class="metric-grid">
            <div v-for="item in overview.userMetrics" :key="item.label" class="metric-item">
              <div>
                <p>{{ item.label }}</p>
                <strong>
                  <span v-if="item.prefix">{{ item.prefix }}</span>{{ formatValue(item.value) }}
                  <small>{{ item.unit }}</small>
                </strong>
                <em>{{ item.desc }}</em>
              </div>
            </div>
          </div>
        </div>
      </ElCol>

      <ElCol :xs="24" :lg="8">
        <div class="art-card panel-card metric-panel">
          <div class="panel-header">
            <div>
              <h3>应用产品</h3>
              <p>应用、套餐和授权规模</p>
            </div>
          </div>
          <div class="metric-grid">
            <div v-for="item in overview.appMetrics" :key="item.label" class="metric-item">
              <div>
                <p>{{ item.label }}</p>
                <strong>
                  <span v-if="item.prefix">{{ item.prefix }}</span>{{ formatValue(item.value) }}
                  <small>{{ item.unit }}</small>
                </strong>
                <em>{{ item.desc }}</em>
              </div>
            </div>
          </div>
        </div>
      </ElCol>
    </ElRow>

    <ElRow :gutter="16">
      <ElCol :xs="24" :lg="12">
        <div class="art-card panel-card">
          <div class="panel-header">
            <div>
              <h3>应用销售排行</h3>
              <p>按授权数量排序</p>
            </div>
          </div>
          <div class="rank-list">
            <div v-for="(item, index) in overview.appRanking" :key="item.name" class="rank-item">
              <span class="rank-no">{{ index + 1 }}</span>
              <div class="rank-content">
                <p>{{ item.name || '未命名应用' }}</p>
                <small>{{ item.extra }}</small>
              </div>
              <b>{{ item.value }}</b>
            </div>
            <ElEmpty v-if="overview.appRanking.length === 0" description="暂无应用数据" />
          </div>
        </div>
      </ElCol>

      <ElCol :xs="24" :lg="12">
        <div class="art-card panel-card">
          <div class="panel-header">
            <div>
              <h3>代理商排行</h3>
              <p>按累计开通授权排序</p>
            </div>
          </div>
          <div class="rank-list">
            <div v-for="(item, index) in overview.agentRanking" :key="item.name" class="rank-item">
              <span class="rank-no agent">{{ index + 1 }}</span>
              <div class="rank-content">
                <p>{{ item.name || '未命名代理商' }}</p>
                <small>{{ item.extra }}</small>
              </div>
              <b>{{ item.value }}</b>
            </div>
            <ElEmpty v-if="overview.agentRanking.length === 0" description="暂无代理商数据" />
          </div>
        </div>
      </ElCol>

    </ElRow>

    <ElRow :gutter="16">
      <ElCol :xs="24" :lg="24">
        <div class="art-card panel-card">
          <div class="panel-header">
            <div>
              <h3>最近动态</h3>
              <p>最新授权开通记录</p>
            </div>
          </div>
          <div class="activity-list">
            <div v-for="item in overview.activities" :key="`${item.title}-${item.time}`" class="activity-item">
              <div class="activity-dot"></div>
              <div>
                <p>{{ item.title }}</p>
                <small>{{ item.desc }}</small>
              </div>
              <span>{{ item.time }}</span>
            </div>
            <ElEmpty v-if="overview.activities.length === 0" description="暂无最近动态" />
          </div>
        </div>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, reactive } from 'vue'
  import {
    AdminDashboardOverview,
    fetchAdminDashboardActivities,
    fetchAdminDashboardAgentMetrics,
    fetchAdminDashboardAgentRanking,
    fetchAdminDashboardAppMetrics,
    fetchAdminDashboardAppRanking,
    fetchAdminDashboardCards,
    fetchAdminDashboardLicenseStatus,
    fetchAdminDashboardPaymentMethods,
    fetchAdminDashboardTrend,
    fetchAdminDashboardUserMetrics
  } from '@/api/dashboard'

  defineOptions({ name: 'Console' })

  const overview = reactive<AdminDashboardOverview>({
    cards: [],
    trend: [],
    licenseStatus: [],
    appRanking: [],
    agentRanking: [],
    activities: [],
    paymentMethods: [],
    agentMetrics: [],
    userMetrics: [],
    appMetrics: []
  })

  const trendDates = computed(() => overview.trend.map((item) => item.date))
  const revenueTrend = computed(() => overview.trend.map((item) => item.revenue))

  const formatValue = (value: number) => {
    if (Number.isInteger(value)) return value.toLocaleString()
    return value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  }

  const trendTotals = computed(() => {
    return overview.trend.reduce(
      (totals, item) => ({
        revenue: totals.revenue + item.revenue,
        orders: totals.orders + item.orders,
        licenses: totals.licenses + item.licenses
      }),
      { revenue: 0, orders: 0, licenses: 0 }
    )
  })

  const dashboardModules = [
    async () => {
      overview.cards = await fetchAdminDashboardCards()
    },
    async () => {
      overview.trend = await fetchAdminDashboardTrend()
    },
    async () => {
      overview.licenseStatus = await fetchAdminDashboardLicenseStatus()
    },
    async () => {
      overview.paymentMethods = await fetchAdminDashboardPaymentMethods()
    },
    async () => {
      overview.agentMetrics = await fetchAdminDashboardAgentMetrics()
    },
    async () => {
      overview.userMetrics = await fetchAdminDashboardUserMetrics()
    },
    async () => {
      overview.appMetrics = await fetchAdminDashboardAppMetrics()
    },
    async () => {
      overview.appRanking = await fetchAdminDashboardAppRanking()
    },
    async () => {
      overview.agentRanking = await fetchAdminDashboardAgentRanking()
    },
    async () => {
      overview.activities = await fetchAdminDashboardActivities()
    }
  ]

  const loadDashboard = async () => {
    await Promise.allSettled(dashboardModules.map((loadModule) => loadModule()))
  }

  onMounted(() => {
    loadDashboard()
  })
</script>

<style lang="scss" scoped>
  .admin-dashboard {
    .card-row {
      margin-bottom: 16px;
    }

    .stat-card,
    .panel-card {
      margin-bottom: 16px;
      padding: 20px;
    }

    .stat-card {
      min-height: 132px;
    }

    .stat-main {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
    }

    .stat-title {
      font-size: 14px;
      color: var(--art-gray-600);
    }

    .stat-value {
      margin-top: 10px;
      font-size: 26px;
      font-weight: 700;
      color: var(--art-gray-900);
    }

    .stat-unit {
      margin-left: 4px;
      font-size: 13px;
      font-weight: 400;
      color: var(--art-gray-500);
    }

    .stat-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      font-size: 22px;
      color: var(--art-primary);
      background: rgba(var(--art-primary-rgb), 0.1);
      border-radius: 14px;
    }

    .stat-footer {
      display: flex;
      gap: 6px;
      margin-top: 16px;
      font-size: 12px;
      color: var(--art-gray-500);
    }

    .panel-card {
      min-height: 360px;
    }

    .metric-panel {
      min-height: 300px;
    }

    .metric-row {
      margin-top: 16px;
    }

    .trend-status-row {
      align-items: stretch;
    }

    .trend-status-col {
      display: flex;
    }

    .trend-status-col .panel-card {
      width: 100%;
      height: 100%;
    }

    .trend-panel,
    .status-panel {
      min-height: 470px;
    }

    .panel-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 18px;

      h3 {
        margin: 0;
        font-size: 17px;
        font-weight: 700;
      }

      p {
        margin-top: 6px;
        font-size: 13px;
        color: var(--art-gray-500);
      }

      &.compact {
        margin-top: 24px;
        margin-bottom: 10px;
      }
    }

    .trend-header {
      gap: 20px;
      align-items: flex-start;
    }

    .trend-summary {
      display: grid;
      grid-template-columns: repeat(3, minmax(104px, 1fr));
      gap: 10px;
      min-width: 360px;
    }

    .summary-item {
      padding: 10px 12px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 12px;

      span {
        display: block;
        font-size: 12px;
        color: var(--art-gray-500);
      }

      strong {
        display: block;
        margin-top: 6px;
        font-size: 18px;
        line-height: 1.2;
        color: var(--art-gray-900);
      }

      &.revenue strong {
        color: var(--art-primary);
      }
    }

    .trend-extra {
      display: grid;
      grid-template-columns: repeat(7, minmax(0, 1fr));
      gap: 10px;
      margin-top: 16px;
      padding-top: 16px;
      border-top: 1px solid var(--art-border-color);
    }

    .trend-extra-item {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      justify-content: space-between;
      gap: 8px;
      min-width: 0;
      min-height: 92px;
      padding: 12px;
      background: var(--art-gray-100);
      border: 1px solid transparent;
      border-radius: 12px;
      transition:
        border-color 0.2s ease,
        background-color 0.2s ease;

      &:hover {
        background: rgba(var(--art-primary-rgb), 0.06);
        border-color: rgba(var(--art-primary-rgb), 0.16);
      }

      .trend-date {
        font-size: 12px;
        color: var(--art-gray-500);
      }

      .trend-revenue {
        overflow: hidden;
        font-size: 17px;
        font-weight: 700;
        color: var(--art-gray-900);
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .trend-meta {
        display: flex;
        align-items: center;
        gap: 6px;
        max-width: 100%;
        overflow: hidden;
        font-size: 12px;
        white-space: nowrap;

        em {
          color: var(--art-gray-500);
          font-style: normal;
        }

        i {
          width: 3px;
          height: 3px;
          background: var(--art-gray-400);
          border-radius: 50%;
        }
      }
    }

    .status-list,
    .rank-list,
    .activity-list {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .metric-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }

    .metric-item {
      position: relative;
      min-height: 112px;
      padding: 14px 14px 14px 18px;
      overflow: hidden;
      background: var(--art-gray-100);
      border-radius: 14px;

      p {
        font-size: 13px;
        color: var(--art-gray-500);
      }

      strong {
        display: block;
        margin-top: 8px;
        font-size: 22px;
        color: var(--art-gray-900);
      }

      small {
        margin-left: 4px;
        font-size: 12px;
        font-weight: 400;
        color: var(--art-gray-500);
      }

      em {
        display: block;
        margin-top: 8px;
        font-size: 12px;
        font-style: normal;
        color: var(--art-gray-500);
      }
    }

    .status-item,
    .payment-item,
    .rank-item,
    .activity-item {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 0;
      border-bottom: 1px solid var(--art-border-color);
    }

    .status-item > div,
    .activity-item {
      justify-content: flex-start;
      gap: 12px;
    }

    .status-dot,
    .activity-dot {
      display: inline-block;
      width: 9px;
      height: 9px;
      margin-right: 8px;
      border-radius: 50%;
      background: var(--art-primary);
    }

    .status-expiring {
      background: var(--el-color-warning);
    }

    .status-expired,
    .status-revoked {
      background: var(--el-color-danger);
    }

    .payment-box {
      margin-top: 10px;
    }

    .rank-no {
      width: 28px;
      height: 28px;
      line-height: 28px;
      text-align: center;
      color: #fff;
      background: var(--art-primary);
      border-radius: 9px;

      &.agent {
        background: var(--el-color-success);
      }
    }

    .rank-content {
      flex: 1;
      min-width: 0;
      margin: 0 12px;

      p {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      small {
        color: var(--art-gray-500);
      }
    }

    .activity-item {
      justify-content: flex-start;

      > div:nth-child(2) {
        flex: 1;
      }

      small,
      span {
        color: var(--art-gray-500);
      }
    }
  }

  @media (max-width: 768px) {
    .admin-dashboard {
      .trend-header {
        flex-direction: column;
      }

      .trend-summary {
        width: 100%;
        min-width: 0;
        grid-template-columns: 1fr;
      }

      .trend-extra {
        grid-template-columns: repeat(2, 1fr);
      }

      .metric-grid {
        grid-template-columns: 1fr;
      }
    }
  }
</style>
