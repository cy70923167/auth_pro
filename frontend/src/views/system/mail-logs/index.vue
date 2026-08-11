<template>
  <div class="mail-logs-page art-full-height">
    <ElCard class="page-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>邮件日志</h2>
            <p>查看测试发送、购买成功、到期提醒和后台开通邮件的发送结果</p>
          </div>
          <ElButton :loading="loading" @click="loadList">刷新</ElButton>
        </div>
      </template>

      <ElForm :model="search" inline class="search-bar">
        <ElFormItem label="事件类型">
          <ElSelect v-model="search.eventType" clearable placeholder="全部" class="search-select">
            <ElOption label="测试发送" value="test" />
            <ElOption label="购买成功" value="purchase_success" />
            <ElOption label="到期提醒" value="expire_reminder" />
            <ElOption label="后台开通" value="license_opened" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="状态">
          <ElSelect v-model="search.status" clearable placeholder="全部" class="search-select">
            <ElOption label="待发送" value="pending" />
            <ElOption label="已发送" value="sent" />
            <ElOption label="失败" value="failed" />
            <ElOption label="跳过" value="skipped" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="关键词">
          <ElInput v-model.trim="search.keyword" clearable placeholder="收件邮箱 / 标题" />
        </ElFormItem>
        <ElFormItem>
          <ElButton type="primary" @click="handleSearch">查询</ElButton>
          <ElButton @click="handleReset">重置</ElButton>
        </ElFormItem>
      </ElForm>

      <ElTable v-loading="loading" :data="list" border>
        <ElTableColumn prop="id" label="ID" width="80" />
        <ElTableColumn label="事件类型" width="130">
          <template #default="{ row }">
            {{ eventTypeLabels[row.eventType] || row.eventType }}
          </template>
        </ElTableColumn>
        <ElTableColumn label="状态" width="100">
          <template #default="{ row }">
            <ElTag :type="statusTagTypes[row.status] || 'info'">
              {{ statusLabels[row.status] || row.status }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="recipient" label="收件邮箱" min-width="180" show-overflow-tooltip />
        <ElTableColumn prop="subject" label="标题" min-width="220" show-overflow-tooltip />
        <ElTableColumn prop="targetType" label="主体" width="90" />
        <ElTableColumn prop="licenseId" label="授权ID" width="100" />
        <ElTableColumn prop="remindDays" label="提醒天数" width="100" />
        <ElTableColumn prop="createdAt" label="创建时间" width="170" />
        <ElTableColumn prop="sentAt" label="发送时间" width="170" />
        <ElTableColumn label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <ElButton link type="primary" @click="openDetail(row)">详情</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="pagination-wrap">
        <ElPagination
          v-model:current-page="search.page"
          v-model:page-size="search.pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList"
          @current-change="loadList"
        />
      </div>
    </ElCard>

    <ElDialog v-model="detailVisible" title="邮件详情" width="760px">
      <ElDescriptions v-if="current" :column="1" border>
        <ElDescriptionsItem label="事件类型">{{ eventTypeLabels[current.eventType] || current.eventType }}</ElDescriptionsItem>
        <ElDescriptionsItem label="状态">{{ statusLabels[current.status] || current.status }}</ElDescriptionsItem>
        <ElDescriptionsItem label="收件邮箱">{{ current.recipient || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="标题">{{ current.subject || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="错误信息">{{ current.error || '-' }}</ElDescriptionsItem>
      </ElDescriptions>
      <div v-if="current" class="content-preview">
        <div class="preview-title">邮件内容</div>
        <ElTabs model-value="source">
          <ElTabPane label="源码" name="source">
            <pre>{{ current.content }}</pre>
          </ElTabPane>
          <ElTabPane label="HTML预览" name="preview">
            <div class="html-preview" v-html="current.content"></div>
          </ElTabPane>
        </ElTabs>
      </div>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { fetchMailLogDetail, fetchMailLogList, MailLogItem, MailLogSearchParams } from '@/api/system-manage'

  defineOptions({ name: 'MailLogs' })

  const loading = ref(false)
  const list = ref<MailLogItem[]>([])
  const total = ref(0)
  const detailVisible = ref(false)
  const current = ref<MailLogItem>()

  const search = reactive<MailLogSearchParams>({
    page: 1,
    pageSize: 10,
    eventType: '',
    status: '',
    keyword: ''
  })

  const eventTypeLabels: Record<string, string> = {
    test: '测试发送',
    purchase_success: '购买成功',
    expire_reminder: '到期提醒',
    license_opened: '后台开通'
  }

  const statusLabels: Record<string, string> = {
    pending: '待发送',
    sent: '已发送',
    failed: '失败',
    skipped: '跳过'
  }

  const statusTagTypes: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'warning',
    sent: 'success',
    failed: 'danger',
    skipped: 'info'
  }

  onMounted(() => {
    loadList()
  })

  const loadList = async () => {
    loading.value = true
    try {
      const data = await fetchMailLogList({ ...search })
      list.value = data.list || []
      total.value = data.total || 0
    } finally {
      loading.value = false
    }
  }

  const handleSearch = () => {
    search.page = 1
    loadList()
  }

  const handleReset = () => {
    search.page = 1
    search.eventType = ''
    search.status = ''
    search.keyword = ''
    loadList()
  }

  const openDetail = async (row: MailLogItem) => {
    current.value = await fetchMailLogDetail(row.id)
    detailVisible.value = true
  }
</script>

<style scoped lang="scss">
  .mail-logs-page {
    .page-card {
      min-height: 100%;
      border: 0;
    }

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;

      h2 {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
      }

      p {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .search-bar {
      margin-bottom: 16px;
    }

    .search-select {
      width: 150px;
    }

    .pagination-wrap {
      display: flex;
      justify-content: flex-end;
      margin-top: 18px;
    }

    .content-preview {
      margin-top: 18px;

      .preview-title {
        margin-bottom: 10px;
        font-weight: 600;
      }

      pre {
        max-height: 360px;
        padding: 14px;
        overflow: auto;
        white-space: pre-wrap;
        border-radius: 10px;
        background: var(--art-bg-color);
      }

      .html-preview {
        max-height: 420px;
        padding: 16px;
        overflow: auto;
        border: 1px solid var(--art-border-color);
        border-radius: 10px;
        background: #fff;
      }
    }
  }
</style>