<template>
  <div class="online-update">
    <ElCard shadow="never" class="art-table-card">
      <div class="update-header">
        <div>
          <h2 class="update-title">在线更新</h2>
          <p class="update-subtitle">整包更新前端静态文件和 Go 服务</p>
        </div>
        <div class="update-actions">
          <ElButton
            :icon="Refresh"
            :loading="loading || historyLoading"
            circle
            @click="loadPage(true)"
          />
          <ElButton :icon="Search" :loading="checking" @click="handleCheck">检查更新</ElButton>
          <ElButton
            type="primary"
            :icon="Download"
            :disabled="!canApply"
            :loading="applying"
            @click="handleApply"
          >
            立即更新
          </ElButton>
        </div>
      </div>

      <ElAlert
        v-if="isInsecureUpdateUrl"
        title="更新源未启用 HTTPS"
        type="warning"
        show-icon
        :closable="false"
        class="update-alert"
      />
      <ElAlert
        v-if="packageError"
        :title="packageError"
        type="warning"
        show-icon
        :closable="false"
        class="update-alert"
      />
      <ElAlert
        v-if="versionError"
        :title="versionError"
        type="error"
        show-icon
        :closable="false"
        class="update-alert"
      />
      <ElAlert
        v-if="job?.status === 'restarting'"
        title="服务正在重启，页面稍后可能短暂无法访问"
        type="success"
        show-icon
        :closable="false"
        class="update-alert"
      />

      <div v-if="job" ref="jobSectionRef" class="update-section job-section">
        <div class="section-header">
          <strong>更新任务</strong>
          <ElTag :type="jobStatusTag(job.status)" effect="plain">
            {{ jobStatusText(job.status) }}
          </ElTag>
        </div>
        <div class="job-progress">
          <ElProgress :percentage="jobProgress" :status="jobProgressStatus" :stroke-width="10" />
          <div class="job-progress-meta">
            <span>{{ job.message }}</span>
            <span>目标版本 v{{ job.version }}</span>
          </div>
        </div>
        <ElScrollbar max-height="220px" class="job-logs">
          <div v-for="log in job.logs" :key="log" class="job-log">{{ log }}</div>
          <ElEmpty v-if="!job.logs.length" description="暂无日志" :image-size="60" />
        </ElScrollbar>
      </div>

      <div class="version-grid">
        <div class="version-item">
          <span class="version-label">当前版本</span>
          <div class="version-value">
            <strong>v{{ currentVersion }}</strong>
            <ElTag type="info" effect="plain">当前</ElTag>
          </div>
          <span class="version-meta">{{ status?.buildTime || '未记录构建时间' }}</span>
        </div>

        <div class="version-item latest" :class="{ available: updateAvailable }">
          <span class="version-label">最新版本</span>
          <div class="version-value">
            <strong>{{ latestVersion }}</strong>
            <ElTag :type="updateAvailable ? 'success' : 'info'" effect="plain">
              {{ updateAvailable ? '可更新' : '已是最新' }}
            </ElTag>
          </div>
          <span class="version-meta">{{ formatDate(latest?.releasedAt) }}</span>
        </div>

        <div class="version-item">
          <span class="version-label">更新通道</span>
          <div class="version-value plain">
            <strong>{{ latest?.channel || 'stable' }}</strong>
          </div>
          <span class="version-meta">{{ status?.serviceName || 'auth_pro' }}</span>
        </div>
      </div>

      <div class="update-section">
        <div class="section-header">
          <strong>更新内容</strong>
          <ElText v-if="latest?.force" type="danger" size="small">强制更新</ElText>
        </div>
        <div v-if="latest?.notes?.length" class="notes-list">
          <div v-for="note in latest.notes" :key="note" class="note-item">
            <ArtSvgIcon icon="ri:checkbox-circle-line" />
            <span>{{ note }}</span>
          </div>
        </div>
        <ElEmpty v-else description="暂无更新日志" :image-size="72" />
      </div>

      <div class="update-section history-section">
        <div class="section-header">
          <div>
            <strong>历史版本</strong>
            <span class="section-description">仅展示版本记录和更新日志</span>
          </div>
          <ElButton link type="primary" :loading="historyLoading" @click="loadHistory(true)">
            刷新记录
          </ElButton>
        </div>
        <ElAlert
          v-if="historyError"
          :title="historyError"
          type="warning"
          show-icon
          :closable="false"
          class="history-alert"
        />
        <ElSkeleton v-if="historyLoading && !historyReleases.length" :rows="4" animated />
        <ElTimeline v-else-if="historyReleases.length" class="release-timeline">
          <ElTimelineItem
            v-for="release in historyReleases"
            :key="release.version"
            :timestamp="formatDate(release.releasedAt)"
            placement="top"
          >
            <div class="release-card">
              <div class="release-header">
                <div class="release-version">
                  <strong>v{{ release.version }}</strong>
                  <ElTag size="small" effect="plain">{{ release.channel || 'stable' }}</ElTag>
                  <ElTag
                    v-if="isCurrentRelease(release.version)"
                    size="small"
                    type="info"
                    effect="plain"
                  >
                    当前版本
                  </ElTag>
                  <ElTag
                    v-if="isLatestRelease(release.version)"
                    size="small"
                    type="success"
                    effect="plain"
                  >
                    最新版本
                  </ElTag>
                </div>
              </div>
              <div v-if="release.notes.length" class="release-notes">
                <div
                  v-for="(note, noteIndex) in release.notes"
                  :key="`${release.version}-${noteIndex}`"
                  class="release-note"
                >
                  <span class="release-note-dot" />
                  <span>{{ note }}</span>
                </div>
              </div>
              <ElText v-else type="info" size="small">该版本未记录更新日志</ElText>
            </div>
          </ElTimelineItem>
        </ElTimeline>
        <ElEmpty v-else description="暂无历史版本记录" :image-size="72" />
      </div>

      <div class="update-section package-section">
        <div class="section-header">
          <strong>更新包</strong>
          <ElTag :type="packageValid ? 'success' : 'warning'" effect="plain">
            {{ packageValid ? '已就绪' : '待完善' }}
          </ElTag>
        </div>
        <div class="package-grid">
          <div class="package-item">
            <span>文件</span>
            <strong>{{ latest?.package.fileName || '未配置' }}</strong>
          </div>
          <div class="package-item">
            <span>大小</span>
            <strong>{{ formatBytes(latest?.package.size) }}</strong>
          </div>
        </div>
      </div>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { Download, Refresh, Search } from '@element-plus/icons-vue'
  import {
    fetchOnlineUpdateApply,
    fetchOnlineUpdateCheck,
    fetchOnlineUpdateHistory,
    fetchOnlineUpdateJob,
    fetchOnlineUpdateStatus,
    OnlineUpdateCheckResult,
    OnlineUpdateHistory,
    OnlineUpdateJob,
    OnlineUpdateStatus
  } from '@/api/update'

  defineOptions({ name: 'OnlineUpdate' })

  const loading = ref(false)
  const historyLoading = ref(false)
  const checking = ref(false)
  const applying = ref(false)
  const status = ref<OnlineUpdateStatus | null>(null)
  const history = ref<OnlineUpdateHistory | null>(null)
  const historyError = ref('')
  const checkResult = ref<OnlineUpdateCheckResult | null>(null)
  const job = ref<OnlineUpdateJob | null>(null)
  const jobSectionRef = ref<HTMLElement | null>(null)
  let jobTimer: ReturnType<typeof setInterval> | undefined
  let reloadScheduled = false

  const latest = computed(() => checkResult.value?.latest || status.value?.latest || null)
  const currentVersion = computed(
    () => checkResult.value?.currentVersion || status.value?.currentVersion || '-'
  )
  const latestVersion = computed(() => (latest.value?.version ? `v${latest.value.version}` : '-'))
  const historyReleases = computed(() => history.value?.releases || [])
  const updateAvailable = computed(() => checkResult.value?.updateAvailable === true)
  const packageError = computed(() => checkResult.value?.packageError || '')
  const versionError = computed(() => checkResult.value?.versionError || '')
  const packageValid = computed(() => checkResult.value?.packageValid === true)
  const isJobActive = computed(() =>
    job.value ? ['running', 'restarting'].includes(job.value.status) : false
  )
  const canApply = computed(
    () => checkResult.value?.canApply === true && !applying.value && !isJobActive.value
  )
  const jobProgress = computed(() => {
    if (!job.value) return 0
    if (job.value.status === 'success') return 100
    const progress = Number(job.value.progress)
    const normalizedProgress = Number.isFinite(progress) ? Math.min(100, Math.max(0, progress)) : 0
    if (job.value.status === 'restarting') return Math.max(95, normalizedProgress)
    return normalizedProgress
  })
  const jobProgressStatus = computed(() => {
    if (job.value?.status === 'success') return 'success' as const
    if (job.value?.status === 'failed') return 'exception' as const
    return undefined
  })
  const isInsecureUpdateUrl = computed(() => {
    const url = checkResult.value?.updateUrl || status.value?.updateUrl || ''
    return url.startsWith('http://')
  })

  const loadStatus = async () => {
    loading.value = true
    try {
      status.value = await fetchOnlineUpdateStatus()
      job.value = status.value.runningJob || job.value
      if (job.value && ['running', 'restarting'].includes(job.value.status)) {
        startJobPolling(job.value.id)
      }
    } catch {
      ElMessage.error('更新状态加载失败')
    } finally {
      loading.value = false
    }
  }

  const loadHistory = async (refresh = false) => {
    historyLoading.value = true
    historyError.value = ''
    try {
      history.value = await fetchOnlineUpdateHistory(refresh)
    } catch (error: any) {
      historyError.value = error?.message || '历史版本加载失败'
    } finally {
      historyLoading.value = false
    }
  }

  const loadPage = async (refreshHistory = false) => {
    await Promise.all([loadStatus(), loadHistory(refreshHistory)])
  }

  const handleCheck = async () => {
    checking.value = true
    try {
      checkResult.value = await fetchOnlineUpdateCheck()
      void loadHistory(true)
      if (checkResult.value.updateAvailable) {
        ElMessage.success(`发现新版本 v${checkResult.value.latest.version}`)
      } else {
        ElMessage.success('当前已经是最新版本')
      }
    } catch (error: any) {
      ElMessage.error(error?.message || '检查更新失败')
    } finally {
      checking.value = false
    }
  }

  const handleApply = async () => {
    if (!latest.value) return
    try {
      await ElMessageBox.confirm(
        `确认更新到 v${latest.value.version}？更新过程中服务会短暂重启。`,
        '在线更新',
        {
          confirmButtonText: '开始更新',
          cancelButtonText: '取消',
          type: 'warning'
        }
      )
    } catch {
      return
    }

    applying.value = true
    try {
      job.value = await fetchOnlineUpdateApply()
      await nextTick()
      jobSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
      ElMessage.success('更新任务已启动')
      startJobPolling(job.value.id)
    } catch (error: any) {
      ElMessage.error(error?.message || '更新启动失败')
    } finally {
      applying.value = false
    }
  }

  const startJobPolling = (id: string) => {
    stopJobPolling()
    jobTimer = setInterval(async () => {
      try {
        const nextJob = await fetchOnlineUpdateJob(id)
        job.value = nextJob
        if (nextJob.status === 'success') {
          stopJobPolling()
          ElMessage.success('更新完成，正在加载新版本')
          if (!reloadScheduled) {
            reloadScheduled = true
            window.setTimeout(() => window.location.reload(), 1000)
          }
          return
        }
        if (nextJob.status === 'failed') {
          stopJobPolling()
          ElMessage.error(nextJob.error || nextJob.message || '更新失败，已尝试回滚')
        }
      } catch {
        if (job.value?.status === 'restarting') return
        stopJobPolling()
      }
    }, 2000)
  }

  const stopJobPolling = () => {
    if (jobTimer) clearInterval(jobTimer)
    jobTimer = undefined
  }

  const jobStatusText = (value: OnlineUpdateJob['status']) => {
    const labels: Record<OnlineUpdateJob['status'], string> = {
      running: '执行中',
      restarting: '重启中',
      success: '已完成',
      failed: '失败'
    }
    return labels[value] || value
  }

  const jobStatusTag = (value: OnlineUpdateJob['status']) => {
    const types: Record<OnlineUpdateJob['status'], 'primary' | 'success' | 'danger' | 'warning'> = {
      running: 'primary',
      restarting: 'warning',
      success: 'success',
      failed: 'danger'
    }
    return types[value] || 'primary'
  }

  const normalizedVersion = (value?: string) => (value || '').trim().replace(/^v/i, '')

  const isCurrentRelease = (version: string) =>
    normalizedVersion(version) === normalizedVersion(currentVersion.value)

  const isLatestRelease = (version: string) => {
    const knownLatestVersion = latest.value?.version || historyReleases.value[0]?.version
    return normalizedVersion(version) === normalizedVersion(knownLatestVersion)
  }

  const formatBytes = (value?: number) => {
    if (!value || value <= 0) return '未配置'
    if (value < 1024) return `${value} B`
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
    return `${(value / 1024 / 1024).toFixed(2)} MB`
  }

  const formatDate = (value?: string) => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return date.toLocaleString()
  }

  onMounted(() => {
    void loadPage()
  })

  onBeforeUnmount(stopJobPolling)
</script>

<style lang="scss" scoped>
  .online-update {
    .update-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 18px;

      .update-title {
        margin: 0;
        font-size: 20px;
        color: var(--art-gray-900);
      }

      .update-subtitle {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--art-gray-600);
      }

      .update-actions {
        display: flex;
        flex-shrink: 0;
        gap: 10px;
        align-items: center;
      }
    }

    .update-alert {
      margin-bottom: 12px;
    }

    .version-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 14px;
    }

    .version-item {
      min-width: 0;
      padding: 18px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 12px;

      &.latest.available {
        background: rgba(var(--art-primary-rgb), 0.07);
        border-color: rgba(var(--art-primary-rgb), 0.18);
      }

      .version-label {
        display: block;
        font-size: 13px;
        color: var(--art-gray-500);
      }

      .version-value {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 10px;
        margin-top: 10px;

        strong {
          overflow: hidden;
          font-size: 24px;
          color: var(--art-gray-900);
          text-overflow: ellipsis;
          white-space: nowrap;
        }

        &.plain {
          justify-content: flex-start;
        }
      }

      .version-meta {
        display: block;
        margin-top: 10px;
        overflow: hidden;
        font-size: 12px;
        color: var(--art-gray-500);
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .update-section {
      margin-top: 22px;
      padding-top: 20px;
      border-top: 1px solid var(--art-border-color);

      .section-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 14px;

        strong {
          font-size: 15px;
          color: var(--art-gray-900);
        }

        .section-description {
          margin-left: 10px;
          font-size: 12px;
          color: var(--art-gray-500);
        }
      }
    }

    .notes-list {
      display: grid;
      gap: 10px;

      .note-item {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        font-size: 13px;
        line-height: 1.6;
        color: var(--art-gray-700);

        .art-svg-icon {
          flex-shrink: 0;
          margin-top: 3px;
          color: var(--el-color-success);
        }
      }
    }

    .history-alert {
      margin-bottom: 16px;
    }

    .release-timeline {
      padding-left: 4px;
    }

    .release-card {
      padding: 16px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 10px;
    }

    .release-version {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;

      strong {
        margin-right: 2px;
        font-size: 16px;
        color: var(--art-gray-900);
      }
    }

    .release-notes {
      display: grid;
      gap: 8px;
      margin-top: 12px;
    }

    .release-note {
      display: flex;
      align-items: flex-start;
      gap: 9px;
      font-size: 13px;
      line-height: 1.6;
      color: var(--art-gray-700);
    }

    .release-note-dot {
      flex: 0 0 5px;
      width: 5px;
      height: 5px;
      margin-top: 8px;
      background: var(--el-color-primary);
      border-radius: 50%;
    }

    .package-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }

    .package-item {
      min-width: 0;
      padding: 12px;
      background: var(--art-gray-100);
      border-radius: 10px;

      &.wide {
        grid-column: span 2;
      }

      span {
        display: block;
        font-size: 12px;
        color: var(--art-gray-500);
      }

      strong {
        display: block;
        margin-top: 6px;
        overflow: hidden;
        font-size: 13px;
        color: var(--art-gray-800);
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .job-section {
      scroll-margin-top: 20px;
    }

    .job-progress {
      padding: 14px;
      margin-bottom: 14px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 10px;

      .job-progress-meta {
        display: flex;
        justify-content: space-between;
        gap: 12px;
        margin-top: 10px;
        font-size: 12px;
        color: var(--art-gray-600);

        span:last-child {
          flex-shrink: 0;
          color: var(--art-gray-500);
        }
      }
    }

    .job-logs {
      padding: 12px;
      background: #111827;
      border-radius: 10px;

      .job-log {
        font-family: 'Roboto Mono', Consolas, monospace;
        font-size: 12px;
        line-height: 1.8;
        color: #d1d5db;
        word-break: break-all;
      }
    }

    @media (max-width: 900px) {
      .version-grid,
      .package-grid {
        grid-template-columns: 1fr;
      }

      .package-item.wide {
        grid-column: span 1;
      }
    }

    @media (max-width: 768px) {
      .update-header {
        flex-direction: column;

        .update-actions {
          width: 100%;
        }
      }
    }
  }
</style>
