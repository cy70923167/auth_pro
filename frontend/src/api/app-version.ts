import request from '@/utils/http'

export type AppVersionSourceType = 'upload' | 'url'

export interface AppVersionApp {
  id: number
  name: string
  appKey: string
}

export interface AppVersionItem {
  id: number
  appId: number
  version: string
  title: string
  changelog: string
  updateSql: string
  packageName: string
  sourceType: AppVersionSourceType
  downloadUrl: string
  fileSizeBytes: number
  fileSizeMb: number
  fileMd5: string
  forceUpdate: boolean
  minVersion: string
  revision: number
  publishedAt: string
  updatedAt: string
}

export interface AppVersionListData {
  app: AppVersionApp
  list: AppVersionItem[]
  total: number
  latestVersion: string
}

export function fetchAppVersions(appId: number, page: number, pageSize: number) {
  return request.get<AppVersionListData>({
    url: `/api/app/${appId}/versions`,
    params: { page, pageSize }
  })
}

export function createAppVersion(appId: number, data: FormData) {
  return request.post<{ id: number }>({
    url: `/api/app/${appId}/versions`,
    data,
    timeout: 10 * 60 * 1000
  })
}

export function updateAppVersion(appId: number, versionId: number, data: FormData) {
  return request.put<void>({
    url: `/api/app/${appId}/versions/${versionId}`,
    data,
    timeout: 10 * 60 * 1000
  })
}

export function deleteAppVersion(appId: number, versionId: number) {
  return request.del<void>({
    url: `/api/app/${appId}/versions/${versionId}`
  })
}

export function createAppVersionDownloadUrl(appId: number, versionId: number) {
  return request.post<{ downloadUrl: string; expiresIn: number }>({
    url: `/api/app/${appId}/versions/${versionId}/download-url`
  })
}
