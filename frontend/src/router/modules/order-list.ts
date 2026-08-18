import { AppRouteRecord } from '@/types/router'

export const orderListRoutes: AppRouteRecord = {
  path: '/order-list',
  name: 'OrderList',
  component: '/system/payment-orders',
  meta: {
    title: 'menus.system.paymentOrders',
    icon: 'ri:file-list-3-line',
    keepAlive: true,
    roles: ['R_SUPER', 'R_ADMIN']
  }
}
