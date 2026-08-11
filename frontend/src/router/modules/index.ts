import { AppRouteRecord } from '@/types/router'
import { dashboardRoutes } from './dashboard'
import { systemRoutes } from './system'
import { userManageRoutes } from './user-manage'
import { orderListRoutes } from './order-list'
import { resultRoutes } from './result'
import { exceptionRoutes } from './exception'
import { licenseRoutes } from './license'
import { agentRoutes } from './agent'
import { piracyRoutes } from './piracy'
import { sdkRoutes } from './sdk'
import { pluginStoreRoutes } from './plugin-store'
import { onlineUpdateRoutes } from './online-update'

/**
 * 导出所有模块化路由
 */
export const routeModules: AppRouteRecord[] = [
  dashboardRoutes,
  userManageRoutes,
  orderListRoutes,
  systemRoutes,
  licenseRoutes,
  agentRoutes,
  piracyRoutes,
  resultRoutes,
  exceptionRoutes,
  sdkRoutes,
  pluginStoreRoutes,
  onlineUpdateRoutes
]
