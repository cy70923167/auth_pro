<template>
  <div class="license-plans">
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">套餐管理</span>
          <el-button type="primary" @click="handleAdd">新增套餐</el-button>
        </div>
      </template>

      <div class="search-bar">
        <el-select
          v-model="query.appId"
          placeholder="选择应用"
          clearable
          style="width: 220px"
          @change="fetchList"
        >
          <el-option v-for="app in appOptions" :key="app.id" :label="app.name" :value="app.id" />
        </el-select>
        <el-input
          v-model="query.keyword"
          placeholder="套餐名/应用名"
          clearable
          style="width: 220px"
          @keyup.enter="fetchList"
        />
        <el-select
          v-model="query.status"
          placeholder="状态"
          clearable
          style="width: 140px"
          @change="fetchList"
        >
          <el-option label="启用" value="enabled" />
          <el-option label="禁用" value="disabled" />
        </el-select>
        <el-button type="primary" @click="fetchList">查询</el-button>
        <el-button @click="handleReset">重置</el-button>
      </div>

      <el-table :data="tableData" stripe>
        <el-table-column prop="appName" label="应用" min-width="150" />
        <el-table-column prop="name" label="套餐名称" min-width="140" />
        <el-table-column prop="licenseType" label="授权方式" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.licenseType" size="small" effect="plain">{{
              licenseTypeLabel(row.licenseType)
            }}</el-tag>
            <span v-else class="text-secondary">通用</span>
          </template>
        </el-table-column>
        <el-table-column prop="durationText" label="授权时长" width="120" />
        <el-table-column prop="price" label="价格" width="120">
          <template #default="{ row }">¥{{ Number(row.price || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="最大站点数" width="110" align="center">
          <template #default="{ row }">
            <span v-if="row.licenseType === 'key'">{{ Number(row.maxSites || 0) || '不限' }}</span>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="80" align="center" />
        <el-table-column prop="enabled" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="创建时间" width="160" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" size="small" @click="handleToggle(row)">
              {{ row.enabled ? '禁用' : '启用' }}
            </el-button>
            <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="90px">
        <el-form-item label="所属应用" prop="appId">
          <el-select
            v-model="formData.appId"
            placeholder="请选择应用"
            style="width: 100%"
            @change="handleAppChange"
          >
            <el-option v-for="app in appOptions" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="套餐名称" prop="name">
          <el-input v-model="formData.name" placeholder="例如：1个月、3个月、1年、永久" />
        </el-form-item>
        <el-form-item label="授权方式" prop="licenseType">
          <el-select
            v-model="formData.licenseType"
            placeholder="请选择授权方式"
            clearable
            style="width: 100%"
            :disabled="!formData.appId || availableLicenseTypes.length === 0"
          >
            <el-option label="通用（当前应用全部授权方式）" value="" />
            <el-option
              v-for="licenseType in availableLicenseTypes"
              :key="licenseType"
              :label="licenseTypeLabels[licenseType] || licenseType"
              :value="licenseType"
            />
          </el-select>
          <div class="form-tip" style="margin-top: 4px; margin-left: 0">{{ licenseTypeTip }}</div>
        </el-form-item>
        <el-form-item label="快捷时长">
          <el-space wrap>
            <el-button size="small" @click="applyPreset('1个月', 30)">1个月</el-button>
            <el-button size="small" @click="applyPreset('3个月', 90)">3个月</el-button>
            <el-button size="small" @click="applyPreset('1年', 365)">1年</el-button>
            <el-button size="small" @click="applyPreset('永久', 0)">永久</el-button>
          </el-space>
        </el-form-item>
        <el-form-item label="授权天数" prop="durationDays">
          <el-input-number
            v-model="formData.durationDays"
            :min="0"
            :precision="0"
            :step="30"
            controls-position="right"
          />
          <span class="form-tip">0 表示永久</span>
        </el-form-item>
        <el-form-item label="价格" prop="price">
          <el-input-number
            v-model="formData.price"
            :min="0"
            :precision="2"
            :step="10"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item v-if="formData.licenseType === 'key'" label="最大站点数">
          <el-input-number
            v-model="formData.maxSites"
            :min="0"
            :precision="0"
            :step="1"
            controls-position="right"
          />
          <span class="form-tip">0 表示不限制</span>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
            v-model="formData.sort"
            :precision="0"
            :step="10"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="formData.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.remark" type="textarea" :rows="2" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, reactive, ref } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import request from '@/utils/http'

  const tableData = ref<any[]>([])
  const appOptions = ref<any[]>([])
  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const formRef = ref()

  const query = reactive({
    appId: undefined as number | undefined,
    keyword: '',
    status: ''
  })

  const formData = reactive({
    id: 0,
    appId: undefined as number | undefined,
    name: '',
    licenseType: '',
    durationDays: 30,
    price: 0,
    maxSites: 0,
    sort: 0,
    enabled: true,
    remark: ''
  })

  const licenseTypeLabels: Record<string, string> = {
    domain: '单域名授权',
    wildcard: '泛域名授权',
    ip: 'IP 授权',
    key: '密钥授权'
  }

  const selectedApp = computed(() => appOptions.value.find((app) => app.id === formData.appId))
  const availableLicenseTypes = computed<string[]>(() =>
    Array.isArray(selectedApp.value?.purchaseLicenseTypes)
      ? selectedApp.value.purchaseLicenseTypes
      : []
  )
  const licenseTypeTip = computed(() => {
    if (!formData.appId) return '请先选择所属应用'
    if (availableLicenseTypes.value.length === 0)
      return '该应用未配置可用授权方式，请先在应用管理中配置'
    return `仅显示当前应用已配置的授权方式：${availableLicenseTypes.value.map(licenseTypeLabel).join('、')}`
  })

  function licenseTypeLabel(value: string): string {
    return licenseTypeLabels[value] || value
  }

  const dialogTitle = computed(() => (isEdit.value ? '编辑套餐' : '新增套餐'))

  const formRules = {
    appId: [{ required: true, message: '请选择应用', trigger: 'change' }],
    name: [{ required: true, message: '请输入套餐名称', trigger: 'blur' }],
    durationDays: [{ required: true, message: '请输入授权天数', trigger: 'blur' }],
    price: [{ required: true, message: '请输入价格', trigger: 'blur' }]
  }

  async function fetchApps() {
    const data = await request.get<any[]>({ url: '/api/app/list' })
    appOptions.value = data || []
  }

  async function fetchList() {
    const data = await request.get<any[]>({
      url: '/api/plan/list',
      params: {
        appId: query.appId,
        keyword: query.keyword,
        status: query.status
      }
    })
    tableData.value = data || []
  }

  function handleReset() {
    query.appId = undefined
    query.keyword = ''
    query.status = ''
    fetchList()
  }

  function resetForm() {
    Object.assign(formData, {
      id: 0,
      appId: undefined,
      name: '',
      licenseType: '',
      durationDays: 30,
      price: 0,
      maxSites: 0,
      sort: 0,
      enabled: true,
      remark: ''
    })
  }

  function handleAppChange() {
    if (formData.licenseType && !availableLicenseTypes.value.includes(formData.licenseType)) {
      formData.licenseType = ''
    }
    formRef.value?.clearValidate('licenseType')
  }

  function applyPreset(name: string, days: number) {
    formData.name = name
    formData.durationDays = days
  }

  function handleAdd() {
    isEdit.value = false
    resetForm()
    dialogVisible.value = true
  }

  function handleEdit(row: any) {
    isEdit.value = true
    Object.assign(formData, {
      id: row.id,
      appId: row.appId,
      name: row.name,
      licenseType: row.licenseType || '',
      durationDays: row.durationDays,
      price: Number(row.price || 0),
      maxSites: Number(row.maxSites || 0),
      sort: row.sort || 0,
      enabled: row.enabled,
      remark: row.remark || ''
    })
    handleAppChange()
    dialogVisible.value = true
  }

  async function handleSubmit() {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    const payload = {
      appId: formData.appId,
      name: formData.name,
      licenseType: formData.licenseType,
      durationDays: formData.durationDays,
      price: formData.price,
      maxSites: formData.maxSites,
      sort: formData.sort,
      enabled: formData.enabled,
      remark: formData.remark
    }

    if (isEdit.value) {
      await request.put({ url: `/api/plan/${formData.id}`, data: payload })
      ElMessage.success('编辑成功')
    } else {
      await request.post({ url: '/api/plan/create', data: payload })
      ElMessage.success('新增成功')
    }
    dialogVisible.value = false
    fetchList()
  }

  async function handleToggle(row: any) {
    await request.put({ url: `/api/plan/${row.id}/toggle` })
    ElMessage.success('操作成功')
    fetchList()
  }

  async function handleDelete(row: any) {
    try {
      await ElMessageBox.confirm(`确定删除套餐「${row.name}」？`, '删除套餐', { type: 'warning' })
      await request.del({ url: `/api/plan/${row.id}` })
      ElMessage.success('删除成功')
      fetchList()
    } catch {
      return
    }
  }

  onMounted(async () => {
    await fetchApps()
    fetchList()
  })
</script>

<style scoped lang="scss">
  .license-plans {
    .table-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .card-title {
      font-weight: 600;
    }

    .search-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      margin-bottom: 16px;
    }

    .form-tip {
      margin-left: 12px;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    .text-secondary {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }
  }
</style>
