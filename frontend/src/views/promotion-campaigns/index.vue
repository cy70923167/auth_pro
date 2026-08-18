<template>
  <div class="promotion-page">
    <div class="page-heading">
      <div>
        <div class="eyebrow">PROMOTION CENTER</div>
        <h1>活动管理</h1>
        <p>统一配置用户与代理商活动价，同一应用同一时间仅允许一个启用活动。</p>
      </div>
      <el-button type="primary" size="large" @click="handleAdd">新建活动</el-button>
    </div>

    <div class="summary-grid">
      <div v-for="item in summaryItems" :key="item.label" class="summary-card">
        <div class="summary-icon" :class="item.tone">
          <ArtSvgIcon :icon="item.icon" />
        </div>
        <div>
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
        </div>
      </div>
    </div>

    <el-card shadow="never" class="campaign-card">
      <div class="toolbar">
        <div class="filters">
          <el-select
            v-model="query.appId"
            placeholder="全部应用"
            clearable
            filterable
            class="filter-item"
            @change="fetchList"
          >
            <el-option v-for="app in appOptions" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
          <el-input
            v-model="query.keyword"
            placeholder="活动名称或应用"
            clearable
            class="filter-item keyword-input"
            @keyup.enter="fetchList"
          />
          <el-select
            v-model="query.audience"
            placeholder="适用对象"
            clearable
            class="filter-item"
            @change="fetchList"
          >
            <el-option label="用户" value="user" />
            <el-option label="代理商" value="agent" />
            <el-option label="全部" value="all" />
          </el-select>
          <el-select
            v-model="query.status"
            placeholder="活动状态"
            clearable
            class="filter-item"
            @change="fetchList"
          >
            <el-option label="进行中" value="active" />
            <el-option label="未开始" value="upcoming" />
            <el-option label="已结束" value="ended" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
        </div>
        <div class="toolbar-actions">
          <el-button @click="handleReset">重置</el-button>
          <el-button type="primary" :loading="loading" @click="fetchList">查询</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="tableData" row-key="id" class="campaign-table">
        <el-table-column label="活动" min-width="190">
          <template #default="{ row }">
            <div class="campaign-name">
              <strong>{{ row.name }}</strong>
              <span>{{ row.appName }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="适用对象" width="110">
          <template #default="{ row }">
            <el-tag effect="plain" :type="audienceTagType(row.audience)">
              {{ audienceLabel(row.audience) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="活动套餐" min-width="220">
          <template #default="{ row }">
            <div class="plan-tags">
              <el-tag v-for="plan in row.plans" :key="plan.planId" size="small" type="info">
                {{ plan.planName }} · {{ ruleLabel(plan) }}
              </el-tag>
              <span v-if="!row.plans.length" class="muted">未选择套餐</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="活动时间" min-width="225">
          <template #default="{ row }">
            <div class="time-range">
              <span>{{ formatDateTime(row.startsAt) }}</span>
              <span class="time-divider">至 {{ formatDateTime(row.endsAt) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }: { row: PromotionCampaignItem }">
            <el-tag :type="statusMeta[row.status].type" effect="light">
              {{ statusMeta[row.status].label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="handleToggle(row)"
            >
              {{ row.enabled ? '禁用' : '启用' }}
            </el-button>
            <el-button
              link
              type="danger"
              :loading="deletingId === row.id"
              @click="handleDelete(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无活动，创建一个活动开始配置优惠" />
        </template>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑活动' : '新建活动'"
      width="920px"
      destroy-on-close
      class="campaign-dialog"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top">
        <div class="form-grid">
          <el-form-item label="活动名称" prop="name">
            <el-input v-model="form.name" maxlength="100" show-word-limit placeholder="例如：春季限时活动" />
          </el-form-item>
          <el-form-item label="适用对象" prop="audience">
            <el-segmented v-model="form.audience" :options="audienceOptions" block class="audience-field" />
          </el-form-item>
        </div>

        <div class="form-grid">
          <el-form-item label="所属应用" prop="appId">
            <el-select
              v-model="form.appId"
              placeholder="选择应用"
              filterable
              style="width: 100%"
              :disabled="isEdit"
              @change="handleAppChange"
            >
              <el-option v-for="app in appOptions" :key="app.id" :label="app.name" :value="app.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="活动状态">
            <div class="switch-field">
              <el-switch v-model="form.enabled" />
              <span>{{ form.enabled ? '保存后立即按活动时间生效' : '保存为禁用活动' }}</span>
            </div>
          </el-form-item>
        </div>

        <el-form-item label="活动时间" prop="dateRange">
          <el-date-picker
            v-model="dateRangeModel"
            type="datetimerange"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            range-separator="至"
            format="YYYY-MM-DD HH:mm"
            :default-time="defaultTimes"
            class="date-range-field"
          />
          <div class="field-tip">结束时间不包含在活动区间内，首尾相接的活动可以连续配置。</div>
        </el-form-item>

        <el-form-item label="选择活动套餐" prop="plans">
          <div v-if="!form.appId" class="plan-empty">请先选择所属应用</div>
          <div v-else-if="plansLoading" class="plan-empty">正在加载套餐...</div>
          <el-checkbox-group v-else v-model="selectedPlanIds" class="plan-selector">
            <el-checkbox
              v-for="plan in planOptions"
              :key="plan.id"
              :value="plan.id"
              :disabled="!plan.enabled && !selectedPlanIds.includes(plan.id)"
              class="plan-option"
              :class="{ disabled: !plan.enabled }"
            >
              <span class="plan-option-name">{{ plan.name }}</span>
              <span class="plan-option-price">¥{{ money(plan.price) }}</span>
              <span v-if="!plan.enabled" class="plan-option-state">已禁用</span>
            </el-checkbox>
          </el-checkbox-group>
          <div v-if="form.appId && !plansLoading && !planOptions.length" class="plan-empty">
            该应用暂无套餐
          </div>
        </el-form-item>

        <div v-if="form.plans.length" class="rule-panel">
          <div class="rule-panel-header">
            <div>
              <strong>套餐优惠规则</strong>
              <span>每个套餐独立配置，成交时只采用最低价格</span>
            </div>
            <el-tag type="primary" effect="plain">已选 {{ form.plans.length }} 个</el-tag>
          </div>
          <div v-for="draft in form.plans" :key="draft.planId" class="rule-row">
            <div class="rule-plan">
              <strong>{{ planById(draft.planId)?.name || `套餐 ${draft.planId}` }}</strong>
              <span>原价 ¥{{ money(planById(draft.planId)?.price || 0) }}</span>
            </div>
            <el-select v-model="draft.ruleType" class="rule-type" @change="normalizeRuleValue(draft)">
              <el-option label="折扣" value="discount" />
              <el-option label="立减" value="reduction" />
              <el-option label="固定活动价" value="fixed_price" />
            </el-select>
            <el-input-number
              v-model="draft.value"
              :min="ruleMinimum(draft.ruleType)"
              :max="ruleMaximum(draft)"
              :precision="draft.ruleType === 'discount' ? 3 : 2"
              :step="draft.ruleType === 'discount' ? 0.1 : 1"
              controls-position="right"
              class="rule-value"
            />
            <div class="rule-preview">{{ rulePreview(draft) }}</div>
          </div>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEdit ? '保存修改' : '创建活动' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, reactive, ref } from 'vue'
  import {
    ElMessage,
    ElMessageBox,
    type FormInstance,
    type FormRules,
    type TagProps
  } from 'element-plus'
  import {
    createPromotionCampaign,
    deletePromotionCampaign,
    fetchPromotionApps,
    fetchPromotionCampaigns,
    fetchPromotionPlans,
    togglePromotionCampaign,
    updatePromotionCampaign,
    type PromotionAudience,
    type PromotionCampaignItem,
    type PromotionCampaignPlan,
    type PromotionCampaignStatus,
    type PromotionPlanOption,
    type PromotionRuleType
  } from '@/api/promotion-campaign'

  defineOptions({ name: 'PromotionCampaigns' })

  interface CampaignRuleDraft {
    planId: number
    ruleType: PromotionRuleType
    value: number
  }

  interface CampaignForm {
    id: number
    appId?: number
    name: string
    audience: PromotionAudience
    dateRange: [Date | null, Date | null]
    enabled: boolean
    plans: CampaignRuleDraft[]
  }

  const loading = ref(false)
  const submitting = ref(false)
  const deletingId = ref(0)
  const plansLoading = ref(false)
  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const formRef = ref<FormInstance>()
  const tableData = ref<PromotionCampaignItem[]>([])
  const appOptions = ref<Array<{ id: number; name: string }>>([])
  const planOptions = ref<PromotionPlanOption[]>([])

  const query = reactive<{
    appId?: number
    keyword: string
    audience: PromotionAudience | ''
    status: PromotionCampaignStatus | ''
  }>({
    appId: undefined,
    keyword: '',
    audience: '',
    status: ''
  })

  const createInitialForm = (): CampaignForm => ({
    id: 0,
    appId: undefined,
    name: '',
    audience: 'all',
    dateRange: [null, null],
    enabled: true,
    plans: []
  })

  const form = reactive<CampaignForm>(createInitialForm())
  const defaultTimes: [Date, Date] = [new Date(2000, 0, 1, 0, 0, 0), new Date(2000, 0, 1, 23, 59, 59)]
  const dateRangeModel = computed<[Date, Date] | null>({
    get: () => {
      const [start, end] = form.dateRange
      return start && end ? ([start, end] as [Date, Date]) : null
    },
    set: (value: [Date, Date] | null) => {
      form.dateRange = value ? [value[0], value[1]] : [null, null]
    }
  })
  const audienceOptions = [
    { label: '用户', value: 'user' },
    { label: '代理商', value: 'agent' },
    { label: '全部', value: 'all' }
  ]

  const statusMeta: Record<PromotionCampaignStatus, { label: string; type: TagProps['type'] }> = {
    active: { label: '进行中', type: 'success' },
    upcoming: { label: '未开始', type: 'primary' },
    ended: { label: '已结束', type: 'info' },
    disabled: { label: '已禁用', type: 'warning' }
  }

  const formRules: FormRules<CampaignForm> = {
    name: [{ required: true, message: '请输入活动名称', trigger: 'blur' }],
    appId: [{ required: true, message: '请选择所属应用', trigger: 'change' }],
    audience: [{ required: true, message: '请选择适用对象', trigger: 'change' }],
    dateRange: [
      {
        validator: (_rule, value: CampaignForm['dateRange'], callback) => {
          const [start, end] = value || [null, null]
          if (!start || !end) {
            callback(new Error('请选择完整活动时间'))
            return
          }
          if (end.getTime() <= start.getTime()) {
            callback(new Error('结束时间必须晚于开始时间'))
            return
          }
          callback()
        },
        trigger: 'change'
      }
    ],
    plans: [
      {
        validator: (_rule, value: CampaignRuleDraft[], callback) => {
          if (!value?.length) {
            callback(new Error('请至少选择一个活动套餐'))
            return
          }
          const invalid = value.some((draft) => {
            if (!Number.isFinite(draft.value)) return true
            if (draft.ruleType === 'discount') return draft.value <= 0 || draft.value > 10
            const price = Number(planById(draft.planId)?.price || 0)
            return draft.value < 0 || draft.value > price || (draft.ruleType === 'reduction' && draft.value === 0)
          })
          if (invalid) {
            callback(new Error('请检查套餐优惠值，立减和固定价不能高于套餐原价'))
            return
          }
          callback()
        },
        trigger: 'change'
      }
    ]
  }

  const selectedPlanIds = computed<number[]>({
    get: () => form.plans.map((item) => item.planId),
    set: (ids) => {
      const previous = new Map(form.plans.map((item) => [item.planId, item]))
      form.plans = ids.map((planId) => {
        const existing = previous.get(planId)
        if (existing) return existing
        return {
          planId,
          ruleType: 'fixed_price',
          value: Number(planById(planId)?.price || 0)
        }
      })
      void nextTick(() => formRef.value?.validateField('plans').catch(() => undefined))
    }
  })

  const summaryItems = computed(() => {
    const counts = tableData.value.reduce(
      (result, item) => {
        result[item.status] += 1
        return result
      },
      { active: 0, upcoming: 0, ended: 0, disabled: 0 }
    )
    return [
      { label: '全部活动', value: tableData.value.length, icon: 'ri:price-tag-3-line', tone: 'blue' },
      { label: '进行中', value: counts.active, icon: 'ri:flashlight-line', tone: 'green' },
      { label: '未开始', value: counts.upcoming, icon: 'ri:time-line', tone: 'purple' },
      { label: '已禁用', value: counts.disabled, icon: 'ri:pause-circle-line', tone: 'orange' }
    ]
  })

  const money = (value: number) => Number(value || 0).toFixed(2)
  const planById = (planId: number) => planOptions.value.find((plan) => plan.id === planId)

  const audienceLabel = (audience: PromotionAudience) =>
    ({ user: '用户', agent: '代理商', all: '全部' })[audience]

  const audienceTagType = (audience: PromotionAudience): TagProps['type'] =>
    ({ user: 'primary', agent: 'warning', all: 'success' })[audience] as TagProps['type']

  const formatDateTime = (value: string) => {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }).format(date)
  }

  const ruleLabel = (plan: PromotionCampaignPlan) => {
    if (plan.ruleType === 'discount') return `${plan.value} 折`
    if (plan.ruleType === 'reduction') return `立减 ¥${money(plan.value)}`
    return `固定 ¥${money(plan.value)}`
  }

  const ruleMinimum = (ruleType: PromotionRuleType) => {
    if (ruleType === 'fixed_price') return 0
    if (ruleType === 'reduction') return 0.01
    return 0.001
  }

  const ruleMaximum = (draft: CampaignRuleDraft) => {
    if (draft.ruleType === 'discount') return 10
    return Number(planById(draft.planId)?.price || 0)
  }

  const normalizeRuleValue = (draft: CampaignRuleDraft) => {
    const price = Number(planById(draft.planId)?.price || 0)
    if (draft.ruleType === 'discount') {
      draft.value = 9
      return
    }
    draft.value = Math.min(price, draft.ruleType === 'reduction' ? Math.max(0.01, price * 0.1) : price)
  }

  const rulePreview = (draft: CampaignRuleDraft) => {
    const price = Number(planById(draft.planId)?.price || 0)
    let result = draft.value
    if (draft.ruleType === 'discount') result = (price * draft.value) / 10
    if (draft.ruleType === 'reduction') result = Math.max(0, price - draft.value)
    return `活动计算价 ¥${money(result)}`
  }

  const fetchApps = async () => {
    appOptions.value = (await fetchPromotionApps()) || []
  }

  const fetchList = async () => {
    loading.value = true
    try {
      tableData.value =
        (await fetchPromotionCampaigns({
          appId: query.appId,
          keyword: query.keyword.trim() || undefined,
          audience: query.audience || undefined,
          status: query.status || undefined
        })) || []
    } catch {
      ElMessage.error('活动列表加载失败')
    } finally {
      loading.value = false
    }
  }

  const loadPlans = async (appId: number) => {
    plansLoading.value = true
    try {
      planOptions.value = (await fetchPromotionPlans(appId)) || []
    } catch {
      planOptions.value = []
      ElMessage.error('套餐列表加载失败')
    } finally {
      plansLoading.value = false
    }
  }

  const handleReset = () => {
    Object.assign(query, { appId: undefined, keyword: '', audience: '', status: '' })
    void fetchList()
  }

  const resetForm = () => {
    Object.assign(form, createInitialForm())
    planOptions.value = []
    isEdit.value = false
    formRef.value?.clearValidate()
  }

  const handleAdd = () => {
    resetForm()
    dialogVisible.value = true
  }

  const handleAppChange = async (appId?: number) => {
    form.plans = []
    planOptions.value = []
    if (appId) await loadPlans(appId)
    formRef.value?.clearValidate('plans')
  }

  const handleEdit = async (row: PromotionCampaignItem) => {
    isEdit.value = true
    dialogVisible.value = true
    await loadPlans(row.appId)
    Object.assign(form, {
      id: row.id,
      appId: row.appId,
      name: row.name,
      audience: row.audience,
      dateRange: [new Date(row.startsAt), new Date(row.endsAt)],
      enabled: row.enabled,
      plans: row.plans.map((plan) => ({
        planId: plan.planId,
        ruleType: plan.ruleType,
        value: Number(plan.value)
      }))
    })
    await nextTick()
    formRef.value?.clearValidate()
  }

  const handleSubmit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    const [startAt, endAt] = form.dateRange
    if (!valid || !form.appId || !startAt || !endAt) return

    submitting.value = true
    try {
      const payload = {
        appId: form.appId,
        name: form.name.trim(),
        audience: form.audience,
        startsAt: startAt.toISOString(),
        endsAt: endAt.toISOString(),
        enabled: form.enabled,
        plans: form.plans.map((draft) => ({
          planId: draft.planId,
          ruleType: draft.ruleType,
          value: Number(draft.value)
        }))
      }
      if (isEdit.value) {
        await updatePromotionCampaign(form.id, payload)
        ElMessage.success('活动已更新')
      } else {
        await createPromotionCampaign(payload)
        ElMessage.success('活动已创建')
      }
      dialogVisible.value = false
      await fetchList()
    } finally {
      submitting.value = false
    }
  }

  const handleToggle = async (row: PromotionCampaignItem) => {
    const enabled = !row.enabled
    if (!enabled) {
      try {
        await ElMessageBox.confirm(`确定禁用活动「${row.name}」？`, '禁用活动', {
          type: 'warning',
          confirmButtonText: '确认禁用'
        })
      } catch {
        return
      }
    }
    await togglePromotionCampaign(row.id, enabled)
    ElMessage.success(enabled ? '活动已启用' : '活动已禁用')
    await fetchList()
  }

  const handleDelete = async (row: PromotionCampaignItem) => {
    if (deletingId.value !== 0) return
    try {
      await ElMessageBox.confirm(
        `删除后活动「${row.name}」及其套餐优惠规则将永久移除，不再参与后续定价。历史订单中的活动定价快照不会被修改。`,
        '确认删除活动',
        {
          type: 'warning',
          confirmButtonText: '确认删除',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return
    }

    deletingId.value = row.id
    try {
      await deletePromotionCampaign(row.id)
      ElMessage.success('活动已删除，历史订单不受影响')
      await fetchList()
    } finally {
      deletingId.value = 0
    }
  }

  onMounted(async () => {
    await Promise.all([fetchApps(), fetchList()])
  })
</script>

<style scoped lang="scss">
  .promotion-page {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .page-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    padding: 6px 2px;

    h1 {
      margin: 4px 0 6px;
      color: var(--art-gray-900);
      font-size: 28px;
      line-height: 1.2;
    }

    p {
      margin: 0;
      color: var(--art-gray-600);
    }
  }

  .eyebrow {
    color: var(--el-color-primary);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.16em;
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 14px;
  }

  .summary-card {
    display: flex;
    gap: 14px;
    align-items: center;
    padding: 18px;
    background: var(--art-main-bg-color);
    border: 1px solid var(--art-card-border);
    border-radius: 12px;

    > div:last-child {
      display: flex;
      flex-direction: column;
      gap: 3px;
    }

    span {
      color: var(--art-gray-600);
      font-size: 13px;
    }

    strong {
      color: var(--art-gray-900);
      font-size: 24px;
      line-height: 1;
    }
  }

  .summary-icon {
    display: grid;
    width: 42px;
    height: 42px;
    color: #fff;
    border-radius: 12px;
    place-items: center;

    &.blue { background: linear-gradient(135deg, #4776e6, #6b8df2); }
    &.green { background: linear-gradient(135deg, #18a875, #42c99a); }
    &.purple { background: linear-gradient(135deg, #7956d8, #9b7bea); }
    &.orange { background: linear-gradient(135deg, #ef8d3c, #f5ad64); }
  }

  .campaign-card {
    border-radius: 14px;
  }

  .toolbar {
    display: flex;
    gap: 12px;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 18px;
  }

  .filters,
  .toolbar-actions {
    display: flex;
    gap: 10px;
    align-items: center;
  }

  .filter-item { width: 150px; }
  .keyword-input { width: 210px; }

  .campaign-name,
  .time-range,
  .rule-plan {
    display: flex;
    flex-direction: column;
    gap: 4px;

    span {
      color: var(--art-gray-600);
      font-size: 12px;
    }
  }

  .plan-tags {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .time-divider,
  .muted {
    color: var(--art-gray-500);
    font-size: 12px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 18px;
  }

  .campaign-dialog {
    .el-dialog__header {
      padding: 22px 28px 0;
    }

    .el-dialog__title {
      font-size: 18px;
      font-weight: 600;
    }

    .el-dialog__body {
      max-height: min(68vh, 720px);
      padding: 24px 28px 12px;
      overflow-y: auto;
    }

    .el-dialog__footer {
      padding: 16px 28px 22px;
      border-top: 1px solid var(--el-border-color-lighter);
    }

    .el-form-item {
      margin-bottom: 20px;
    }

    .el-form-item__label {
      margin-bottom: 8px;
      color: var(--art-gray-800);
      font-weight: 500;
    }
  }

  .audience-field {
    width: 100%;
    min-width: 0;
    --el-segmented-item-selected-bg-color: var(--el-color-primary);
    --el-segmented-item-selected-color: #fff;

    :deep(.el-segmented__item) {
      min-width: 0;
      padding: 0 10px;
      white-space: nowrap;
    }

    :deep(.el-segmented__item-label) {
      overflow: visible;
      text-overflow: clip;
      white-space: nowrap;
    }
  }

  .switch-field {
    display: flex;
    gap: 10px;
    align-items: center;
    min-height: 32px;
    color: var(--art-gray-600);
    font-size: 13px;
  }

  .date-range-field {
    width: 560px;
    max-width: 100%;
  }

  .field-tip {
    margin-top: 7px;
    color: var(--art-gray-500);
    font-size: 12px;
    line-height: 1.5;
  }

  .plan-selector {
    display: grid;
    width: 100%;
    max-height: 300px;
    padding: 2px;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
    overflow-y: auto;
    scrollbar-gutter: stable;
  }

  .plan-option {
    width: 100%;
    height: auto;
    min-height: 48px;
    margin: 0;
    padding: 12px 14px;
    border: 1px solid var(--el-border-color);
    border-radius: 9px;

    &.is-checked {
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary-light-5);
    }

    &.disabled {
      opacity: 0.72;
    }
  }

  .plan-option-name { margin-right: 8px; }
  .plan-option-price { color: var(--el-color-primary); font-weight: 600; }
  .plan-option-state { margin-left: 8px; color: var(--el-color-warning); font-size: 11px; }

  .plan-empty {
    width: 100%;
    padding: 20px;
    color: var(--art-gray-500);
    text-align: center;
    background: var(--art-bg-color);
    border: 1px dashed var(--el-border-color);
    border-radius: 10px;
  }

  .rule-panel {
    margin-top: 6px;
    padding: 16px;
    background: var(--art-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 12px;
  }

  .rule-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;

    > div {
      display: flex;
      flex-direction: column;
      gap: 3px;
    }

    span {
      color: var(--art-gray-500);
      font-size: 12px;
    }
  }

  .rule-row {
    display: grid;
    grid-template-columns: minmax(140px, 1fr) 140px 150px 145px;
    gap: 10px;
    align-items: center;
    padding: 11px 0;
    border-top: 1px solid var(--el-border-color-lighter);
  }

  .rule-value { width: 150px; }

  .rule-preview {
    color: var(--el-color-success);
    font-size: 12px;
    font-weight: 600;
    text-align: right;
  }

  @media (max-width: 1100px) {
    .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .toolbar { align-items: flex-start; flex-direction: column; }
    .filters { width: 100%; flex-wrap: wrap; }
  }

  @media (max-width: 720px) {
    .page-heading { gap: 16px; align-items: flex-start; flex-direction: column; }
    .summary-grid { grid-template-columns: 1fr; }
    .form-grid,
    .plan-selector { grid-template-columns: 1fr; }
    .date-range-field { width: 100%; }
    .rule-row { grid-template-columns: 1fr; }
    .rule-value,
    .rule-type { width: 100%; }
    .rule-preview { text-align: left; }
  }
</style>