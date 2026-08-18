import { AppRouteRecord } from '@/types/router'

export const onlineUpdateRoutes: AppRouteRecord = {
  path: '/online-update',
  name: 'OnlineUpdate',
  component: '/online-update/index',
  meta: {
    title: 'menus.onlineUpdate',
    icon: 'ri:download-cloud-2-line',
    keepAlive: true,
    roles: ['R_SUPER']
  }
}
