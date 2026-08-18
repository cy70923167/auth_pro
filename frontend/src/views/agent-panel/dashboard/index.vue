<template>
  <div class="panel-dashboard">
    <!-- 统计卡片 -->
    <ElRow :gutter="20" class="flex">
      <ElCol v-for="(item, index) in cardList" :key="index" :sm="12" :md="6" :lg="6">
        <div class="art-card relative flex flex-col justify-center h-35 px-5 mb-5 max-sm:mb-4">
          <span class="text-g-700 text-sm">{{ item.label }}</span>
          <ArtCountTo class="text-[26px] font-medium mt-2" :target="item.value" :duration="1300" />
          <div class="flex-c mt-1">
            <span class="text-xs text-g-600">较上周</span>
            <span
              class="ml-1 text-xs font-semibold"
              :class="[item.change.indexOf('+') === -1 ? 'text-danger' : 'text-success']"
            >
              {{ item.change }}
            </span>
          </div>
          <div class="absolute top-0 bottom-0 right-5 m-auto size-12.5 rounded-xl flex-cc bg-theme/10">
            <ArtSvgIcon :icon="item.icon" class="text-xl text-theme" />
          </div>
        </div>
      </ElCol>
    </ElRow>

    <!-- 图表行 -->
    <ElRow :gutter="20">
      <ElCol :sm="24" :md="12" :lg="14">
        <div class="art-card h-105 p-5 mb-5 max-sm:mb-4">
          <div class="art-card-header">
            <div class="title">
              <h4>授权趋势</h4>
              <p>本月新增 <span class="text-success">+23%</span></p>
            </div>
          </div>
          <ArtLineChart
            height="calc(100% - 56px)"
            :data="lineChartData"
            :xAxisData="lineChartXAxis"
            :showAreaColor="true"
            :showAxisLine="false"
          />
        </div>
      </ElCol>
      <ElCol :sm="24" :md="12" :lg="10">
        <div class="art-card h-105 p-5 mb-5 max-sm:mb-4">
          <div class="art-card-header">
            <div class="title">
              <h4>应用分布</h4>
              <p>按授权数量</p>
            </div>
          </div>
          <ArtBarChart
            height="calc(100% - 56px)"
            :data="barChartData"
            :xAxisData="barChartXAxis"
          />
        </div>
      </ElCol>
    </ElRow>

    <!-- 信息 + 最近开码 -->
    <ElRow :gutter="20">
      <ElCol :sm="24" :md="24" :lg="8">
        <div class="art-card p-5 mb-5 max-sm:mb-4">
          <div class="art-card-header mb-3">
            <div class="title"><h4>我的信息</h4></div>
          </div>
          <div class="info-list">
            <div class="info-item"><span class="info-label">代理商</span><span>{{ agentInfo.name }}</span></div>
            <div class="info-item"><span class="info-label">等级</span><el-tag type="warning" size="small">{{ agentInfo.levelName }}</el-tag></div>
            <div class="info-item"><span class="info-label">折扣</span><span>{{ agentInfo.discount }}</span></div>
            <div class="info-item"><span class="info-label">联系方式</span><span>{{ agentInfo.contact }}</span></div>
            <div class="info-item"><span class="info-label">注册时间</span><span>{{ agentInfo.createdAt }}</span></div>
          </div>
        </div>
      </ElCol>
      <ElCol :sm="24" :md="24" :lg="16">
        <div class="art-card p-5 mb-5 max-sm:mb-4">
          <div class="art-card-header mb-3">
            <div class="title"><h4>最近开码</h4></div>
          </div>
          <el-table :data="recentLicenses" stripe size="small">
            <el-table-column prop="domain" label="域名/IP/密钥" min-width="180" show-overflow-tooltip />
            <el-table-column prop="appName" label="应用" width="100" />
            <el-table-column prop="typeLabel" label="类型" width="80">
              <template #default="{ row }">
                <el-tag size="small">{{ row.typeLabel }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="expireAt" label="到期时间" width="130" />
            <el-table-column prop="createdAt" label="开通时间" width="130" />
          </el-table>
        </div>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import axios from 'axios'
import ArtCountTo from '@/components/core/text-effect/art-count-to/index.vue'
import ArtSvgIcon from '@/components/core/base/art-svg-icon/index.vue'
import ArtLineChart from '@/components/core/charts/art-line-chart/index.vue'
import ArtBarChart from '@/components/core/charts/art-bar-chart/index.vue'

function headers() {
  return { Authorization: `Bearer ${localStorage.getItem('agent_panel_token') || ''}` }
}

const cardList = reactive([
  { label: '账户余额(元)', icon: 'ri:money-cny-circle-line', value: 0, change: '+0%' },
  { label: '有效授权', icon: 'ri:shield-check-line', value: 0, change: '+0%' },
  { label: '剩余配额', icon: 'ri:key-2-line', value: 0, change: '' },
  { label: '即将到期', icon: 'ri:time-line', value: 0, change: '' }
])

const lineChartData = ref<number[]>([])
const lineChartXAxis = ref<string[]>([])

const barChartData = ref<number[]>([])
const barChartXAxis = ref<string[]>([])

const agentInfo = reactive({
  name: '-',
  levelName: '-',
  level: '',
  discount: '-',
  contact: '-',
  createdAt: '-'
})

interface RecentLicense {
  domain: string
  appName: string
  typeLabel: string
  expireAt: string
  createdAt: string
}
const recentLicenses = ref<RecentLicense[]>([])

async function fetchStats() {
  try {
    const { data } = await axios.get('/api/agent-panel/dashboard/stats', { headers: headers() })
    if (data.code === 200) {
      cardList[0].value = data.data.balance ?? 0
      cardList[1].value = data.data.activeLicenses ?? 0
      cardList[2].value = data.data.quotaRemain ?? 0
      cardList[3].value = data.data.expiringSoon ?? 0
      cardList[1].change = data.data.weekChange || '+0%'
    }
  } catch { /* 静默失败，保持默认值 */ }
}

async function fetchInfo() {
  try {
    const { data } = await axios.get('/api/agent-panel/dashboard/info', { headers: headers() })
    if (data.code === 200) {
      Object.assign(agentInfo, data.data)
    }
  } catch { /* 静默失败 */ }
}

async function fetchTrend() {
  try {
    const { data } = await axios.get('/api/agent-panel/dashboard/trend', { headers: headers() })
    if (data.code === 200 && Array.isArray(data.data)) {
      lineChartXAxis.value = data.data.map((i: any) => {
        const [, m] = (i.month || '').split('-')
        return `${Number(m)}月`
      })
      lineChartData.value = data.data.map((i: any) => i.count)
    }
  } catch { /* 静默失败 */ }
}

async function fetchAppDist() {
  try {
    const { data } = await axios.get('/api/agent-panel/dashboard/app-dist', { headers: headers() })
    if (data.code === 200 && Array.isArray(data.data)) {
      barChartXAxis.value = data.data.map((i: any) => i.name)
      barChartData.value = data.data.map((i: any) => i.count)
    }
  } catch { /* 静默失败 */ }
}

async function fetchRecent() {
  try {
    const { data } = await axios.get('/api/agent-panel/dashboard/recent-licenses', { headers: headers() })
    if (data.code === 200 && Array.isArray(data.data)) {
      recentLicenses.value = data.data
    }
  } catch { /* 静默失败 */ }
}

onMounted(() => {
  fetchStats()
  fetchInfo()
  fetchTrend()
  fetchAppDist()
  fetchRecent()
})
</script>

<style scoped lang="scss">
.panel-dashboard {
  background: var(--el-bg-color);

  .art-card {
    overflow: hidden;
    background: var(--el-bg-color);
    border-radius: 12px !important;
  }
}

.info-list {
  .info-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 0;
    font-size: 14px;
    border-bottom: 1px solid var(--el-border-color-lighter);
    &:last-child { border-bottom: none; }
  }
  .info-label { color: var(--el-text-color-secondary); }
}
</style>