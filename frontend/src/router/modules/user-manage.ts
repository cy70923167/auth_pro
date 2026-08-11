import { AppRouteRecord } from '@/types/router'

export const userManageRoutes: AppRouteRecord = {
  path: '/user-manage',
  name: 'User',
  component: '/system/user',
  meta: {
    title: 'menus.system.user',
    icon: 'ri:user-line',
    keepAlive: true,
    roles: ['R_SUPER', 'R_ADMIN']
  }
}