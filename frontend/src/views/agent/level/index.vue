<template>
  <div class="agent-level-page">
    <el-card shadow="hover" class="mb-4 search-card">
      <el-form :model="searchForm" inline>
        <el-form-item label="等级关键词">
          <el-input
            v-model.trim="searchForm.keyword"
            placeholder="编码 / 名称"
            clearable
            class="search-input"
            @keyup.enter="handleSearch()"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="searchForm.status"
            placeholder="全部状态"
            clearable
            class="status-select"
          >
            <el-option label="启用" value="enabled" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleSearch()">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-row :gutter="16" class="mb-4 level-overview">
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card primary">
          <span class="stat-label">等级总数</span>
          <strong>{{ stats.total }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card success">
          <span class="stat-label">当前页启用</span>
          <strong>{{ stats.enabled }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card warning">
          <span class="stat-label">最低折扣</span>
          <strong>{{ stats.minDiscount ? `${stats.minDiscount}折` : '-' }}</strong>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card info">
          <span class="stat-label">绑定代理商</span>
          <strong>{{ stats.agentCount }}</strong>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover" class="table-card">
      <template #header>
        <div class="table-header">
          <div>
            <span class="card-title">代理商等级</span>
            <p class="card-desc"
              >等级决定代理商开通授权时的折扣；修改折扣后，会同步更新已绑定该等级的代理商。</p
            >
          </div>
          <el-button type="primary" @click="handleAdd">新增等级</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading" row-key="id">
        <el-table-column prop="name" label="等级" min-width="180">
          <template #default="{ row }">
            <div class="level-name">
              <el-tag :type="levelTagType(row.discount)" size="small">{{ row.name }}</el-tag>
              <span class="level-code">{{ row.code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="discount" label="折扣" width="110" align="center">
          <template #default="{ row }">
            <span class="discount-text">{{ formatDiscount(row.discount) }}折</span>
          </template>
        </el-table-column>
        <el-table-column prop="selfServiceEnabled" label="用户自助开通" width="130" align="center">
          <template #default="{ row }">
            <el-tag :type="row.selfServiceEnabled ? 'success' : 'info'" size="small" effect="plain">
              {{ row.selfServiceEnabled ? '已开放' : '未开放' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="upgradePrice" label="开通价格" width="120" align="right">
          <template #default="{ row }">
            <span v-if="row.selfServiceEnabled" class="price-text"
              >¥{{ Number(row.upgradePrice).toFixed(2) }}</span
            >
            <span v-else class="remark-text">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="openingBonus" label="开通赠送" width="120" align="right">
          <template #default="{ row }">
            <span v-if="Number(row.openingBonus) > 0" class="bonus-text"
              >+ ¥{{ Number(row.openingBonus).toFixed(2) }}</span
            >
            <span v-else class="remark-text">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="agentCount" label="绑定代理商" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.agentCount > 0 ? 'primary' : 'info'" size="small" effect="plain">
              {{ row.agentCount }} 个
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="90" align="center" />
        <el-table-column prop="enabled" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="remark-text">{{ row.remark || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="updatedAt" label="更新时间" width="160" />
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-tooltip
              :disabled="row.enabled || row.agentCount === 0"
              content="已有代理商使用，不能禁用"
              placement="top"
            >
              <span>
                <el-button
                  link
                  type="primary"
                  size="small"
                  :disabled="row.enabled && row.agentCount > 0"
                  @click="handleToggle(row)"
                >
                  {{ row.enabled ? '禁用' : '启用' }}
                </el-button>
              </span>
            </el-tooltip>
            <el-tooltip
              :disabled="row.agentCount === 0"
              content="已有代理商使用，不能删除"
              placement="top"
            >
              <span>
                <el-button
                  link
                  type="danger"
                  size="small"
                  :disabled="row.agentCount > 0"
                  @click="handleDelete(row)"
                >
                  删除
                </el-button>
              </span>
            </el-tooltip>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无代理商等级">
            <el-button type="primary" @click="handleAdd">新增第一个等级</el-button>
          </el-empty>
        </template>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleSearch()"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="560px"
      destroy-on-close
      @closed="resetFormValidate"
    >
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="120px">
        <el-form-item label="等级编码" prop="code">
          <el-input
            v-model.trim="formData.code"
            :disabled="isEdit"
            placeholder="例如 gold、vip_1"
          />
          <div class="form-tip">编码用于代理商等级绑定，保存后不可修改。</div>
        </el-form-item>
        <el-form-item label="等级名称" prop="name">
          <el-input
            v-model.trim="formData.name"
            placeholder="例如 金牌代理"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="等级折扣" prop="discount">
          <el-input-number
            v-model="formData.discount"
            :min="1"
            :max="10"
            :step="0.1"
            :precision="1"
          />
          <span class="form-unit">折</span>
          <span class="form-tip inline">数值越小，代理商拿货价格越低。</span>
        </el-form-item>
        <el-form-item label="用户自助开通">
          <el-switch
            v-model="formData.selfServiceEnabled"
            active-text="允许"
            inactive-text="关闭"
          />
          <div class="form-tip">默认关闭；仅开启后，该等级才会出现在用户端开通代理页面。</div>
        </el-form-item>
        <el-form-item label="开通价格" prop="upgradePrice">
          <el-input-number
            v-model="formData.upgradePrice"
            :min="0"
            :max="9999999999.99"
            :step="100"
            :precision="2"
            :disabled="!formData.selfServiceEnabled"
          />
          <span class="form-unit">元</span>
        </el-form-item>
        <el-form-item label="开通赠送余额" prop="openingBonus">
          <el-input-number
            v-model="formData.openingBonus"
            :min="0"
            :max="9999999999.99"
            :step="100"
            :precision="2"
          />
          <span class="form-unit">元</span>
          <div class="form-tip">用户成功自助开通后一次性发放，修改不影响已有订单。</div>
        </el-form-item>
        <el-form-item label="等级权益">
          <el-input
            v-model="formData.benefits"
            type="textarea"
            :rows="4"
            maxlength="1000"
            show-word-limit
            placeholder="每行填写一项权益，将展示在用户端等级卡片中"
          />
        </el-form-item>
        <el-form-item label="排序" prop="sort">
          <el-input-number v-model="formData.sort" :min="0" :max="999" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch
            v-model="formData.enabled"
            :disabled="isEdit && formData.agentCount > 0"
            active-text="启用"
            inactive-text="禁用"
          />
          <div v-if="isEdit && formData.agentCount > 0" class="form-tip">
            当前等级已绑定 {{ formData.agentCount }} 个代理商，不能直接禁用。
          </div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="formData.remark"
            type="textarea"
            :rows="3"
            maxlength="255"
            show-word-limit
            placeholder="可选备注"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, reactive, ref } from 'vue'
  import type { FormInstance, FormRules } from 'element-plus'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import request from '@/utils/http'

  type LevelStatus = '' | 'enabled' | 'disabled'

  type AgentLevel = {
    id: number
    code: string
    name: string
    discount: number
    selfServiceEnabled: boolean
    upgradePrice: number
    openingBonus: number
    benefits: string
    sort: number
    enabled: boolean
    remark: string
    agentCount: number
    createdAt?: string
    updatedAt: string
  }

  type LevelForm = {
    id: number
    code: string
    name: string
    discount: number
    selfServiceEnabled: boolean
    upgradePrice: number
    openingBonus: number
    benefits: string
    sort: number
    enabled: boolean
    remark: string
    agentCount: number
  }

  const defaultForm: LevelForm = {
    id: 0,
    code: '',
    name: '',
    discount: 9,
    selfServiceEnabled: false,
    upgradePrice: 0,
    openingBonus: 0,
    benefits: '',
    sort: 0,
    enabled: true,
    remark: '',
    agentCount: 0
  }

  const loading = ref(false)
  const submitLoading = ref(false)
  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const formRef = ref<FormInstance>()
  const tableData = ref<AgentLevel[]>([])

  const dialogTitle = computed(() => (isEdit.value ? '编辑等级' : '新增等级'))
  const searchForm = reactive<{ keyword: string; status: LevelStatus }>({ keyword: '', status: '' })
  const pagination = reactive({ page: 1, pageSize: 10, total: 0 })
  const formData = reactive<LevelForm>({ ...defaultForm })

  const formRules: FormRules<LevelForm> = {
    code: [
      { required: true, message: '请输入等级编码', trigger: 'blur' },
      {
        pattern: /^[A-Za-z0-9_-]{2,50}$/,
        message: '仅支持字母、数字、下划线和中划线，长度 2-50 位',
        trigger: 'blur'
      }
    ],
    name: [{ required: true, message: '请输入等级名称', trigger: 'blur' }],
    discount: [{ required: true, message: '请输入折扣', trigger: 'change' }],
    upgradePrice: [
      {
        validator: (_rule, value, callback) => {
          if (formData.selfServiceEnabled && Number(value) < 0.01) {
            callback(new Error('允许用户自助开通时，价格不能低于 0.01 元'))
            return
          }
          callback()
        },
        trigger: 'change'
      }
    ],
    sort: [{ required: true, message: '请输入排序', trigger: 'change' }]
  }

  const stats = computed(() => {
    const discounts = tableData.value.map((item) => Number(item.discount || 0)).filter(Boolean)
    return {
      total: pagination.total,
      enabled: tableData.value.filter((item) => item.enabled).length,
      minDiscount: discounts.length ? Math.min(...discounts) : 0,
      agentCount: tableData.value.reduce((total, item) => total + Number(item.agentCount || 0), 0)
    }
  })

  function formatDiscount(discount: number) {
    return Number(discount || 0)
      .toFixed(1)
      .replace(/\.0$/, '')
  }

  function levelTagType(discount: number) {
    if (discount <= 7) return 'warning'
    if (discount <= 8) return 'success'
    return 'info'
  }

  function normalizeLevel(item: AgentLevel): AgentLevel {
    return {
      ...item,
      discount: Number(item.discount || 0),
      selfServiceEnabled: Boolean(item.selfServiceEnabled),
      upgradePrice: Number(item.upgradePrice || 0),
      openingBonus: Number(item.openingBonus || 0),
      benefits: item.benefits || '',
      sort: Number(item.sort || 0),
      agentCount: Number(item.agentCount || 0),
      remark: item.remark || ''
    }
  }

  async function handleSearch() {
    loading.value = true
    try {
      const params: Record<string, string | number> = {
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      if (searchForm.keyword) params.keyword = searchForm.keyword
      if (searchForm.status) params.status = searchForm.status

      const data = await request.get<{ list: AgentLevel[]; total: number }>({
        url: '/api/agent-level/list',
        params
      })
      tableData.value = (data.list || []).map(normalizeLevel)
      pagination.total = Number(data.total || 0)
    } finally {
      loading.value = false
    }
  }

  function handleReset() {
    searchForm.keyword = ''
    searchForm.status = ''
    pagination.page = 1
    handleSearch()
  }

  function handleSizeChange() {
    pagination.page = 1
    handleSearch()
  }

  function resetFormValidate() {
    formRef.value?.clearValidate()
  }

  function handleAdd() {
    isEdit.value = false
    Object.assign(formData, defaultForm)
    dialogVisible.value = true
  }

  function handleEdit(row: AgentLevel) {
    isEdit.value = true
    Object.assign(formData, {
      id: row.id,
      code: row.code,
      name: row.name,
      discount: Number(row.discount),
      selfServiceEnabled: row.selfServiceEnabled,
      upgradePrice: Number(row.upgradePrice || 0),
      openingBonus: Number(row.openingBonus || 0),
      benefits: row.benefits || '',
      sort: row.sort,
      enabled: row.enabled,
      remark: row.remark,
      agentCount: row.agentCount
    })
    dialogVisible.value = true
  }

  function buildPayload() {
    return {
      code: formData.code.trim(),
      name: formData.name.trim(),
      discount: Number(formData.discount),
      selfServiceEnabled: formData.selfServiceEnabled,
      upgradePrice: Number(formData.upgradePrice || 0),
      openingBonus: Number(formData.openingBonus || 0),
      benefits: formData.benefits.trim(),
      sort: Number(formData.sort || 0),
      enabled: formData.enabled,
      remark: formData.remark.trim()
    }
  }

  async function handleSubmit() {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    submitLoading.value = true
    try {
      const payload = buildPayload()
      if (isEdit.value) {
        await request.put({ url: `/api/agent-level/${formData.id}`, params: payload })
        ElMessage.success('编辑成功，已同步该等级下的代理商折扣')
      } else {
        await request.post({ url: '/api/agent-level/create', params: payload })
        ElMessage.success('新增成功')
      }
      dialogVisible.value = false
      handleSearch()
    } finally {
      submitLoading.value = false
    }
  }

  async function handleToggle(row: AgentLevel) {
    if (row.enabled && row.agentCount > 0) {
      ElMessage.warning('该等级已有代理商使用，不能禁用')
      return
    }

    const action = row.enabled ? '禁用' : '启用'
    try {
      await ElMessageBox.confirm(`确定${action}等级「${row.name}」？`, '提示', { type: 'warning' })
      await request.put({
        url: `/api/agent-level/${row.id}`,
        params: {
          name: row.name,
          discount: Number(row.discount),
          selfServiceEnabled: row.selfServiceEnabled,
          upgradePrice: Number(row.upgradePrice || 0),
          openingBonus: Number(row.openingBonus || 0),
          benefits: row.benefits || '',
          sort: row.sort,
          enabled: !row.enabled,
          remark: row.remark
        }
      })
      ElMessage.success(`${action}成功`)
      handleSearch()
    } catch {
      return
    }
  }

  async function handleDelete(row: AgentLevel) {
    if (row.agentCount > 0) {
      ElMessage.warning('该等级已有代理商使用，不能删除')
      return
    }

    try {
      await ElMessageBox.confirm(`删除等级「${row.name}」后不可恢复，确定继续？`, '危险操作', {
        type: 'error',
        confirmButtonText: '确认删除',
        confirmButtonClass: 'el-button--danger'
      })
      await request.del({ url: `/api/agent-level/${row.id}` })
      ElMessage.success('删除成功')
      handleSearch()
    } catch {
      return
    }
  }

  onMounted(() => {
    handleSearch()
  })
</script>

<style scoped lang="scss">
  .agent-level-page {
    .mb-4 {
      margin-bottom: 16px;
    }

    .search-card {
      :deep(.el-card__body) {
        padding-bottom: 2px;
      }
    }

    .search-input {
      width: 220px;
    }

    .status-select {
      width: 140px;
    }

    .level-overview {
      row-gap: 16px;
    }

    .table-header {
      display: flex;
      gap: 16px;
      align-items: center;
      justify-content: space-between;
    }

    .card-title {
      font-size: 16px;
      font-weight: 600;
    }

    .card-desc {
      margin: 6px 0 0;
      font-size: 12px;
      line-height: 1.5;
      color: var(--el-text-color-secondary);
    }

    .level-name {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    .level-code {
      font-family: 'Roboto Mono', monospace;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    .discount-text {
      font-weight: 700;
      color: var(--el-color-primary);
    }

    .bonus-text {
      font-weight: 600;
      color: var(--el-color-success);
    }

    .remark-text {
      color: var(--el-text-color-regular);
    }

    .pagination-wrapper {
      display: flex;
      justify-content: flex-end;
      margin-top: 16px;
    }

    .form-unit {
      margin-left: 8px;
      color: var(--el-text-color-secondary);
    }

    .form-tip {
      margin-top: 4px;
      font-size: 12px;
      line-height: 1.4;
      color: var(--el-text-color-secondary);

      &.inline {
        margin-top: 0;
        margin-left: 12px;
      }
    }
  }

  .stat-card {
    :deep(.el-card__body) {
      display: flex;
      align-items: center;
      justify-content: space-between;
      min-height: 64px;
    }

    .stat-label {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }

    strong {
      font-size: 24px;
      color: var(--el-text-color-primary);
    }

    &.primary strong {
      color: var(--el-color-primary);
    }

    &.success strong {
      color: var(--el-color-success);
    }

    &.warning strong {
      color: var(--el-color-warning);
    }

    &.info strong {
      color: var(--el-color-info);
    }
  }

  @media (width <= 768px) {
    .agent-level-page {
      .table-header {
        flex-direction: column;
        align-items: flex-start;
      }

      .search-input,
      .status-select {
        width: 100%;
      }
    }
  }
</style>
