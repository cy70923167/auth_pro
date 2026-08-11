import { AppRouteRecord } from '@/types/router'

export const licenseRoutes: AppRouteRecord = {
  path: '/license',
  name: 'License',
  component: '/index/index',
  meta: {
    title: '授权管理',
    icon: 'ri:shield-keyhole-line',
    roles: ['R_SUPER', 'R_ADMIN']
  },
  children: [
    {
      path: 'dashboard',
      name: 'LicenseDashboard',
      component: '/license/dashboard',
      meta: {
        title: '授权概览',
        icon: 'ri:dashboard-line',
        keepAlive: false
      }
    },
    {
      path: 'list',
      name: 'LicenseList',
      component: '/license/list',
      meta: {
        title: '授权列表',
        icon: 'ri:file-list-3-line',
        keepAlive: true
      }
    },
    {
      path: 'apps',
      name: 'LicenseApps',
      component: '/license/apps',
      meta: {
        title: '应用管理',
        icon: 'ri:apps-line',
        keepAlive: true
      }
    },
    {
      path: 'apps/:id/versions',
      name: 'AppVersions',
      component: '/license/app-versions',
      meta: {
        title: '版本管理',
        icon: 'ri:git-branch-line',
        isHide: true,
        keepAlive: false,
        activePath: '/license/apps'
      }
    },
    {
      path: 'plans',
      name: 'LicensePlans',
      component: '/license/plans',
      meta: {
        title: '套餐管理',
        icon: 'ri:price-tag-3-line',
        keepAlive: true
      }
    },
    {
      path: 'cards',
      name: 'LicenseCards',
      component: '/license/cards',
      meta: {
        title: '卡密管理',
        icon: 'ri:coupon-3-line',
        keepAlive: true
      }
    },
    {
      path: 'logs',
      name: 'LicenseLogs',
      component: '/license/logs',
      meta: {
        title: '验证日志',
        icon: 'ri:file-text-line',
        keepAlive: true
      }
    }
  ]
}
