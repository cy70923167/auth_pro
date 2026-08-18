<template>
  <div class="payment-orders-page art-full-height">
    <ElCard class="page-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>订单列表</h2>
            <p>查看用户充值、代理充值和支付测试的全部易支付订单</p>
          </div>
          <ElButton :loading="loading" @click="loadList">刷新</ElButton>
        </div>
      </template>

      <ElForm :model="search" inline class="search-bar">
        <ElFormItem label="订单号">
          <ElInput
            v-model.trim="search.orderNo"
            clearable
            placeholder="支持模糊搜索"
            class="search-input"
            @keyup.enter="handleSearch"
            @clear="handleSearch"
          />
        </ElFormItem>
        <ElFormItem label="订单类型">
          <ElSelect v-model="search.subjectType" clearable placeholder="全部" class="search-select">
            <ElOption label="用户充值" value="user" />
            <ElOption label="代理充值" value="agent" />
            <ElOption label="支付测试" value="test" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="支付方式">
          <ElSelect v-model="search.payMethod" clearable placeholder="全部" class="search-select">
            <ElOption
              v-for="item in payMethodOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="支付状态">
          <ElSelect v-model="search.status" clearable placeholder="全部" class="search-select">
            <ElOption
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem>
          <ElButton type="primary" @click="handleSearch">查询</ElButton>
          <ElButton @click="handleReset">重置</ElButton>
        </ElFormItem>
      </ElForm>

      <ElTable v-loading="loading" :data="list" border>
        <ElTableColumn prop="orderNo" label="订单号" min-width="190" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.orderNo }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="订单类型" width="100" align="center">
          <template #default="{ row }">
            <ElTag :type="subjectTagTypes[row.subjectType] || 'info'" size="small">
              {{ subjectLabels[row.subjectType] || row.subjectType }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="下单主体" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.subjectType === 'test'">支付测试</span>
            <span v-else>{{ row.subjectName || `#${row.subjectId}` }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="金额" width="110" align="right">
          <template #default="{ row }">
            <span class="amount-text">¥{{ Number(row.amount || 0).toFixed(2) }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="支付方式" width="100" align="center">
          <template #default="{ row }">
            {{ payMethodLabel(row.payMethod) }}
          </template>
        </ElTableColumn>
        <ElTableColumn label="支付状态" width="100" align="center">
          <template #default="{ row }">
            <ElTag :type="statusTagTypes[row.status] || 'info'" size="small">
              {{ statusLabels[row.status] || row.status }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="createdAt" label="创建时间" width="170" />
        <ElTableColumn prop="paidAt" label="支付时间" width="170">
          <template #default="{ row }">{{ row.paidAt || '-' }}</template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="90" fixed="right">
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

    <ElDialog v-model="detailVisible" title="订单详情" width="560px">
      <ElDescriptions v-if="current" :column="1" border>
        <ElDescriptionsItem label="订单号">
          <span class="mono">{{ current.orderNo }}</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="订单类型">
          {{ subjectLabels[current.subjectType] || current.subjectType }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="下单主体">
          {{ current.subjectType === 'test' ? '支付测试' : current.subjectName || `#${current.subjectId}` }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="订单金额">¥{{ Number(current.amount || 0).toFixed(2) }}</ElDescriptionsItem>
        <ElDescriptionsItem label="实付金额">
          {{ current.paidAmount ? `¥${Number(current.paidAmount).toFixed(2)}` : '-' }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="支付方式">{{ payMethodLabel(current.payMethod) }}</ElDescriptionsItem>
        <ElDescriptionsItem label="支付渠道">{{ current.payChannel || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="支付状态">
          <ElTag :type="statusTagTypes[current.status] || 'info'" size="small">
            {{ statusLabels[current.status] || current.status }}
          </ElTag>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="网关交易号">{{ current.gatewayTradeNo || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="备注">{{ current.remark || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="创建时间">{{ current.createdAt || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="支付时间">{{ current.paidAt || '-' }}</ElDescriptionsItem>
      </ElDescriptions>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import {
    fetchPaymentOrderList,
    type PaymentOrderItem,
    type PaymentOrderSearchParams
  } from '@/api/system-manage'

  defineOptions({ name: 'PaymentOrders' })

  const loading = ref(false)
  const list = ref<PaymentOrderItem[]>([])
  const total = ref(0)
  const detailVisible = ref(false)
  const current = ref<PaymentOrderItem>()

  const search = reactive<PaymentOrderSearchParams>({
    page: 1,
    pageSize: 20,
    orderNo: '',
    subjectType: '',
    status: '',
    payMethod: ''
  })

  const payMethodOptions = [
    { label: '支付宝', value: 'alipay' },
    { label: '微信支付', value: 'wxpay' },
    { label: 'QQ 钱包', value: 'qqpay' },
    { label: '余额支付', value: 'balance' },
    { label: '配额支付', value: 'quota' }
  ]

  const statusOptions = [
    { label: '待支付', value: 'pending' },
    { label: '已支付', value: 'paid' },
    { label: '失败', value: 'failed' },
    { label: '已取消', value: 'cancelled' }
  ]

  const subjectLabels: Record<string, string> = {
    user: '用户充值',
    agent: '代理充值',
    test: '支付测试'
  }

  const subjectTagTypes: Record<string, 'primary' | 'success' | 'warning' | 'info'> = {
    user: 'primary',
    agent: 'success',
    test: 'warning'
  }

  const statusLabels: Record<string, string> = {
    pending: '待支付',
    paid: '已支付',
    failed: '失败',
    cancelled: '已取消'
  }

  const statusTagTypes: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'warning',
    paid: 'success',
    failed: 'danger',
    cancelled: 'info'
  }

  const payMethodLabel = (value: string) => {
    return payMethodOptions.find((item) => item.value === value)?.label || value || '-'
  }

  onMounted(() => {
    loadList()
  })

  const loadList = async () => {
    loading.value = true
    try {
      const data = await fetchPaymentOrderList({ ...search })
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
    search.orderNo = ''
    search.subjectType = ''
    search.status = ''
    search.payMethod = ''
    loadList()
  }

  const openDetail = (row: PaymentOrderItem) => {
    current.value = row
    detailVisible.value = true
  }
</script>

<style scoped lang="scss">
  .payment-orders-page {
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

    .search-input {
      width: 200px;
    }

    .search-select {
      width: 140px;
    }

    .mono {
      font-family: 'Roboto Mono', monospace;
    }

    .amount-text {
      font-weight: 600;
      color: var(--art-gray-900);
    }

    .pagination-wrap {
      display: flex;
      justify-content: flex-end;
      margin-top: 18px;
    }
  }
</style>
