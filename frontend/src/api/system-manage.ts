import request from '@/utils/http'
import { AppRouteRecord } from '@/types/router'

export type EpayPayType = 'alipay' | 'wxpay' | 'qqpay'

export interface SystemConfigData {
  siteName: string
  siteSubtitle: string
  siteLogo: string
  installedAt: string
  stationQQ: string
  icpNumber: string
  domainLicenseNotice: string
  registrationEnabled: boolean
  selfPurchaseEnabled: boolean
  piracyDetectionEnabled: boolean
}

export interface PaymentConfigData {
  easypayEnabled: boolean
  easypayGateway: string
  easypayPid: string
  easypayMerchantKey?: string
  easypayMerchantKeySet: boolean
  easypayDefaultType: EpayPayType
  easypayPayTypes: EpayPayType[]
  easypayNotifyUrl: string
  easypayReturnUrl: string
}

export interface PaymentV2ConfigData {
  easypayEnabled: boolean
  easypayGateway: string
  easypayPid: string
  easypayMerchantKey?: string
  easypayMerchantKeySet: boolean
  easypayPlatformKey?: string
  easypayPlatformKeySet: boolean
  easypayDefaultType: EpayPayType
  easypayPayTypes: EpayPayType[]
  easypayNotifyUrl: string
  easypayReturnUrl: string
}

export type SystemFeatureSwitchKey =
  | 'registrationEnabled'
  | 'selfPurchaseEnabled'
  | 'piracyDetectionEnabled'

export interface SystemFeatureSwitchData {
  key: SystemFeatureSwitchKey
  enabled: boolean
}

export interface RealnameAppItem {
  id: number
  appName: string
  appKey: string
}

export type RealnameProvider = 'alipay' | 'kuaitong' | 'tencent' | 'xiaomu'
export type KuaitongAuthType = 'face' | 'two_element'
export type XiaomuProductMode = 'three_element' | 'face_h5' | 'tencent_h5'

export interface RealnameConfigData {
  enabled: boolean
  pluginEnabled: boolean
  provider: RealnameProvider
  appId: string
  privateKeySet: boolean
  alipayPublicKey: string
  gateway: string
  kuaitongAccessKey: string
  kuaitongSecretSet: boolean
  kuaitongAuthType: KuaitongAuthType
  tencentApiKey: string
  tencentSecretSet: boolean
  tencentBaseUrl: string
  tencentUsePackage: boolean
  tencentProductCode: string
  xiaomuAppKey: string
  xiaomuSecretSet: boolean
  xiaomuBaseUrl: string
  xiaomuProductMode: XiaomuProductMode
  requireAppIds: number[]
  apps: RealnameAppItem[]
}

export interface UpdateRealnameConfigParams {
  enabled: boolean
  provider: RealnameProvider
  appId: string
  privateKey?: string
  alipayPublicKey: string
  gateway: string
  kuaitongAccessKey: string
  kuaitongSecret?: string
  kuaitongAuthType: KuaitongAuthType
  tencentApiKey: string
  tencentApiSecret?: string
  tencentBaseUrl: string
  tencentUsePackage: boolean
  xiaomuAppKey: string
  xiaomuAppSecret?: string
  xiaomuBaseUrl: string
  xiaomuProductMode: XiaomuProductMode
  requireAppIds: number[]
}

export interface MailConfigData {
  provider: 'qq' | 'aliyun' | 'custom'
  smtpHost: string
  smtpPort: number
  smtpSecure: 'ssl' | 'starttls' | 'none'
  smtpUsername: string
  smtpPassword?: string
  passwordSet: boolean
  smtpFromEmail: string
  smtpFromName: string
  enabledPurchaseSuccess: boolean
  enabledExpireReminder: boolean
  enabledLicenseOpened: boolean
  expireRemindDays: string
  purchaseSubject: string
  purchaseContent: string
  purchaseContentType: 'text' | 'html'
  expireSubject: string
  expireContent: string
  expireContentType: 'text' | 'html'
  openedSubject: string
  openedContent: string
  openedContentType: 'text' | 'html'
}

export interface MailContentTypeUpdateData {
  template: 'purchase' | 'expire' | 'opened'
  contentType: 'text' | 'html'
  content: string
}

export interface MailLogSearchParams {
  page: number
  pageSize: number
  eventType?: string
  status?: string
  keyword?: string
}

export interface MailLogItem {
  id: number
  eventType: string
  targetType: string
  targetId?: number
  licenseId?: number
  recipient: string
  subject: string
  content: string
  status: string
  error: string
  remindDays?: number
  createdAt: string
  sentAt: string
}

// 获取用户列表
export function fetchGetUserList(params: Api.SystemManage.UserSearchParams) {
  return request.get<Api.SystemManage.UserList>({
    url: '/api/user/list',
    params
  })
}

// 删除用户
export function fetchDeleteUser(id: number) {
  return request.del<null>({
    url: `/api/user/${id}`,
    showSuccessMessage: true
  })
}

// 获取角色列表
export function fetchGetRoleList(params: Api.SystemManage.RoleSearchParams) {
  return request.get<Api.SystemManage.RoleList>({
    url: '/api/role/list',
    params
  })
}

// 创建角色
export function fetchCreateRole(data: any) {
  return request.post<any>({
    url: '/api/role/create',
    data,
    showSuccessMessage: true
  })
}

// 更新角色
export function fetchUpdateRole(id: number, data: any) {
  return request.put<any>({
    url: `/api/role/${id}`,
    data,
    showSuccessMessage: true
  })
}

// 删除角色
export function fetchDeleteRole(id: number) {
  return request.del<any>({
    url: `/api/role/${id}`,
    showSuccessMessage: true
  })
}

// 获取角色菜单权限
export function fetchRoleMenus(id: number) {
  return request.get<any>({
    url: `/api/role/${id}/menus`
  })
}

// 更新角色菜单权限
export function fetchUpdateRoleMenus(id: number, menuIds: number[]) {
  return request.put<any>({
    url: `/api/role/${id}/menus`,
    data: { menuIds },
    showSuccessMessage: true
  })
}

// 获取菜单列表（侧边栏动态路由用）
export function fetchGetMenuList() {
  return request.get<AppRouteRecord[]>({
    url: '/api/system/menus'
  })
}

// 获取菜单管理列表（全量树形）
export function fetchMenuManageList() {
  return request.get<any>({
    url: '/api/menu/list'
  })
}

// 创建菜单
export function fetchCreateMenu(data: any) {
  return request.post<any>({
    url: '/api/menu/create',
    data,
    showSuccessMessage: true
  })
}

// 更新菜单
export function fetchUpdateMenu(id: number, data: any) {
  return request.put<any>({
    url: `/api/menu/${id}`,
    data,
    showSuccessMessage: true
  })
}

// 删除菜单
export function fetchDeleteMenu(id: number) {
  return request.del<any>({
    url: `/api/menu/${id}`,
    showSuccessMessage: true
  })
}

export function fetchPublicSystemConfig() {
  return request.get<SystemConfigData>({
    url: '/api/system-config/public',
    params: { _t: Date.now() },
    showErrorMessage: false
  })
}

export function fetchSystemConfig() {
  return request.get<SystemConfigData>({
    url: '/api/system/config'
  })
}

export function fetchUpdateSystemConfig(data: SystemConfigData) {
  return request.put<SystemConfigData>({
    url: '/api/system/config',
    data
  })
}

export function fetchPaymentConfig() {
  return request.get<PaymentConfigData>({
    url: '/api/system/payment-config'
  })
}

export function fetchUpdatePaymentConfig(data: PaymentConfigData) {
  return request.put<PaymentConfigData>({
    url: '/api/system/payment-config',
    data
  })
}

export interface PaymentTestOrderData {
  orderNo: string
  amount: number | string
  payType: EpayPayType
  payUrl?: string
  status?: 'pending' | 'paid' | 'failed' | 'cancelled'
  paidAt?: string
  gatewayTradeNo?: string
}

// 发起易支付测试订单，支付成功后只记录结果，不入账。
export function fetchCreatePaymentTest(data: { amount: number; payType: EpayPayType }) {
  return request.post<PaymentTestOrderData>({
    url: '/api/system/payment-config/test',
    data
  })
}

export function fetchPaymentTestStatus(orderNo: string) {
  return request.get<PaymentTestOrderData>({
    url: `/api/system/payment-config/test/${encodeURIComponent(orderNo)}`,
    showErrorMessage: false
  })
}

// ========== 易支付 V2 ==========

export function fetchPaymentV2Config() {
  return request.get<PaymentV2ConfigData>({
    url: '/api/system/payment-v2-config'
  })
}

export function fetchUpdatePaymentV2Config(data: PaymentV2ConfigData) {
  return request.put<PaymentV2ConfigData>({
    url: '/api/system/payment-v2-config',
    data
  })
}

export function fetchCreatePaymentV2Test(data: { amount: number; payType: EpayPayType }) {
  return request.post<PaymentTestOrderData>({
    url: '/api/system/payment-v2-config/test',
    data
  })
}

export function fetchPaymentV2TestStatus(orderNo: string) {
  return request.get<PaymentTestOrderData>({
    url: `/api/system/payment-v2-config/test/${encodeURIComponent(orderNo)}`,
    showErrorMessage: false
  })
}

export type PaymentOrderSubjectType = 'user' | 'agent' | 'test'
export type PaymentOrderStatus = 'pending' | 'paid' | 'failed' | 'cancelled'

export interface PaymentOrderSearchParams {
  page: number
  pageSize: number
  orderNo?: string
  subjectType?: string
  status?: string
  payMethod?: string
}

export interface PaymentOrderItem {
  orderNo: string
  subjectType: PaymentOrderSubjectType
  subjectId: number
  subjectName: string
  amount: number
  paidAmount: number
  payChannel: string
  payMethod: string
  status: PaymentOrderStatus
  gatewayTradeNo: string
  remark: string
  createdAt: string
  paidAt: string
}

export function fetchPaymentOrderList(params: PaymentOrderSearchParams) {
  return request.get<{ list: PaymentOrderItem[]; total: number; page: number; pageSize: number }>({
    url: '/api/system/payment-orders',
    params
  })
}

// 独立保存用户端功能开关，避免提交其他未保存配置。
export function fetchUpdateSystemFeatureSwitch(key: SystemFeatureSwitchKey, enabled: boolean) {
  return request.put<SystemFeatureSwitchData>({
    url: `/api/system/config/switch/${key}`,
    data: { enabled },
    showErrorMessage: false,
    showSuccessMessage: false
  })
}

// 获取实名认证配置
export function fetchRealnameConfig() {
  return request.get<RealnameConfigData>({
    url: '/api/system/realname-config'
  })
}

// 更新实名认证配置
export function fetchUpdateRealnameConfig(data: UpdateRealnameConfigParams) {
  return request.put<RealnameConfigData>({
    url: '/api/system/realname-config',
    data,
    showSuccessMessage: true
  })
}

export interface RealnameRecordItem {
  id: number
  ownerType: string
  ownerId: number
  ownerName: string
  ownerEmail: string
  provider: string
  realName: string
  idCard: string
  status: 'passed' | 'failed'
  failReason: string
  serialNo: string
  score: string
  createdAt: string
}

export interface RealnameRecordListResult {
  list: RealnameRecordItem[]
  total: number
  page: number
  pageSize: number
}

// 实名认证记录列表（展示层脱敏，由后端处理）
export function fetchRealnameRecords(params: {
  page: number
  pageSize: number
  keyword?: string
  provider?: string
  status?: 'passed' | 'failed'
}) {
  return request.get<RealnameRecordListResult>({
    url: '/api/system/realname-records',
    params
  })
}

export function fetchMailConfig() {
  return request.get<MailConfigData>({
    url: '/api/system/mail-config'
  })
}

export function fetchUpdateMailConfig(data: MailConfigData) {
  return request.put<MailConfigData>({
    url: '/api/system/mail-config',
    data
  })
}

export function fetchUpdateMailContentType(data: MailContentTypeUpdateData) {
  return request.put<{ message?: string }>({
    url: '/api/system/mail-config/content-type',
    data
  })
}

export function fetchTestMailConfig(recipient: string) {
  return request.post<any>({
    url: '/api/system/mail-config/test',
    data: { recipient },
    showSuccessMessage: true
  })
}

export function fetchMailLogList(params: MailLogSearchParams) {
  return request.get<{ list: MailLogItem[]; total: number; page: number; pageSize: number }>({
    url: '/api/system/mail-logs',
    params
  })
}

export function fetchMailLogDetail(id: number) {
  return request.get<MailLogItem>({
    url: `/api/system/mail-logs/${id}`
  })
}

// ========== 应用商店（插件中心） ==========

export interface PluginInfo {
  id: string
  category: string
  name: string
  description: string
  homepage: string
  icon: string
  version: string
  official: boolean
  enabled: boolean
  configured: boolean
  local: boolean
  remote: boolean
  source: string
  downloadUrl: string
}

export interface PluginCategoryGroup {
  category: string
  title: string
  plugins: PluginInfo[]
}

export interface PluginSource {
  id: number
  name: string
  url: string
  state: 'ok' | 'error' | 'unknown'
}

export interface PluginListData {
  categories: PluginCategoryGroup[]
  sources: PluginSource[]
}

export interface PluginListParams {
  source?: string
  q?: string
}

export function fetchPluginList(params?: PluginListParams) {
  return request.get<PluginListData>({
    url: '/api/system/plugins',
    params
  })
}

export function fetchAddPluginSource(name: string, url: string) {
  return request.post<null>({
    url: '/api/system/plugin-sources',
    data: { name, url }
  })
}

export function fetchDeletePluginSource(id: number) {
  return request.del<null>({
    url: `/api/system/plugin-sources/${id}`
  })
}

export function fetchDownloadPlugin(id: string) {
  return request.post<null>({
    url: `/api/system/plugins/${id}/download`
  })
}

export function fetchTogglePlugin(id: string, enabled: boolean) {
  return request.post<null>({
    url: `/api/system/plugins/${id}/toggle`,
    data: { enabled }
  })
}
