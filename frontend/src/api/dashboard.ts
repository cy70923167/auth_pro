import request from '@/utils/http'

export interface DashboardCard {
  title: string
  value: number
  unit: string
  icon: string
  trend: number
  prefix: string
}

export interface DashboardTrendItem {
  date: string
  revenue: number
  orders: number
  licenses: number
}

export interface DashboardStatusItem {
  name: string
  value: number
  type: string
}

export interface DashboardRankItem {
  name: string
  value: number
  revenue: number
  extra: string
}

export interface DashboardTodoItem {
  title: string
  value: number
  level: string
  desc: string
}

export interface DashboardActivityItem {
  title: string
  desc: string
  time: string
  type: string
}

export interface DashboardMetricItem {
  label: string
  value: number
  unit: string
  prefix: string
  desc: string
  level: string
}

export interface AdminDashboardOverview {
  cards: DashboardCard[]
  trend: DashboardTrendItem[]
  licenseStatus: DashboardStatusItem[]
  appRanking: DashboardRankItem[]
  agentRanking: DashboardRankItem[]
  todos?: DashboardTodoItem[]
  activities: DashboardActivityItem[]
  paymentMethods: DashboardRankItem[]
  agentMetrics: DashboardMetricItem[]
  userMetrics: DashboardMetricItem[]
  appMetrics: DashboardMetricItem[]
  riskAlerts?: DashboardTodoItem[]
}

export function fetchAdminDashboardOverview() {
  return request.get<AdminDashboardOverview>({
    url: '/api/dashboard/overview'
  })
}

export function fetchAdminDashboardCards() {
  return request.get<DashboardCard[]>({
    url: '/api/dashboard/cards'
  })
}

export function fetchAdminDashboardTrend() {
  return request.get<DashboardTrendItem[]>({
    url: '/api/dashboard/trend'
  })
}

export function fetchAdminDashboardLicenseStatus() {
  return request.get<DashboardStatusItem[]>({
    url: '/api/dashboard/license-status'
  })
}

export function fetchAdminDashboardPaymentMethods() {
  return request.get<DashboardRankItem[]>({
    url: '/api/dashboard/payment-methods'
  })
}

export function fetchAdminDashboardAgentMetrics() {
  return request.get<DashboardMetricItem[]>({
    url: '/api/dashboard/agent-metrics'
  })
}

export function fetchAdminDashboardUserMetrics() {
  return request.get<DashboardMetricItem[]>({
    url: '/api/dashboard/user-metrics'
  })
}

export function fetchAdminDashboardAppMetrics() {
  return request.get<DashboardMetricItem[]>({
    url: '/api/dashboard/app-metrics'
  })
}

export function fetchAdminDashboardAppRanking() {
  return request.get<DashboardRankItem[]>({
    url: '/api/dashboard/app-ranking'
  })
}

export function fetchAdminDashboardAgentRanking() {
  return request.get<DashboardRankItem[]>({
    url: '/api/dashboard/agent-ranking'
  })
}

export function fetchAdminDashboardActivities() {
  return request.get<DashboardActivityItem[]>({
    url: '/api/dashboard/activities'
  })
}
