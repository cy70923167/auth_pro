<template>
  <div class="license-list">
    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="域名/IP/密钥">
          <el-input
            v-model="searchForm.keyword"
            placeholder="请输入"
            clearable
            style="width: 200px"
          />
        </el-form-item>
        <el-form-item label="授权类型">
          <el-select v-model="searchForm.type" placeholder="全部" clearable style="width: 130px">
            <el-option label="单域名" value="domain" />
            <el-option label="泛域名" value="wildcard" />
            <el-option label="IP" value="ip" />
            <el-option label="密钥" value="key" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 120px">
            <el-option label="正常" value="active" />
            <el-option label="已过期" value="expired" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item label="应用">
          <el-select v-model="searchForm.appId" placeholder="全部" clearable style="width: 140px">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 操作栏 + 表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">授权列表</span>
          <el-button type="primary" @click="handleAdd">新增授权</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="domain" label="域名/IP/密钥" min-width="200" show-overflow-tooltip />
        <el-table-column label="归属账号" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="owner-cell">
              <el-tag :type="row.ownerType === 'agent' ? 'warning' : 'info'" size="small">
                {{ row.ownerType === 'agent' ? '代理' : '用户' }}
              </el-tag>
              <span>{{ row.ownerName || `ID ${row.ownerId}` }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="appName" label="应用" width="120" />
        <el-table-column prop="typeLabel" label="类型" width="90">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small">{{ row.typeLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="statusLabel" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTagMap[row.status]" size="small">{{ row.statusLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="expireAt" label="到期时间" width="160" />
        <el-table-column prop="verifyCount" label="验证次数" width="100" align="center" />
        <el-table-column label="站点" width="120" align="center">
          <template #default="{ row }">
            <span v-if="row.type === 'key'">
              {{ row.boundSites ?? 0 }} / {{ Number(row.maxSites) ? row.maxSites : '不限' }}
            </span>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="160" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.type === 'key'"
              link
              type="primary"
              size="small"
              @click="openSiteDialog(row)"
            >
              站点
            </el-button>
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" size="small" @click="handleToggle(row)">
              {{ row.status === 'active' ? '禁用' : '启用' }}
            </el-button>
            <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px" destroy-on-close>
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="100px">
        <template v-if="!isEdit">
          <el-form-item label="开通到" prop="ownerType">
            <el-segmented v-model="formData.ownerType" :options="ownerTypeOptions" block />
          </el-form-item>
          <el-form-item label="归属账号" prop="ownerId">
            <el-select
              v-model="formData.ownerId"
              filterable
              remote
              reserve-keyword
              clearable
              :remote-method="fetchOwnerOptions"
              :loading="ownerLoading"
              :placeholder="ownerSelectPlaceholder"
              style="width: 100%"
              @visible-change="handleOwnerSelectVisible"
            >
              <el-option
                v-for="owner in ownerOptions"
                :key="`${owner.type}-${owner.id}`"
                :label="formatOwnerOption(owner)"
                :value="owner.id"
              />
            </el-select>
            <div class="form-tip">可按名称或登录账号搜索，授权会直接显示在该账号下</div>
          </el-form-item>
        </template>
        <el-form-item label="应用" prop="appId">
          <el-select
            v-model="formData.appId"
            placeholder="请选择应用"
            style="width: 100%"
            :disabled="isEdit"
            @change="handleAppChange"
          >
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="!isEdit" label="套餐" prop="planId">
          <el-select
            v-model="formData.planId"
            :loading="planLoading"
            :disabled="!formData.appId"
            :placeholder="formData.appId ? '请选择套餐' : '请先选择应用'"
            no-data-text="该应用暂无可用套餐"
            style="width: 100%"
          >
            <el-option
              v-for="plan in planList"
              :key="plan.id"
              :label="`${plan.name}（${plan.durationText}，¥${Number(plan.price).toFixed(2)}）`"
              :value="plan.id"
            />
          </el-select>
          <div v-if="formData.appId && !planLoading && planList.length === 0" class="form-tip">
            该应用暂无启用套餐，请先在套餐管理中配置
          </div>
        </el-form-item>
        <el-form-item label="授权类型" prop="type">
          <el-radio-group v-model="formData.type">
            <el-radio value="domain">单域名</el-radio>
            <el-radio value="wildcard">泛域名</el-radio>
            <el-radio value="ip">IP地址</el-radio>
            <el-radio value="key">密钥</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="domainLabel" prop="domain">
          <el-input v-model="formData.domain" :placeholder="domainPlaceholder" />
          <div class="form-tip">{{ domainTip }}</div>
        </el-form-item>
        <el-form-item v-if="!isEdit" label="到期时间">
          <el-input
            :model-value="planExpireText"
            disabled
            :placeholder="formData.planId ? '' : '选择套餐后自动计算'"
          />
          <div class="form-tip">到期时间由所选套餐自动计算，实际时间以后端创建结果为准</div>
        </el-form-item>
        <el-form-item v-else label="到期时间" prop="expireAt">
          <el-date-picker
            v-model="formData.expireAt"
            type="datetime"
            placeholder="选择到期时间，留空为永久"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.remark" type="textarea" :rows="2" placeholder="可选备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>

    <!-- 密钥站点管理弹窗 -->
    <el-dialog v-model="siteDialog.visible" title="密钥绑定站点" width="680px" destroy-on-close>
      <el-alert
        v-if="siteDialog.maxSites > 0"
        :title="`当前已绑定 ${siteDialog.list.length} / ${siteDialog.maxSites} 个站点，达到上限后新站点验证会被拒绝，可解绑释放名额。`"
        type="info"
        show-icon
        :closable="false"
        class="mb-3"
      />
      <el-alert
        v-else
        title="该密钥不限制站点数量。"
        type="info"
        show-icon
        :closable="false"
        class="mb-3"
      />
      <el-table
        :data="siteDialog.list"
        size="small"
        v-loading="siteDialog.loading"
        max-height="360"
      >
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="row.targetType === 'ip' ? 'warning' : ''" size="small">
              {{ row.targetType === 'ip' ? 'IP' : '域名' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target" label="站点" min-width="160" show-overflow-tooltip />
        <el-table-column prop="serverIp" label="最近服务器IP" width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.serverIp || '--' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="firstSeenAt" label="首次绑定" width="160" />
        <el-table-column prop="lastSeenAt" label="最近验证" width="160" />
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="handleUnbindSite(row)"
              >解绑</el-button
            >
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无绑定站点" :image-size="60" />
        </template>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, computed, onBeforeUnmount, onMounted, watch } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import request from '@/utils/http'

  const loading = ref(false)
  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const submitting = ref(false)
  const ownerLoading = ref(false)
  const planLoading = ref(false)

  const dialogTitle = computed(() => (isEdit.value ? '编辑授权' : '新增授权'))

  const searchForm = reactive({
    keyword: '',
    type: '',
    status: '',
    appId: ''
  })

  const pagination = reactive({
    page: 1,
    pageSize: 10,
    total: 0
  })

  const appList = ref<{ id: string; name: string }[]>([])

  interface PlanOption {
    id: number
    name: string
    durationDays: number
    durationText: string
    price: number
  }

  const planList = ref<PlanOption[]>([])

  type OwnerType = 'user' | 'agent'

  interface OwnerOption {
    id: number
    name: string
    account: string
    type: OwnerType
  }

  const ownerTypeOptions = [
    { label: '用户账号', value: 'user' },
    { label: '代理账号', value: 'agent' }
  ]
  const ownerOptions = ref<OwnerOption[]>([])
  let ownerSearchTimer: ReturnType<typeof setTimeout> | undefined
  let ownerSearchSequence = 0
  let planRequestSequence = 0

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger' | undefined

  const typeTagMap: Record<string, TagType> = {
    domain: undefined,
    wildcard: 'success',
    ip: 'warning',
    key: 'info'
  }

  const statusTagMap: Record<string, TagType> = {
    active: 'success',
    expired: 'info',
    disabled: 'danger'
  }

  const tableData = ref<any[]>([])

  const formRef = ref()
  const formData = reactive({
    id: 0,
    appId: '',
    planId: null as number | null,
    ownerType: 'user' as OwnerType,
    ownerId: null as number | null,
    type: 'domain',
    domain: '',
    expireAt: '',
    remark: ''
  })

  const siteDialog = reactive({
    visible: false,
    loading: false,
    licenseId: 0,
    licenseNo: '',
    maxSites: 0,
    list: [] as any[]
  })

  function validateLicenseTarget(type: string, value: string) {
    const target = (value || '').trim().toLowerCase()
    if (type === 'key') return ''
    if (type === 'domain' && !isValidSingleDomain(target)) return '单域名格式不正确'
    if (type === 'wildcard' && (!target.startsWith('*.') || !isValidSingleDomain(target.slice(2))))
      return '泛域名格式不正确'
    if (type === 'ip' && !isValidIP(target)) return 'IP 格式不正确'
    return ''
  }

  function isValidSingleDomain(value: string) {
    if (
      !value ||
      value.startsWith('*.') ||
      value.endsWith('.') ||
      /[/:@\s]/.test(value) ||
      isValidIP(value)
    )
      return false
    const labels = value.split('.')
    if (labels.length < 2) return false
    if (!/^[a-z]{2,}$/.test(labels[labels.length - 1])) return false
    return labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))
  }

  function isValidIP(value: string) {
    const ipv4 = /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/
    const ipv6 = /^(([0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|::1|::)$/i
    return ipv4.test(value) || ipv6.test(value)
  }

  const validateDomainRule = (_rule: any, value: string, callback: (error?: Error) => void) => {
    const target = (value || '').trim()
    if (formData.type !== 'key' && !target) {
      callback(new Error('请输入授权值'))
      return
    }
    const message = validateLicenseTarget(formData.type, target)
    if (message) {
      callback(new Error(message))
      return
    }
    callback()
  }

  const formRules = {
    appId: [{ required: true, message: '请选择应用', trigger: 'change' }],
    planId: [{ required: true, message: '请选择套餐', trigger: 'change' }],
    ownerType: [{ required: true, message: '请选择开通对象', trigger: 'change' }],
    ownerId: [{ required: true, message: '请选择归属账号', trigger: 'change' }],
    type: [{ required: true, message: '请选择授权类型', trigger: 'change' }],
    domain: [{ validator: validateDomainRule, trigger: ['blur', 'change'] }]
  }

  const domainLabel = computed(() => {
    const map: Record<string, string> = {
      domain: '域名',
      wildcard: '泛域名',
      ip: 'IP地址',
      key: '密钥'
    }
    return map[formData.type] || '域名'
  })

  const domainPlaceholder = computed(() => {
    const map: Record<string, string> = {
      domain: '例: example.com',
      wildcard: '例: *.example.com',
      ip: '例: 192.168.1.1',
      key: '留空自动生成'
    }
    return map[formData.type] || ''
  })

  const domainTip = computed(() => {
    const map: Record<string, string> = {
      domain: '只填写根域名或具体单域名，不带 http://、端口和路径',
      wildcard: '必须以 *. 开头，例如 *.example.com',
      ip: '只支持标准 IPv4 或 IPv6 地址，不支持网段或范围',
      key: '不填写时系统自动生成密钥'
    }
    return map[formData.type] || ''
  })

  const planExpireText = computed(() => {
    const plan = planList.value.find((item) => item.id === formData.planId)
    if (!plan) return ''
    if (plan.durationDays <= 0) return '永久'

    const expireAt = new Date()
    expireAt.setDate(expireAt.getDate() + plan.durationDays)
    const pad = (value: number) => String(value).padStart(2, '0')
    return `${expireAt.getFullYear()}-${pad(expireAt.getMonth() + 1)}-${pad(expireAt.getDate())} ${pad(expireAt.getHours())}:${pad(expireAt.getMinutes())}:${pad(expireAt.getSeconds())}`
  })

  const ownerSelectPlaceholder = computed(() =>
    formData.ownerType === 'agent' ? '搜索并选择代理账号' : '搜索并选择用户账号'
  )

  watch(
    () => formData.type,
    () => {
      formRef.value?.clearValidate?.('domain')
      if (formData.domain) formRef.value?.validateField?.('domain')
    }
  )

  watch(
    () => formData.ownerType,
    () => {
      formData.ownerId = null
      ownerOptions.value = []
      formRef.value?.clearValidate?.('ownerId')
      if (!isEdit.value && dialogVisible.value) fetchOwnerOptions('')
    }
  )

  function formatOwnerOption(owner: OwnerOption) {
    return owner.name === owner.account ? owner.name : `${owner.name} (${owner.account})`
  }

  function fetchOwnerOptions(keyword = '') {
    if (ownerSearchTimer) clearTimeout(ownerSearchTimer)
    const sequence = ++ownerSearchSequence
    ownerSearchTimer = setTimeout(async () => {
      ownerLoading.value = true
      try {
        const data = await request.get<OwnerOption[]>({
          url: '/api/license/owners',
          params: {
            ownerType: formData.ownerType,
            keyword: keyword.trim(),
            limit: 30
          }
        })
        if (sequence === ownerSearchSequence) ownerOptions.value = data || []
      } catch {
        if (sequence === ownerSearchSequence) ownerOptions.value = []
      } finally {
        if (sequence === ownerSearchSequence) ownerLoading.value = false
      }
    }, 250)
  }

  function handleOwnerSelectVisible(visible: boolean) {
    if (visible && ownerOptions.value.length === 0) fetchOwnerOptions('')
  }

  async function fetchAppList() {
    try {
      const data = await request.get<any[]>({ url: '/api/license/apps' })
      appList.value = data || []
    } catch {
      appList.value = []
    }
  }

  async function fetchPlanList(appId: string | number) {
    const sequence = ++planRequestSequence
    planList.value = []
    if (!appId) return

    planLoading.value = true
    try {
      const data = await request.get<PlanOption[]>({
        url: '/api/plan/list',
        params: { appId: Number(appId), status: 'enabled' }
      })
      if (sequence === planRequestSequence) planList.value = data || []
    } catch {
      if (sequence === planRequestSequence) planList.value = []
    } finally {
      if (sequence === planRequestSequence) planLoading.value = false
    }
  }

  function handleAppChange(appId: string | number) {
    if (isEdit.value) return
    formData.planId = null
    formRef.value?.clearValidate?.('planId')
    fetchPlanList(appId)
  }

  async function handleSearch() {
    loading.value = true
    try {
      const params: Record<string, any> = {
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      if (searchForm.keyword) params.keyword = searchForm.keyword
      if (searchForm.type) params.type = searchForm.type
      if (searchForm.status) params.status = searchForm.status
      if (searchForm.appId) params.appId = searchForm.appId

      const data = await request.get<{ list: any[]; total: number }>({
        url: '/api/license/list',
        params
      })
      tableData.value = data.list || []
      pagination.total = data.total || 0
    } catch (e) {
      console.error('[LicenseList] 查询失败:', e)
    } finally {
      loading.value = false
    }
  }

  function handleReset() {
    searchForm.keyword = ''
    searchForm.type = ''
    searchForm.status = ''
    searchForm.appId = ''
    pagination.page = 1
    handleSearch()
  }

  function handleAdd() {
    isEdit.value = false
    formData.id = 0
    formData.appId = ''
    formData.planId = null
    formData.ownerType = 'user'
    formData.ownerId = null
    formData.type = 'domain'
    formData.domain = ''
    formData.expireAt = ''
    formData.remark = ''
    planRequestSequence++
    planList.value = []
    planLoading.value = false
    ownerOptions.value = []
    dialogVisible.value = true
    fetchOwnerOptions('')
  }

  function handleEdit(row: any) {
    isEdit.value = true
    formData.id = row.id
    formData.appId = String(row.appId)
    formData.planId = null
    formData.type = row.type
    formData.domain = row.domain
    formData.expireAt = row.expireAt
    formData.remark = row.remark
    planRequestSequence++
    planList.value = []
    planLoading.value = false
    dialogVisible.value = true
  }

  async function handleToggle(row: any) {
    const newStatus = row.status === 'active' ? 'disabled' : 'active'
    const action = row.status === 'active' ? '禁用' : '启用'
    try {
      await ElMessageBox.confirm(`确定${action}该授权？`, '提示', { type: 'warning' })
      await request.put({ url: `/api/license/${row.id}/toggle`, params: { status: newStatus } })
      ElMessage.success(`${action}成功`)
      handleSearch()
    } catch (error) {
      if (error !== 'cancel' && error !== 'close') {
        console.error(`[LicenseList] ${action}失败:`, error)
      }
    }
  }

  async function handleDelete(row: any) {
    try {
      await ElMessageBox.confirm('确定删除该授权？删除后不可恢复', '警告', { type: 'error' })
      await request.del({ url: `/api/license/${row.id}` })
      ElMessage.success('删除成功')
      handleSearch()
    } catch (error) {
      if (error !== 'cancel' && error !== 'close') {
        console.error('[LicenseList] 删除失败:', error)
      }
    }
  }

  async function openSiteDialog(row: any) {
    siteDialog.licenseId = Number(row.id)
    siteDialog.licenseNo = row.licenseNo || ''
    siteDialog.maxSites = Number(row.maxSites) || 0
    siteDialog.visible = true
    await fetchLicenseSites()
  }

  async function fetchLicenseSites() {
    siteDialog.loading = true
    try {
      const data = await request.get<{ list: any[]; boundSites: number; maxSites: number }>({
        url: `/api/license/${siteDialog.licenseId}/sites`
      })
      siteDialog.list = data?.list || []
      if (data?.maxSites !== undefined) siteDialog.maxSites = Number(data.maxSites)
    } catch (error) {
      console.error('[LicenseList] 加载绑定站点失败:', error)
      ElMessage.error('加载绑定站点失败')
    } finally {
      siteDialog.loading = false
    }
  }

  async function handleUnbindSite(row: any) {
    try {
      await ElMessageBox.confirm(`确定解绑站点「${row.target}」？解绑后名额立即释放。`, '提示', {
        type: 'warning'
      })
      await request.del({ url: `/api/license/${siteDialog.licenseId}/sites/${row.id}` })
      ElMessage.success('解绑成功')
      await fetchLicenseSites()
      handleSearch()
    } catch (error) {
      if (error !== 'cancel' && error !== 'close') {
        console.error('[LicenseList] 解绑失败:', error)
      }
    }
  }

  async function handleSubmit() {
    if (submitting.value) return
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    submitting.value = true
    try {
      if (isEdit.value) {
        await request.put({
          url: `/api/license/${formData.id}`,
          params: {
            appId: Number(formData.appId),
            type: formData.type,
            domain: formData.domain,
            expireAt: formData.expireAt,
            remark: formData.remark
          }
        })
        ElMessage.success('编辑成功')
      } else {
        await request.post({
          url: '/api/license/create',
          params: {
            appId: Number(formData.appId),
            planId: Number(formData.planId),
            ownerType: formData.ownerType,
            ownerId: formData.ownerId,
            type: formData.type,
            domain: formData.domain,
            remark: formData.remark
          }
        })
        ElMessage.success('新增成功')
      }
      dialogVisible.value = false
      handleSearch()
    } catch (e) {
      console.error('[LicenseList] 提交失败:', e)
    } finally {
      submitting.value = false
    }
  }

  onMounted(() => {
    fetchAppList()
    handleSearch()
  })

  onBeforeUnmount(() => {
    if (ownerSearchTimer) clearTimeout(ownerSearchTimer)
    ownerSearchSequence++
    planRequestSequence++
  })
</script>

<style scoped lang="scss">
  .license-list {
    padding: 0;
  }

  .mb-4 {
    margin-bottom: 16px;
  }

  .mb-3 {
    margin-bottom: 12px;
  }

  .table-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .card-title {
    font-weight: 600;
  }

  .pagination-wrapper {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .form-tip {
    margin-top: 4px;
    font-size: 12px;
    line-height: 18px;
    color: var(--el-text-color-secondary);
  }

  .owner-cell {
    display: flex;
    gap: 8px;
    align-items: center;
    min-width: 0;

    span:last-child {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
</style>
