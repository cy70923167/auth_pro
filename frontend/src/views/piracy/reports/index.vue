<template>
  <div class="data-reports">
    <!-- 时间范围选择 -->
    <el-card shadow="hover" class="mb-4">
      <div class="filter-bar">
        <el-radio-group v-model="timeRange" @change="handleTimeChange">
          <el-radio-button value="7d">近7天</el-radio-button>
          <el-radio-button value="30d">近30天</el-radio-button>
          <el-radio-button value="90d">近90天</el-radio-button>
        </el-radio-group>
        <div class="filter-right">
          <el-date-picker
            v-model="customRange"
            type="daterange"
            start-placeholder="开始"
            end-placeholder="结束"
            style="width: 240px"
            @change="handleCustomRange"
          />
          <el-dropdown trigger="click" @command="handleExport">
            <el-button type="primary" plain>
              导出报表 <el-icon class="el-icon--right"><i class="ri-download-line" /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="excel">导出 Excel</el-dropdown-item>
                <el-dropdown-item command="pdf">导出 PDF</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-card>

    <!-- 趋势图 -->
    <el-row :gutter="16" class="mb-4">
      <el-col :xs="24" :lg="12">
        <el-card shadow="hover">
          <template #header><span class="card-title">盗版趋势</span></template>
          <div class="chart-container" ref="piracyChartRef"></div>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card shadow="hover">
          <template #header><span class="card-title">验证通过率</span></template>
          <div class="chart-container" ref="verifyChartRef"></div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 收入概览 + 代理商排行 -->
    <el-row :gutter="16" class="mb-4">
      <el-col :xs="24" :lg="14">
        <el-card shadow="hover">
          <template #header><span class="card-title">收入报表</span></template>
          <div class="chart-container" ref="revenueChartRef"></div>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="10">
        <el-card shadow="hover">
          <template #header><span class="card-title">代理商业绩排行</span></template>
          <div class="rank-list">
            <div v-for="(item, idx) in agentRank" :key="item.id" class="rank-item">
              <div class="rank-left">
                <span class="rank-badge" :class="'rank-' + (idx + 1)">{{ idx + 1 }}</span>
                <span class="rank-name">{{ item.name }}</span>
              </div>
              <div class="rank-right">
                <span class="rank-count">{{ item.licenses }} 个</span>
                <span class="rank-amount">¥{{ item.amount.toLocaleString() }}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 应用数据明细 -->
    <el-card shadow="hover">
      <template #header><span class="card-title">应用数据明细</span></template>
      <el-table :data="appStatsData" stripe>
        <el-table-column prop="appName" label="应用" min-width="120" />
        <el-table-column prop="totalLicenses" label="总授权数" width="100" align="center" />
        <el-table-column prop="activeLicenses" label="有效授权" width="100" align="center" />
        <el-table-column prop="verifyCount" label="验证次数" width="110" align="center" />
        <el-table-column prop="passRate" label="通过率" width="90" align="center">
          <template #default="{ row }">
            <span :class="row.passRate >= 95 ? 'text-success' : 'text-warning'">{{ row.passRate }}%</span>
          </template>
        </el-table-column>
        <el-table-column prop="piracyCount" label="盗版次数" width="100" align="center">
          <template #default="{ row }">
            <span class="text-danger">{{ row.piracyCount }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="revenue" label="收入(元)" width="120" align="right">
          <template #default="{ row }">
            <span class="text-primary font-bold">¥{{ row.revenue.toLocaleString() }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import request from '@/utils/http'

echarts.use([LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const timeRange = ref('30d')
const customRange = ref([])

const piracyChartRef = ref<HTMLElement>()
const verifyChartRef = ref<HTMLElement>()
const revenueChartRef = ref<HTMLElement>()

const agentRank = ref<any[]>([])
const appStatsData = ref<any[]>([])

let piracyChart: any = null
let verifyChart: any = null
let revenueChart: any = null

async function loadData() {
  const daysMap: Record<string, string> = { '7d': '7', '30d': '30', '90d': '90' }
  const data = await request.get<any>({ url: '/api/piracy/report/overview', params: { days: daysMap[timeRange.value] || '30' } })

  agentRank.value = data.agentRank || []
  appStatsData.value = data.appStats || []

  nextTick(() => {
    renderPiracyChart(data.piracyTrend)
    renderVerifyChart(data.verifyTrend)
    renderRevenueChart(data.revenueTrend)
  })
}

function renderPiracyChart(trend: any) {
  if (!piracyChartRef.value) return
  if (!piracyChart) piracyChart = echarts.init(piracyChartRef.value)
  piracyChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['盗版请求', '新增案例'] },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: trend.dates || [] },
    yAxis: [
      { type: 'value', name: '请求数' },
      { type: 'value', name: '案例数', max: 10 }
    ],
    series: [
      { name: '盗版请求', type: 'line', smooth: true, data: trend.piracyRequests || [], itemStyle: { color: '#f56c6c' } },
      { name: '新增案例', type: 'bar', yAxisIndex: 1, data: trend.newCases || [], itemStyle: { color: '#e6a23c' } }
    ]
  })
}

function renderVerifyChart(trend: any) {
  if (!verifyChartRef.value) return
  if (!verifyChart) verifyChart = echarts.init(verifyChartRef.value)
  verifyChart.setOption({
    tooltip: { trigger: 'axis', formatter: '{b}<br/>{a}: {c}%' },
    grid: { left: 50, right: 20, top: 30, bottom: 30 },
    xAxis: { type: 'category', data: trend.dates || [] },
    yAxis: { type: 'value', min: 85, max: 100, axisLabel: { formatter: '{value}%' } },
    series: [
      { name: '通过率', type: 'line', smooth: true, data: trend.passRates || [], areaStyle: { opacity: 0.15 }, itemStyle: { color: '#67c23a' } }
    ]
  })
}

function renderRevenueChart(trend: any) {
  if (!revenueChartRef.value) return
  if (!revenueChart) revenueChart = echarts.init(revenueChartRef.value)
  revenueChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['充值', '消费'] },
    grid: { left: 60, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: trend.dates || [] },
    yAxis: { type: 'value', axisLabel: { formatter: '¥{value}' } },
    series: [
      { name: '充值', type: 'bar', stack: 'total', data: trend.recharges || [], itemStyle: { color: '#67c23a' } },
      { name: '消费', type: 'bar', stack: 'total', data: trend.consumes || [], itemStyle: { color: '#409eff' } }
    ]
  })
}

function handleTimeChange() {
  loadData()
}

function handleCustomRange() {
  if (customRange.value && customRange.value.length === 2) {
    loadData()
  }
}

function handleExport(format: string) {
  ElMessage.success(`正在生成 ${format.toUpperCase()} 报表...`)
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">
.mb-4 { margin-bottom: 16px; }
.card-title { font-weight: 600; }
.chart-container { height: 280px; width: 100%; }

.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}
.filter-right { display: flex; align-items: center; gap: 12px; }

.stats-card {
  text-align: center;
  .stats-title { font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 8px; }
  .stats-value { font-size: 24px; font-weight: 700; }
}

.rank-list {
  .rank-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--el-border-color-lighter);
    &:last-child { border-bottom: none; }
  }
  .rank-left { display: flex; align-items: center; gap: 12px; }
  .rank-right { display: flex; align-items: center; gap: 16px; }
  .rank-badge {
    width: 24px; height: 24px; border-radius: 6px;
    display: flex; align-items: center; justify-content: center;
    font-size: 12px; font-weight: 700; color: #fff;
    background: var(--el-text-color-secondary);
    &.rank-1 { background: #f5a623; }
    &.rank-2 { background: #b8b8b8; }
    &.rank-3 { background: #cd7f32; }
  }
  .rank-name { font-size: 14px; font-weight: 500; }
  .rank-count { font-size: 13px; color: var(--el-text-color-secondary); }
  .rank-amount { font-size: 14px; font-weight: 600; color: var(--el-color-primary); }
}

.text-success { color: #67c23a; }
.text-warning { color: #e6a23c; }
.text-danger { color: #f56c6c; }
.text-primary { color: var(--el-color-primary); }
.font-bold { font-weight: 600; }
</style>