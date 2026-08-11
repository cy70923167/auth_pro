import { AppRouteRecord } from '@/types/router'

export const piracyRoutes: AppRouteRecord = {
  path: '/piracy',
  name: 'Piracy',
  component: '/index/index',
  meta: {
    title: '反盗版',
    icon: 'ri:shield-cross-line',
    roles: ['R_SUPER', 'R_ADMIN']
  },
  children: [
    {
      path: 'tracking',
      name: 'PiracyTracking',
      component: '/piracy/tracking',
      meta: {
        title: '盗版追踪',
        icon: 'ri:spy-line',
        keepAlive: true
      }
    },
    {
      path: 'blacklist',
      name: 'PiracyBlacklist',
      component: '/piracy/blacklist',
      meta: {
        title: '黑名单管理',
        icon: 'ri:forbid-line',
        keepAlive: true
      }
    },
    {
      path: 'alerts',
      name: 'PiracyAlerts',
      component: '/piracy/alerts',
      meta: {
        title: '告警中心',
        icon: 'ri:alarm-warning-line',
        keepAlive: false
      }
    },
    {
      path: 'reports',
      name: 'PiracyReports',
      component: '/piracy/reports',
      meta: {
        title: '数据报表',
        icon: 'ri:bar-chart-box-line',
        keepAlive: false
      }
    }
  ]
}