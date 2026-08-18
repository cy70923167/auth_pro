<template>
  <div class="app-versions-page">
    <header class="page-header">
      <div class="header-main">
        <ElTooltip content="返回应用管理" placement="bottom">
          <ElButton class="back-button" circle aria-label="返回应用管理" @click="goBack">
            <ArtSvgIcon icon="ri:arrow-left-line" />
          </ElButton>
        </ElTooltip>
        <div class="header-copy">
          <div class="title-line">
            <h1>{{ appInfo?.name || '版本管理' }}</h1>
            <ElTag v-if="latestVersion" type="success" effect="plain">
              最新版本 {{ latestVersion }}
            </ElTag>
          </div>
          <p v-if="appInfo">{{ appInfo.appKey }}</p>
        </div>
      </div>
      <ElButton type="primary" :disabled="!appInfo" @click="openCreateDialog">
        <ArtSvgIcon icon="ri:add-line" />
        发布版本
      </ElButton>
    </header>

    <ElCard shadow="never" class="version-table-card">
      <ElTable v-loading="loading" :data="versions" stripe>
        <ElTableColumn label="版本号" min-width="130">
          <template #default="{ row }">
            <span class="version-number">{{ row.version }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="title" label="更新标题" min-width="190" show-overflow-tooltip />
        <ElTableColumn label="更新包" min-width="190">
          <template #default="{ row }">
            <div class="package-cell">
              <span class="package-name">{{ row.packageName || '外部下载地址' }}</span>
              <span class="package-meta">
                {{ row.sourceType === 'upload' ? '本地上传' : '外部 URL' }} ·
                {{ formatFileSize(row.fileSizeBytes) }}
              </span>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn label="更新策略" min-width="150">
          <template #default="{ row }">
            <div class="policy-tags">
              <ElTag v-if="row.forceUpdate" type="danger" size="small">强制更新</ElTag>
              <ElTag v-else type="info" size="small">可选更新</ElTag>
              <ElTag v-if="row.minVersion" type="warning" size="small" effect="plain">
                低于 {{ row.minVersion }} 强更
              </ElTag>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="publishedAt" label="发布时间" width="170" />
        <ElTableColumn label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <ElButton link type="primary" size="small" @click="openDetail(row)">详情</ElButton>
            <ElButton link type="primary" size="small" @click="openEditDialog(row)">编辑</ElButton>
            <ElButton link type="primary" size="small" @click="downloadPackage(row)">
              下载
            </ElButton>
            <ElButton link type="danger" size="small" @click="handleDelete(row)">删除</ElButton>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty description="暂无版本记录">
            <ElButton type="primary" @click="openCreateDialog">发布首个版本</ElButton>
          </ElEmpty>
        </template>
      </ElTable>

      <div v-if="pagination.total > pagination.pageSize" class="pagination-wrap">
        <ElPagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          @size-change="loadVersions"
          @current-change="loadVersions"
        />
      </div>
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? '编辑版本' : '发布版本'"
      width="760px"
      destroy-on-close
      class="version-dialog"
      @closed="resetForm"
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
        <ElRow :gutter="16">
          <ElCol :xs="24" :md="8">
            <ElFormItem label="版本号" prop="version">
              <ElInput v-model.trim="form.version" maxlength="50" placeholder="例如 1.2.0" />
            </ElFormItem>
          </ElCol>
          <ElCol :xs="24" :md="8">
            <ElFormItem label="更新标题" prop="title">
              <ElInput
                v-model.trim="form.title"
                maxlength="200"
                show-word-limit
                placeholder="例如 性能优化与问题修复"
              />
            </ElFormItem>
          </ElCol>
        </ElRow>

        <ElFormItem label="更新日志" prop="changelog">
          <ElInput
            v-model="form.changelog"
            type="textarea"
            :rows="6"
            maxlength="20000"
            show-word-limit
            placeholder="请输入本次版本的更新内容"
          />
        </ElFormItem>

        <ElFormItem label="更新 SQL">
          <ElInput
            v-model="form.updateSql"
            class="sql-input"
            type="textarea"
            :rows="6"
            maxlength="100000"
            placeholder="无数据库变更时可留空"
          />
        </ElFormItem>

        <ElFormItem label="更新包来源">
          <ElSegmented
            v-model="form.sourceType"
            :options="sourceOptions"
            @change="handleSourceChange"
          />
        </ElFormItem>

        <div v-if="form.sourceType === 'upload'" class="package-source-panel">
          <ElUpload
            ref="uploadRef"
            drag
            :auto-upload="false"
            :limit="1"
            :file-list="fileList"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
            :on-exceed="handleFileExceed"
          >
            <ArtSvgIcon icon="ri:upload-cloud-2-line" class="upload-icon" />
            <div class="upload-title">选择更新包</div>
            <template #tip>
              <div class="upload-tip"> 单个文件不超过 512MB，服务端将自动计算文件大小与 MD5 </div>
            </template>
          </ElUpload>
          <ElAlert
            v-if="editingId && currentPackageName && !selectedFile"
            type="info"
            :closable="false"
            show-icon
            :title="`当前更新包：${currentPackageName}`"
          />
        </div>

        <ElRow v-else :gutter="16" class="package-source-panel">
          <ElCol :xs="24">
            <ElFormItem label="下载地址" prop="downloadUrl">
              <ElInput
                v-model.trim="form.downloadUrl"
                maxlength="2048"
                placeholder="https://example.com/releases/app-1.2.0.zip"
              />
            </ElFormItem>
          </ElCol>
          <ElCol :xs="24" :md="8">
            <ElFormItem label="文件大小（MB）" prop="fileSizeMb">
              <ElInputNumber
                v-model="form.fileSizeMb"
                :min="0.001"
                :max="512"
                :precision="3"
                :step="1"
                controls-position="right"
              />
            </ElFormItem>
          </ElCol>
          <ElCol :xs="24" :md="16">
            <ElFormItem label="文件 MD5" prop="fileMd5">
              <ElInput
                v-model.trim="form.fileMd5"
                maxlength="32"
                class="mono-input"
                placeholder="32 位十六进制字符串"
              />
            </ElFormItem>
          </ElCol>
        </ElRow>

        <div class="update-policy-panel">
          <ElRow :gutter="16">
            <ElCol :xs="24" :md="12">
              <ElFormItem label="强制更新">
                <ElSwitch v-model="form.forceUpdate" active-text="开启" inactive-text="关闭" />
              </ElFormItem>
            </ElCol>
            <ElCol :xs="24" :md="12">
              <ElFormItem label="最低版本" prop="minVersion">
                <ElInput
                  v-model.trim="form.minVersion"
                  maxlength="50"
                  clearable
                  placeholder="低于此版本时强制更新"
                />
              </ElFormItem>
            </ElCol>
          </ElRow>
        </div>
      </ElForm>

      <template #footer>
        <ElButton :disabled="submitting" @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitVersion">
          {{ editingId ? '保存' : '发布' }}
        </ElButton>
      </template>
    </ElDialog>

    <ElDrawer v-model="detailVisible" title="版本详情" size="min(640px, 92vw)">
      <template v-if="detailVersion">
        <ElDescriptions :column="1" border>
          <ElDescriptionsItem label="版本号">{{ detailVersion.version }}</ElDescriptionsItem>
          <ElDescriptionsItem label="更新标题">{{ detailVersion.title }}</ElDescriptionsItem>
          <ElDescriptionsItem label="更新策略">
            <ElTag :type="detailVersion.forceUpdate ? 'danger' : 'info'" size="small">
              {{ detailVersion.forceUpdate ? '强制更新' : '可选更新' }}
            </ElTag>
            <span v-if="detailVersion.minVersion" class="description-inline">
              低于 {{ detailVersion.minVersion }} 时强制更新
            </span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="文件大小">
            {{ formatFileSize(detailVersion.fileSizeBytes) }}
          </ElDescriptionsItem>
          <ElDescriptionsItem label="文件 MD5">
            <span class="mono-text break-text">{{ detailVersion.fileMd5 }}</span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="下载地址">
            <span class="break-text">
              {{
                detailVersion.sourceType === 'upload'
                  ? '点击下载时生成临时地址'
                  : detailVersion.downloadUrl
              }}
            </span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="发布时间">{{ detailVersion.publishedAt }}</ElDescriptionsItem>
        </ElDescriptions>

        <section class="detail-section">
          <h2>更新日志</h2>
          <pre>{{ detailVersion.changelog }}</pre>
        </section>
        <section class="detail-section">
          <h2>更新 SQL</h2>
          <pre class="sql-block">{{ detailVersion.updateSql || '无' }}</pre>
        </section>

        <div class="drawer-actions">
          <ElButton type="primary" @click="downloadPackage(detailVersion)">
            <ArtSvgIcon icon="ri:download-line" />
            下载更新包
          </ElButton>
        </div>
      </template>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import { onMounted, reactive, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import type {
    FormInstance,
    FormRules,
    UploadFile,
    UploadFiles,
    UploadInstance,
    UploadRawFile
  } from 'element-plus'
  import { ElMessage, ElMessageBox, genFileId } from 'element-plus'
  import ArtSvgIcon from '@/components/core/base/art-svg-icon/index.vue'
  import {
    createAppVersion,
    createAppVersionDownloadUrl,
    deleteAppVersion,
    fetchAppVersions,
    updateAppVersion,
    type AppVersionApp,
    type AppVersionItem,
    type AppVersionSourceType
  } from '@/api/app-version'

  defineOptions({ name: 'AppVersions' })

  interface VersionForm {
    version: string
    title: string
    changelog: string
    updateSql: string
    sourceType: AppVersionSourceType
    downloadUrl: string
    fileSizeMb: number
    fileMd5: string
    forceUpdate: boolean
    minVersion: string
  }

  const route = useRoute()
  const router = useRouter()
  const appId = Number(route.params.id)
  const loading = ref(false)
  const submitting = ref(false)
  const appInfo = ref<AppVersionApp>()
  const versions = ref<AppVersionItem[]>([])
  const dialogVisible = ref(false)
  const detailVisible = ref(false)
  const detailVersion = ref<AppVersionItem>()
  const editingId = ref(0)
  const editingRevision = ref(0)
  const currentPackageName = ref('')
  const reusableUploadedPackage = ref(false)
  const selectedFile = ref<File>()
  const fileList = ref<UploadFiles>([])
  const formRef = ref<FormInstance>()
  const uploadRef = ref<UploadInstance>()

  const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
  const latestVersion = ref('')
  const form = reactive<VersionForm>({
    version: '',
    title: '',
    changelog: '',
    updateSql: '',
    sourceType: 'upload',
    downloadUrl: '',
    fileSizeMb: 0.001,
    fileMd5: '',
    forceUpdate: false,
    minVersion: ''
  })

  const versionPattern =
    /^[vV]?\d+(?:[._]\d+)*(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$/
  const sourceOptions = [
    { label: '上传更新包', value: 'upload' },
    { label: '填写下载地址', value: 'url' }
  ]
  const validateVersion = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
    if (!versionPattern.test(value || '')) {
      callback(new Error('请输入有效版本号，例如 1.2.0'))
      return
    }
    callback()
  }

  const validateOptionalVersion = (
    _rule: unknown,
    value: string,
    callback: (error?: Error) => void
  ) => {
    if (value && !versionPattern.test(value)) {
      callback(new Error('请输入有效版本号，例如 1.0.0'))
      return
    }
    callback()
  }

  const rules: FormRules<VersionForm> = {
    version: [
      { required: true, message: '请输入版本号', trigger: 'blur' },
      { validator: validateVersion, trigger: 'blur' }
    ],
    title: [{ required: true, message: '请输入更新标题', trigger: 'blur' }],
    changelog: [{ required: true, message: '请输入更新日志', trigger: 'blur' }],
    downloadUrl: [{ type: 'url', message: '请输入有效下载地址', trigger: 'blur' }],
    fileMd5: [
      { pattern: /^[0-9a-fA-F]{32}$/, message: '请输入 32 位十六进制 MD5', trigger: 'blur' }
    ],
    minVersion: [{ validator: validateOptionalVersion, trigger: 'blur' }]
  }

  async function loadVersions() {
    if (!Number.isInteger(appId) || appId <= 0) {
      ElMessage.error('应用参数不正确')
      goBack()
      return
    }
    loading.value = true
    try {
      const data = await fetchAppVersions(appId, pagination.page, pagination.pageSize)
      appInfo.value = data.app
      versions.value = data.list || []
      pagination.total = Number(data.total || 0)
      latestVersion.value = data.latestVersion || ''
    } finally {
      loading.value = false
    }
  }

  function goBack() {
    router.push('/license/apps')
  }

  function openCreateDialog() {
    resetForm()
    dialogVisible.value = true
  }

  function openEditDialog(row: AppVersionItem) {
    resetForm()
    editingId.value = row.id
    editingRevision.value = row.revision
    currentPackageName.value = row.packageName
    reusableUploadedPackage.value = row.sourceType === 'upload'
    Object.assign(form, {
      version: row.version,
      title: row.title,
      changelog: row.changelog,
      updateSql: row.updateSql,
      sourceType: row.sourceType,
      downloadUrl: row.sourceType === 'url' ? row.downloadUrl : '',
      fileSizeMb: Math.max(Number(row.fileSizeMb || 0), 0.001),
      fileMd5: row.fileMd5,
      forceUpdate: row.forceUpdate,
      minVersion: row.minVersion
    })
    dialogVisible.value = true
  }

  function resetForm() {
    editingId.value = 0
    editingRevision.value = 0
    currentPackageName.value = ''
    reusableUploadedPackage.value = false
    selectedFile.value = undefined
    fileList.value = []
    Object.assign(form, {
      version: '',
      title: '',
      changelog: '',
      updateSql: '',
      sourceType: 'upload',
      downloadUrl: '',
      fileSizeMb: 0.001,
      fileMd5: '',
      forceUpdate: false,
      minVersion: ''
    })
    formRef.value?.clearValidate()
    uploadRef.value?.clearFiles()
  }

  function handleSourceChange() {
    selectedFile.value = undefined
    fileList.value = []
    uploadRef.value?.clearFiles()
    formRef.value?.clearValidate(['downloadUrl', 'fileSizeMb', 'fileMd5'])
  }

  function handleFileChange(uploadFile: UploadFile, uploadFiles: UploadFiles) {
    if (!uploadFile.raw) return
    if (uploadFile.raw.size > 512 * 1024 * 1024) {
      ElMessage.error('更新包不能超过 512MB')
      selectedFile.value = undefined
      fileList.value = []
      uploadRef.value?.clearFiles()
      return
    }
    selectedFile.value = uploadFile.raw
    fileList.value = uploadFiles.slice(-1)
  }

  function handleFileRemove() {
    selectedFile.value = undefined
    fileList.value = []
  }

  function handleFileExceed(files: File[]) {
    uploadRef.value?.clearFiles()
    const file = files[0] as UploadRawFile | undefined
    if (!file) return
    file.uid = genFileId()
    uploadRef.value?.handleStart(file)
  }

  function buildPayload() {
    const payload = new FormData()
    payload.append('version', form.version.trim())
    payload.append('title', form.title.trim())
    payload.append('changelog', form.changelog.trim())
    payload.append('updateSql', form.updateSql.trim())
    payload.append('sourceType', form.sourceType)
    payload.append('forceUpdate', String(form.forceUpdate))
    payload.append('minVersion', form.minVersion.trim())
    if (editingId.value) payload.append('revision', String(editingRevision.value))
    if (form.sourceType === 'upload' && selectedFile.value) {
      payload.append('package', selectedFile.value)
    }
    if (form.sourceType === 'url') {
      payload.append('downloadUrl', form.downloadUrl.trim())
      payload.append('fileSizeMb', String(form.fileSizeMb))
      payload.append('fileMd5', form.fileMd5.trim().toLowerCase())
    }
    return payload
  }

  async function submitVersion() {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (form.sourceType === 'upload' && !selectedFile.value && !reusableUploadedPackage.value) {
      ElMessage.warning('请选择更新包')
      return
    }
    if (form.sourceType === 'url') {
      if (!form.downloadUrl || form.fileSizeMb <= 0 || !/^[0-9a-fA-F]{32}$/.test(form.fileMd5)) {
        ElMessage.warning('请完整填写下载地址、文件大小和文件 MD5')
        return
      }
    }

    submitting.value = true
    try {
      const payload = buildPayload()
      if (editingId.value) {
        await updateAppVersion(appId, editingId.value, payload)
        ElMessage.success('版本更新成功')
      } else {
        await createAppVersion(appId, payload)
        ElMessage.success('版本发布成功')
      }
      dialogVisible.value = false
      pagination.page = 1
      await loadVersions()
    } finally {
      submitting.value = false
    }
  }

  function openDetail(row: AppVersionItem) {
    detailVersion.value = row
    detailVisible.value = true
  }

  function absoluteDownloadUrl(downloadUrl: string) {
    try {
      return new URL(downloadUrl, window.location.origin).toString()
    } catch {
      return downloadUrl
    }
  }

  async function downloadPackage(row: AppVersionItem) {
    if (row.sourceType === 'url') {
      window.open(absoluteDownloadUrl(row.downloadUrl), '_blank', 'noopener,noreferrer')
      return
    }

    const downloadWindow = window.open('', '_blank')
    try {
      const data = await createAppVersionDownloadUrl(appId, row.id)
      const target = absoluteDownloadUrl(data.downloadUrl)
      if (downloadWindow) {
        downloadWindow.opener = null
        downloadWindow.location.href = target
      } else {
        window.location.href = target
      }
    } catch {
      downloadWindow?.close()
    }
  }

  async function handleDelete(row: AppVersionItem) {
    try {
      await ElMessageBox.confirm(
        `确定删除版本「${row.version}」？本地上传的更新包也会被删除。`,
        '删除版本',
        { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' }
      )
      await deleteAppVersion(appId, row.id)
      ElMessage.success('版本删除成功')
      if (versions.value.length === 1 && pagination.page > 1) pagination.page -= 1
      await loadVersions()
    } catch {
      // 用户取消删除时保持当前列表。
    }
  }

  function formatFileSize(bytes: number) {
    const value = Number(bytes || 0)
    if (value < 1024) return `${value} B`
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(2)} KB`
    return `${(value / 1024 / 1024).toFixed(2)} MB`
  }

  onMounted(loadVersions)
</script>

<style scoped lang="scss">
  .app-versions-page {
    min-width: 0;
  }

  .page-header {
    display: flex;
    gap: 20px;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  .header-main,
  .title-line,
  .package-cell,
  .policy-tags {
    display: flex;
    align-items: center;
  }

  .header-main {
    gap: 12px;
    min-width: 0;
  }

  .back-button {
    flex-shrink: 0;
  }

  .header-copy {
    min-width: 0;

    h1 {
      margin: 0;
      font-size: 22px;
      line-height: 1.35;
      color: var(--art-gray-900);
    }

    p {
      margin: 3px 0 0;
      overflow: hidden;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 12px;
      color: var(--art-gray-600);
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .title-line {
    flex-wrap: wrap;
    gap: 10px;
    min-width: 0;
  }

  .version-table-card {
    border-radius: 8px;
  }

  .version-number,
  .mono-text,
  :deep(.mono-input input),
  :deep(.sql-input textarea) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .version-number {
    font-weight: 650;
    color: var(--el-color-primary);
  }

  .package-cell {
    flex-direction: column;
    align-items: flex-start;
    min-width: 0;
  }

  .package-name {
    max-width: 100%;
    overflow: hidden;
    color: var(--art-gray-900);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .package-meta {
    margin-top: 3px;
    font-size: 12px;
    color: var(--art-gray-600);
  }

  .policy-tags {
    flex-direction: column;
    gap: 6px;
    align-items: flex-start;
  }

  .pagination-wrap {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .package-source-panel,
  .update-policy-panel {
    padding: 16px;
    margin-bottom: 18px;
    background: var(--art-gray-100);
    border: 1px solid var(--art-border-color);
    border-radius: 8px;
  }

  .package-source-panel :deep(.el-upload) {
    width: 100%;
  }

  .package-source-panel :deep(.el-upload-dragger) {
    width: 100%;
    padding: 24px;
  }

  .package-source-panel :deep(.el-alert) {
    margin-top: 12px;
  }

  .upload-icon {
    font-size: 34px;
    color: var(--el-color-primary);
  }

  .upload-title {
    margin-top: 6px;
    font-size: 14px;
    font-weight: 600;
    color: var(--art-gray-900);
  }

  .upload-tip {
    font-size: 12px;
    color: var(--art-gray-600);
  }

  .update-policy-panel {
    padding-bottom: 0;
  }

  .description-inline {
    margin-left: 8px;
    color: var(--art-gray-700);
  }

  .break-text {
    overflow-wrap: anywhere;
  }

  .detail-section {
    margin-top: 24px;

    h2 {
      margin: 0 0 10px;
      font-size: 15px;
      color: var(--art-gray-900);
    }

    pre {
      min-height: 72px;
      padding: 14px;
      margin: 0;
      overflow: auto;
      font-family: inherit;
      font-size: 13px;
      line-height: 1.7;
      color: var(--art-gray-800);
      overflow-wrap: anywhere;
      white-space: pre-wrap;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 8px;
    }

    .sql-block {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }
  }

  .drawer-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 20px;
  }

  @media (width <= 767px) {
    .page-header {
      flex-direction: column;
      align-items: flex-start;
    }

    .page-header > .el-button {
      width: 100%;
    }

    .header-copy h1 {
      font-size: 19px;
    }

    .pagination-wrap {
      justify-content: flex-start;
      overflow-x: auto;
    }

    :global(.version-dialog) {
      width: calc(100vw - 24px) !important;
      max-height: calc(100vh - 24px);
      margin-top: 12px !important;
      overflow-y: auto;
    }
  }
</style>
