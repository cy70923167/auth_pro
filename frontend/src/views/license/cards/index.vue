<template>
  <div class="license-cards">
    <ElCard shadow="hover">
      <template #header>
        <div class="header-row">
          <div>
            <div class="title">卡密管理</div>
            <div class="subtitle">按应用、套餐和授权类型生成一次性兑换卡</div>
          </div>
          <ElButton type="primary" @click="openCreateDialog">生成卡密</ElButton>
        </div>
      </template>

      <div class="filters">
        <ElInput v-model="query.keyword" placeholder="批次号/应用/套餐" clearable style="width: 240px" @keyup.enter="fetchBatches" />
        <ElSelect v-model="query.appId" placeholder="全部应用" clearable style="width: 180px" @change="fetchBatches">
          <ElOption v-for="app in appOptions" :key="app.id" :label="app.name" :value="app.id" />
        </ElSelect>
        <ElSelect v-model="query.status" placeholder="全部状态" clearable style="width: 130px" @change="fetchBatches">
          <ElOption label="启用" value="active" />
          <ElOption label="已暂停" value="disabled" />
        </ElSelect>
        <ElSpace :size="10">
          <ElButton type="primary" @click="fetchBatches">查询</ElButton>
          <ElButton @click="resetQuery">重置</ElButton>
        </ElSpace>
      </div>

      <ElTable v-loading="loading" :data="batches" stripe>
        <ElTableColumn prop="batchNo" label="批次号" min-width="190" />
        <ElTableColumn prop="appName" label="应用" min-width="130" />
        <ElTableColumn prop="planName" label="套餐" min-width="130">
          <template #default="{ row }">
            <div>{{ row.planName }}</div>
            <span class="muted">{{ row.durationDays ? `${row.durationDays} 天` : '永久' }} / ¥{{ Number(row.price).toFixed(2) }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="typeLabel" label="授权类型" width="100">
          <template #default="{ row }"><ElTag size="small">{{ row.typeLabel }}</ElTag></template>
        </ElTableColumn>
        <ElTableColumn label="库存" min-width="210">
          <template #default="{ row }">
            <div class="stock-row">
              <span>未兑换 <b>{{ row.unusedCount }}</b></span>
              <span>已兑换 <b>{{ row.redeemedCount }}</b></span>
              <span>已禁用 <b>{{ row.disabledCount }}</b></span>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="status" label="批次状态" width="100">
          <template #default="{ row }">
            <ElTag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '启用' : '已暂停' }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="createdAt" label="生成时间" width="160" />
        <ElTableColumn label="操作" width="245" fixed="right">
          <template #default="{ row }">
            <ElSpace :size="8">
              <ElButton plain type="primary" size="small" @click="openCards(row)">明细</ElButton>
              <ElDropdown trigger="click" @command="(status: string) => exportCards(row, status)">
                <ElButton plain type="primary" size="small">
                  导出<ElIcon class="el-icon--right"><ArrowDown /></ElIcon>
                </ElButton>
                <template #dropdown>
                  <ElDropdownMenu>
                    <ElDropdownItem command="unused">仅未兑换</ElDropdownItem>
                    <ElDropdownItem command="all">全部卡密</ElDropdownItem>
                  </ElDropdownMenu>
                </template>
              </ElDropdown>
              <ElButton plain type="danger" size="small" @click="deleteBatch(row)">
                删除
              </ElButton>
            </ElSpace>
          </template>
        </ElTableColumn>
      </ElTable>

      <div class="pagination">
        <ElPagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @current-change="fetchBatches" @size-change="fetchBatches" />
      </div>
    </ElCard>

    <ElDialog v-model="createDialog.visible" title="生成卡密" width="560px" destroy-on-close>
      <ElAlert title="卡密永久有效且只能兑换一次。套餐时长从兑换成功时开始计算。" type="info" show-icon :closable="false" class="mb-16" />
      <ElForm ref="createFormRef" :model="createForm" :rules="createRules" label-width="90px">
        <ElFormItem label="应用" prop="appId">
          <ElSelect v-model="createForm.appId" placeholder="请选择应用" style="width: 100%" @change="handleCreateAppChange">
            <ElOption v-for="app in enabledApps" :key="app.id" :label="app.name" :value="app.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="套餐" prop="planId">
          <ElSelect v-model="createForm.planId" placeholder="请选择套餐" style="width: 100%" :disabled="!createForm.appId">
            <ElOption v-for="plan in availablePlans" :key="plan.id" :label="`${plan.name}（${plan.durationText} / ¥${Number(plan.price).toFixed(2)}）`" :value="plan.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="授权类型" prop="type">
          <ElSelect v-model="createForm.type" placeholder="请选择授权类型" style="width: 100%" :disabled="!createForm.appId">
            <ElOption v-for="type in availableTypes" :key="type" :label="typeMeta[type]" :value="type" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="生成数量" prop="quantity">
          <ElInputNumber v-model="createForm.quantity" :min="1" :max="5000" :step="10" controls-position="right" />
          <span class="form-tip">单批最多 5000 张</span>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="createForm.remark" type="textarea" :rows="2" maxlength="255" show-word-limit />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElSpace :size="10">
          <ElButton @click="createDialog.visible = false">取消</ElButton>
          <ElButton type="primary" :loading="createDialog.submitting" @click="submitCreate">生成</ElButton>
        </ElSpace>
      </template>
    </ElDialog>

    <ElDialog v-model="resultDialog.visible" title="卡密生成成功" width="720px" destroy-on-close>
      <ElAlert title="完整卡密已保存，可在批次中重复导出。请妥善保管，避免泄露。" type="success" show-icon :closable="false" class="mb-16" />
      <ElInput v-model="resultDialog.text" type="textarea" :rows="12" readonly />
      <template #footer>
        <ElSpace :size="10">
          <ElButton @click="copyGeneratedCards">复制全部</ElButton>
          <ElButton type="primary" @click="resultDialog.visible = false">完成</ElButton>
        </ElSpace>
      </template>
    </ElDialog>

    <ElDrawer v-model="cardsDrawer.visible" :title="`批次明细 · ${cardsDrawer.batchNo}`" size="min(960px, 92vw)" destroy-on-close>
      <div class="drawer-toolbar">
        <ElSelect v-model="cardsDrawer.status" placeholder="全部状态" clearable style="width: 140px" @change="fetchCards">
          <ElOption label="未兑换" value="unused" />
          <ElOption label="已兑换" value="redeemed" />
          <ElOption label="已禁用" value="disabled" />
        </ElSelect>
        <span class="muted">明细展示完整卡密，可直接复制</span>
      </div>
      <ElTable v-loading="cardsDrawer.loading" :data="cardsDrawer.list" stripe>
        <ElTableColumn label="卡密" min-width="270">
          <template #default="{ row }">
            <ElSpace :size="6">
              <span class="card-code">{{ row.cardCode }}</span>
              <ElButton plain type="primary" size="small" @click="copyCardCode(row.cardCode)">复制</ElButton>
            </ElSpace>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="status" label="状态" width="100">
          <template #default="{ row }">
            <ElTag :type="cardStatusType(row.status)" size="small">{{ cardStatusLabel(row.status) }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="兑换主体" min-width="190">
          <template #default="{ row }">
            {{ row.redeemedByAccount ? `${row.redeemedByType === 'user' ? '用户' : '代理'} · ${row.redeemedByAccount}` : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn prop="licenseId" label="授权ID" width="90" />
        <ElTableColumn prop="redeemedAt" label="兑换时间" width="165" />
        <ElTableColumn label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <ElButton v-if="row.status !== 'redeemed'" plain :type="row.status === 'unused' ? 'danger' : 'success'" size="small" @click="toggleCard(row)">
              {{ row.status === 'unused' ? '禁用' : '恢复' }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
      <div class="pagination">
        <ElPagination v-model:current-page="cardsDrawer.page" v-model:page-size="cardsDrawer.pageSize" :total="cardsDrawer.total" layout="total, prev, pager, next" @current-change="fetchCards" />
      </div>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import axios from 'axios'
import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/http'
import { useUserStore } from '@/store/modules/user'

const typeMeta: Record<string, string> = { domain: '单域名', wildcard: '泛域名', ip: 'IP', key: '密钥' }
const loading = ref(false)
const batches = ref<any[]>([])
const total = ref(0)
const appOptions = ref<any[]>([])
const planOptions = ref<any[]>([])
const createFormRef = ref()
const query = reactive({ keyword: '', appId: undefined as number | undefined, status: '', page: 1, pageSize: 20 })
const createDialog = reactive({ visible: false, submitting: false })
const createForm = reactive({ appId: undefined as number | undefined, planId: undefined as number | undefined, type: '', quantity: 100, remark: '' })
const resultDialog = reactive({ visible: false, text: '' })
const cardsDrawer = reactive({ visible: false, loading: false, batchId: 0, batchNo: '', status: '', list: [] as any[], page: 1, pageSize: 20, total: 0 })
const createRules = {
  appId: [{ required: true, message: '请选择应用', trigger: 'change' }],
  planId: [{ required: true, message: '请选择套餐', trigger: 'change' }],
  type: [{ required: true, message: '请选择授权类型', trigger: 'change' }],
  quantity: [{ required: true, message: '请输入生成数量', trigger: 'blur' }]
}
const enabledApps = computed(() => appOptions.value.filter((app) => app.enabled !== false))
const selectedApp = computed(() => appOptions.value.find((app) => app.id === createForm.appId))
const availablePlans = computed(() => planOptions.value.filter((plan) => plan.appId === createForm.appId && plan.enabled !== false))
const availableTypes = computed(() => Array.isArray(selectedApp.value?.purchaseLicenseTypes) ? selectedApp.value.purchaseLicenseTypes : [])

async function loadOptions() {
  const [apps, plans] = await Promise.all([
    request.get<any[]>({ url: '/api/app/list' }),
    request.get<any[]>({ url: '/api/plan/list' })
  ])
  appOptions.value = apps || []
  planOptions.value = plans || []
}
async function fetchBatches() {
  loading.value = true
  try {
    const data = await request.get<any>({ url: '/api/license/cards/batches', params: query })
    batches.value = data?.list || []
    total.value = data?.total || 0
  } finally { loading.value = false }
}
function resetQuery() { Object.assign(query, { keyword: '', appId: undefined, status: '', page: 1 }); fetchBatches() }
function openCreateDialog() {
  Object.assign(createForm, { appId: undefined, planId: undefined, type: '', quantity: 100, remark: '' })
  createDialog.visible = true
}
function handleCreateAppChange() { createForm.planId = undefined; createForm.type = '' }
async function submitCreate() {
  const valid = await createFormRef.value?.validate().catch(() => false)
  if (!valid) return
  await ElMessageBox.confirm(`确认生成 ${createForm.quantity} 张卡密？`, '生成卡密', { type: 'warning' })
  createDialog.submitting = true
  try {
    const data = await request.post<any>({ url: '/api/license/cards/batches', data: createForm })
    createDialog.visible = false
    resultDialog.text = (data?.cards || []).join('\n')
    resultDialog.visible = true
    query.page = 1
    await fetchBatches()
  } finally { createDialog.submitting = false }
}
async function deleteBatch(row: any) {
  await ElMessageBox.confirm(
    `确认删除批次「${row.batchNo}」？这将删除该批次及其所有卡密，此操作不可恢复！`,
    '确认删除',
    {
      type: 'error',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    }
  )
  await request.del({ url: `/api/license/cards/batches/${row.id}` })
  ElMessage.success('批次已删除')
  fetchBatches()
}
function openCards(row: any) {
  Object.assign(cardsDrawer, { visible: true, batchId: row.id, batchNo: row.batchNo, status: '', page: 1 })
  fetchCards()
}
async function fetchCards() {
  cardsDrawer.loading = true
  try {
    const data = await request.get<any>({ url: `/api/license/cards/batches/${cardsDrawer.batchId}/cards`, params: { status: cardsDrawer.status, page: cardsDrawer.page, pageSize: cardsDrawer.pageSize } })
    cardsDrawer.list = data?.list || []
    cardsDrawer.total = data?.total || 0
  } finally { cardsDrawer.loading = false }
}
async function toggleCard(row: any) {
  const status = row.status === 'unused' ? 'disabled' : 'unused'
  await request.put({ url: `/api/license/cards/${row.id}/status`, data: { status } })
  ElMessage.success('卡密状态已更新')
  fetchCards(); fetchBatches()
}
async function exportCards(row: any, status: string) {
  const token = useUserStore().accessToken
  const response = await axios.get(`/api/license/cards/batches/${row.id}/export`, {
    params: { status }, responseType: 'blob', headers: { Authorization: `Bearer ${token}` }
  })
  const url = URL.createObjectURL(response.data)
  const link = document.createElement('a')
  link.href = url
  link.download = `${row.batchNo}.csv`
  link.click()
  URL.revokeObjectURL(url)
}
async function copyGeneratedCards() {
  await navigator.clipboard.writeText(resultDialog.text)
  ElMessage.success('已复制全部卡密')
}
async function copyCardCode(cardCode: string) {
  await navigator.clipboard.writeText(cardCode)
  ElMessage.success('卡密已复制')
}
function cardStatusLabel(status: string) { return ({ unused: '未兑换', redeemed: '已兑换', disabled: '已禁用' } as Record<string, string>)[status] || status }
function cardStatusType(status: string): 'success' | 'info' | 'danger' { return status === 'unused' ? 'success' : status === 'redeemed' ? 'info' : 'danger' }

onMounted(async () => { await loadOptions(); await fetchBatches() })
</script>

<style scoped lang="scss">
.header-row { display: flex; align-items: center; justify-content: space-between; }
.title { font-size: 16px; font-weight: 600; }
.subtitle, .muted { margin-top: 4px; color: var(--el-text-color-secondary); font-size: 12px; }
.filters, .drawer-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.stock-row { display: flex; gap: 12px; font-size: 12px; }
.stock-row b { color: var(--el-text-color-primary); }
.pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
.form-tip { margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px; }
.card-code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; white-space: nowrap; }
.mb-16 { margin-bottom: 16px; }
</style>