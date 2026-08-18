import { AppRouteRecord } from '@/types/router'

export const sdkRoutes: AppRouteRecord = {
  path: '/sdk',
  name: 'Sdk',
  component: '/index/index',
  meta: {
    title: 'SDK 接入',
    icon: 'ri:code-box-line',
    roles: ['R_SUPER', 'R_ADMIN']
  },
  children: [
    {
      path: 'index',
      name: 'SdkIndex',
      component: '/sdk/index',
      meta: {
        title: 'SDK 示例',
        icon: 'ri:code-s-slash-line',
        keepAlive: true,
        roles: ['R_SUPER', 'R_ADMIN']
      }
    },
    {
      path: 'developer-doc',
      name: 'DeveloperDoc',
      component: '/sdk/developer-doc',
      meta: {
        title: 'menus.system.developerDoc',
        icon: 'ri:file-code-line',
        keepAlive: true,
        roles: ['R_SUPER', 'R_ADMIN']
      }
    }
  ]
}