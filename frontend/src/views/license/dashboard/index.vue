<template>
  <div class="license-dashboard">
    <!-- 统计卡片 -->
    <el-row :gutter="16" class="mb-4">
      <el-col :xs="12" :sm="6" v-for="item in statsCards" :key="item.title">
        <el-card shadow="hover" class="stats-card">
          <div class="stats-card-inner">
            <div class="stats-info">
              <span class="stats-title">{{ item.title }}</span>
              <span class="stats-value">{{ item.value }}</span>
            </div>
            <div class="stats-icon" :style="{ backgroundColor: item.bgColor }">
              <el-icon :size="24" :color="item.color">
                <component :is="item.icon" />
              </el-icon>
            </div>
          </div>
          <div class="stats-footer">
            <span :class="item.trend > 0 ? 'trend-up' : 'trend-down'">
              {{ item.trend > 0 ? '+' : '' }}{{ item.trend }}%
            </span>
            <span class="trend-label">较昨日</span>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近授权 + 到期提醒 -->
    <el-row :gutter="16">
      <el-col :xs="24" :lg="14">
        <el-card shadow="hover">
          <template #header>
            <span class="card-title">最近授权</span>
          </template>
          <el-table :data="recentLicenses" stripe>
            <el-table-column prop="domain" label="域名/IP" min-width="180" />
            <el-table-column prop="appName" label="应用" width="120" />
            <el-table-column prop="type" label="类型" width="100">
              <template #default="{ row }">
                <el-tag :type="typeTagMap[row.type]" size="small">{{ row.typeLabel }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="createdAt" label="授权时间" width="160" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="10">
        <el-card shadow="hover">
          <template #header>
            <span class="card-title">即将到期</span>
          </template>
          <div class="expire-list">
            <div v-for="item in expiringSoon" :key="item.id" class="expire-item">
              <div class="expire-info">
                <span class="expire-domain">{{ item.domain }}</span>
                <span class="expire-app">{{ item.appName }}</span>
              </div>
              <el-tag type="warning" size="small">{{ item.daysLeft }}天后到期</el-tag>
            </div>
            <el-empty v-if="expiringSoon.length === 0" description="暂无即将到期的授权" />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/utils/http'

interface StatsItem {
  title: string
  value: number
  trend: number
  icon: string
  color: string
  bgColor: string
}

interface RecentLicense {
  domain: string
  appName: string
  type: string
  typeLabel: string
  createdAt: string
}

interface ExpireItem {
  id: number
  domain: string
  appName: string
  daysLeft: number
}

interface DashboardData {
  stats: { title: string; value: number; trend: number }[]
  recentLicenses: RecentLicense[]
  expiringSoon: ExpireItem[]
}

const iconMap: Record<string, { icon: string; color: string; bgColor: string }> = {
  '总授权数': { icon: 'ri-shield-keyhole-line', color: '#409eff', bgColor: '#ecf5ff' },
  '活跃授权': { icon: 'ri-check-double-line', color: '#67c23a', bgColor: '#f0f9eb' },
  '已过期': { icon: 'ri-time-line', color: '#e6a23c', bgColor: '#fdf6ec' },
  '今日验证': { icon: 'ri-radar-line', color: '#909399', bgColor: '#f4f4f5' }
}

const statsCards = ref<StatsItem[]>([])
const recentLicenses = ref<RecentLicense[]>([])
const expiringSoon = ref<ExpireItem[]>([])

const typeTagMap: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined> = {
  domain: undefined,
  wildcard: 'success',
  ip: 'warning',
  key: 'info'
} as const

const fetchDashboard = async () => {
  try {
    const data = await request.get<DashboardData>({ url: '/api/license/dashboard' })
    statsCards.value = data.stats.map(s => ({
      ...s,
      ...iconMap[s.title] || { icon: 'ri-shield-keyhole-line', color: '#409eff', bgColor: '#ecf5ff' }
    }))
    recentLicenses.value = data.recentLicenses
    expiringSoon.value = data.expiringSoon
  } catch (e) {
    console.error('[LicenseDashboard] 加载失败:', e)
  }
}

onMounted(() => {
  fetchDashboard()
})
</script>

<style scoped lang="scss">
.license-dashboard {
  padding: 0;
}

.stats-card {
  .stats-card-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .stats-info {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .stats-title {
    font-size: 14px;
    color: var(--el-text-color-secondary);
  }

  .stats-value {
    font-size: 28px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  .stats-icon {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .stats-footer {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--el-border-color-lighter);
    font-size: 13px;

    .trend-up {
      color: #67c23a;
      font-weight: 500;
    }

    .trend-down {
      color: #f56c6c;
      font-weight: 500;
    }

    .trend-label {
      color: var(--el-text-color-secondary);
      margin-left: 4px;
    }
  }
}

.mb-4 {
  margin-bottom: 16px;
}

.card-title {
  font-weight: 600;
}

.expire-list {
  .expire-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--el-border-color-lighter);

    &:last-child {
      border-bottom: none;
    }
  }

  .expire-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .expire-domain {
    font-size: 14px;
    font-weight: 500;
  }

  .expire-app {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
}
</style>