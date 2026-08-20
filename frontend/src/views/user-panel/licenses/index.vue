<template>
  <div class="user-licenses">
    <!-- 筛选 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="搜索">
          <el-input
            v-model="searchForm.keyword"
            placeholder="域名/IP/密钥"
            clearable
            style="width: 180px"
          />
        </el-form-item>
        <el-form-item label="应用">
          <el-select v-model="searchForm.appId" placeholder="全部" clearable style="width: 130px">
            <el-option v-for="app in appList" :key="app.id" :label="app.name" :value="app.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 110px">
            <el-option label="正常" value="active" />
            <el-option label="即将到期" value="expiring" />
            <el-option label="已过期" value="expired" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 授权列表 -->
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="card-title">我的授权</span>
          <el-button type="primary" @click="openRedeemDialog">兑换卡密</el-button>
        </div>
      </template>
      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column label="域名/IP/密钥" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="row.bindingPending" type="warning" size="small">待绑定</el-tag>
            <span v-else>{{ row.domain || '--' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="appName" label="应用" width="110" />
        <el-table-column prop="typeLabel" label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small">{{ row.typeLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="statusLabel" label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagMap[row.status]" size="small">{{ row.statusLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="expireAt" label="到期时间" width="130" />
        <el-table-column prop="createdAt" label="开通时间" width="130" />
        <el-table-column prop="source" label="来源" width="100">
          <template #default="{ row }">
            <span class="source-text">{{ row.source }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="套餐原价" width="110" align="right">
          <template #default="{ row }">
            <span>{{ formatLicenseAmount(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="站点" width="110" align="center">
          <template #default="{ row }">
            <el-button
              v-if="row.type === 'key'"
              link
              type="primary"
              size="small"
              @click="openSiteDialog(row)"
            >
              已绑定 {{ row.boundSites ?? 0 }}{{ Number(row.maxSites) ? ` / ${row.maxSites}` : '' }}
            </el-button>
            <span v-else class="text-secondary">--</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              v-if="canEditTargetType(row.type)"
              link
              type="primary"
              size="small"
              @click="openEditDialog(row)"
            >
              {{ row.bindingPending ? '绑定目标' : '修改目标' }}
            </el-button>
            <el-text v-else-if="row.type !== 'key'" type="info" size="small">不可修改</el-text>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="fetchList"
          @size-change="fetchList"
        />
      </div>
    </el-card>

    <el-dialog v-model="editDialog.visible" title="修改授权目标" width="420px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="授权类型">
          <el-tag :type="typeTagMap[editDialog.type]" size="small">{{
            editDialog.typeLabel
          }}</el-tag>
        </el-form-item>
        <el-form-item :label="targetLabel">
          <div class="target-editor">
            <el-input
              v-model="editDialog.target"
              :placeholder="targetPlaceholder"
              :disabled="editDialog.type === 'key'"
              :clearable="editDialog.type !== 'key'"
            />
            <el-button
              v-if="editDialog.type === 'key'"
              type="primary"
              plain
              :loading="editDialog.refreshing"
              title="生成新的16位密钥"
              @click="resetLicenseKey"
            >
              <iconify-icon icon="ri:refresh-line" width="16" />
              重置
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialog.visible = false">取消</el-button>
        <el-button
          v-if="editDialog.type !== 'key'"
          type="primary"
          :loading="editDialog.submitting"
          @click="submitEditTarget"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

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

    <el-dialog v-model="redeemDialog.visible" title="兑换卡密" width="460px" destroy-on-close>
      <el-alert
        title="兑换后授权将归当前账号，不能转给他人。"
        type="info"
        show-icon
        :closable="false"
        class="redeem-alert"
      />
      <el-form label-width="72px" @submit.prevent="submitRedeem">
        <el-form-item label="卡密">
          <el-input
            v-model="redeemDialog.cardCode"
            placeholder="请输入卡密"
            maxlength="64"
            clearable
            autocomplete="off"
            @keyup.enter="submitRedeem"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="redeemDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="redeemDialog.submitting" @click="submitRedeem"
          >确认兑换</el-button
        >
      </template>
    </el-dialog>

    <el-dialog v-model="redeemResult.visible" title="兑换结果" width="520px" destroy-on-close>
      <el-result icon="success" :title="redeemResult.idempotent ? '该卡密已兑换' : '卡密兑换成功'">
        <template #sub-title>
          <div class="redeem-summary">
            <div>{{ redeemResult.appName }} · {{ redeemResult.planName }}</div>
            <div>授权编号：{{ redeemResult.licenseNo }}</div>
            <div>授权类型：{{ redeemResult.typeLabel }} 有效期至：{{ redeemResult.expireAt }}</div>
          </div>
        </template>
        <template #extra>
          <div v-if="redeemResult.type === 'key'" class="license-key-result">
            <el-input :model-value="redeemResult.licenseKey" readonly>
              <template #append>
                <el-button @click="copyRedeemedKey">复制密钥</el-button>
              </template>
            </el-input>
          </div>
          <el-alert
            v-else
            title="授权尚未绑定目标，请在列表中点击“绑定目标”后使用。"
            type="warning"
            show-icon
            :closable="false"
          />
        </template>
      </el-result>
      <template #footer>
        <el-button type="primary" @click="closeRedeemResult">完成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, onMounted, computed } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import axios from 'axios'

  const loading = ref(false)
  const searchForm = reactive({ keyword: '', appId: '', status: '' })
  const pagination = reactive({ page: 1, pageSize: 10, total: 0 })
  const appList = ref<{ id: number; name: string }[]>([])
  const tableData = ref<any[]>([])
  const editDialog = reactive({
    visible: false,
    submitting: false,
    refreshing: false,
    id: 0,
    type: '',
    typeLabel: '',
    target: ''
  })

  const siteDialog = reactive({
    visible: false,
    loading: false,
    licenseId: 0,
    licenseNo: '',
    maxSites: 0,
    list: [] as any[]
  })

  const redeemDialog = reactive({
    visible: false,
    submitting: false,
    cardCode: ''
  })
  const redeemResult = reactive({
    visible: false,
    licenseNo: '',
    appName: '',
    planName: '',
    type: '',
    typeLabel: '',
    licenseKey: '',
    expireAt: '',
    idempotent: false
  })

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
  const typeTagMap: Record<string, TagType | undefined> = {
    domain: undefined,
    wildcard: 'success',
    ip: 'warning',
    key: 'info'
  }
  const statusTagMap: Record<string, TagType | undefined> = {
    active: 'success',
    expiring: 'warning',
    expired: 'info'
  }
  const editableTargetTypes = new Set(['domain', 'wildcard', 'ip', 'key'])
  const targetLabel = computed(() => {
    if (editDialog.type === 'ip') return 'IP'
    if (editDialog.type === 'key') return '授权密钥'
    if (editDialog.type === 'wildcard') return '泛域名'
    return '域名'
  })
  const targetPlaceholder = computed(() => `请输入${targetLabel.value}`)

  function getToken() {
    const stored = localStorage.getItem('user_panel_token')
    return stored || ''
  }

  async function fetchApps() {
    try {
      const { data } = await axios.get('/api/user-panel/apps', {
        headers: { Authorization: `Bearer ${getToken()}` }
      })
      if (data.code === 200) appList.value = data.data || []
    } catch {
      ElMessage.error('加载应用列表失败')
    }
  }

  async function fetchList() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/user-panel/licenses', {
        headers: { Authorization: `Bearer ${getToken()}` },
        params: {
          keyword: searchForm.keyword || undefined,
          appId: searchForm.appId || undefined,
          status: searchForm.status || undefined,
          page: pagination.page,
          pageSize: pagination.pageSize
        }
      })
      if (data.code === 200) {
        tableData.value = data.data.list || []
        pagination.total = data.data.total || 0
      }
    } catch {
      ElMessage.error('加载授权列表失败')
    } finally {
      loading.value = false
    }
  }

  function handleSearch() {
    pagination.page = 1
    fetchList()
  }
  function handleReset() {
    searchForm.keyword = ''
    searchForm.appId = ''
    searchForm.status = ''
    handleSearch()
  }
  function canEditTargetType(type: string) {
    return editableTargetTypes.has(type)
  }
  function formatLicenseAmount(amount: unknown) {
    if (amount === null || amount === undefined || amount === '') return '--'
    return `¥${Number(amount).toFixed(2)}`
  }

  function openEditDialog(row: any) {
    editDialog.id = row.id
    editDialog.type = row.type
    editDialog.typeLabel = row.typeLabel
    editDialog.target = row.domain || ''
    editDialog.visible = true
  }

  async function submitEditTarget() {
    const target = editDialog.target.trim()
    if (!target) {
      ElMessage.warning(`请输入${targetLabel.value}`)
      return
    }

    editDialog.submitting = true
    try {
      const { data } = await axios.put(
        `/api/user-panel/licenses/${editDialog.id}/target`,
        { target },
        {
          headers: { Authorization: `Bearer ${getToken()}` }
        }
      )
      if (data.code === 200) {
        ElMessage.success(data.msg || '更新成功')
        editDialog.visible = false
        fetchList()
      } else {
        ElMessage.error(data.msg || '更新失败')
      }
    } catch {
      ElMessage.error('更新失败')
    } finally {
      editDialog.submitting = false
    }
  }

  async function resetLicenseKey() {
    editDialog.refreshing = true
    try {
      const { data } = await axios.post(
        `/api/user-panel/licenses/${editDialog.id}/refresh-key`,
        {},
        {
          headers: { Authorization: `Bearer ${getToken()}` }
        }
      )
      if (data.code === 200) {
        editDialog.target = data.data?.licenseKey || ''
        ElMessage.success(data.msg || '密钥已重置')
        fetchList()
      } else {
        ElMessage.error(data.msg || '重置失败')
      }
    } catch {
      ElMessage.error('重置失败')
    } finally {
      editDialog.refreshing = false
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
      const { data } = await axios.get(`/api/user-panel/licenses/${siteDialog.licenseId}/sites`, {
        headers: { Authorization: `Bearer ${getToken()}` }
      })
      if (data.code === 200) {
        siteDialog.list = data.data?.list || []
        if (data.data?.maxSites !== undefined) siteDialog.maxSites = Number(data.data.maxSites)
      } else {
        ElMessage.error(data.msg || '加载绑定站点失败')
      }
    } catch {
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
      const { data } = await axios.delete(
        `/api/user-panel/licenses/${siteDialog.licenseId}/sites/${row.id}`,
        {
          headers: { Authorization: `Bearer ${getToken()}` }
        }
      )
      if (data.code === 200) {
        ElMessage.success(data.msg || '解绑成功')
        await fetchLicenseSites()
        fetchList()
      } else {
        ElMessage.error(data.msg || '解绑失败')
      }
    } catch (error) {
      if (error !== 'cancel' && error !== 'close') {
        ElMessage.error('解绑失败，请稍后重试')
      }
    }
  }

  function openRedeemDialog() {
    redeemDialog.cardCode = ''
    redeemDialog.submitting = false
    redeemDialog.visible = true
  }

  async function submitRedeem() {
    if (redeemDialog.submitting) return
    const cardCode = redeemDialog.cardCode.trim()
    if (!cardCode) {
      ElMessage.warning('请输入卡密')
      return
    }

    redeemDialog.submitting = true
    try {
      const { data } = await axios.post(
        '/api/user-panel/cards/redeem',
        { cardCode },
        {
          headers: { Authorization: `Bearer ${getToken()}` }
        }
      )
      if (data.code !== 200) {
        ElMessage.error(data.msg || '兑换失败')
        return
      }
      redeemDialog.visible = false
      Object.assign(redeemResult, {
        ...data.data,
        visible: true,
        licenseKey: data.data?.licenseKey || '',
        idempotent: data.data?.idempotent === true
      })
      await fetchList()
    } catch {
      ElMessage.error('兑换失败，请稍后重试')
    } finally {
      redeemDialog.submitting = false
    }
  }

  async function copyRedeemedKey() {
    if (!redeemResult.licenseKey) return
    try {
      await navigator.clipboard.writeText(redeemResult.licenseKey)
      ElMessage.success('密钥已复制')
    } catch {
      ElMessage.error('复制失败，请手动复制')
    }
  }

  function closeRedeemResult() {
    redeemResult.visible = false
  }

  onMounted(() => {
    fetchApps()
    fetchList()
  })
</script>

<style scoped lang="scss">
  .mb-4 {
    margin-bottom: 16px;
  }

  .mb-3 {
    margin-bottom: 12px;
  }

  .text-secondary {
    color: var(--el-text-color-secondary);
  }

  .card-header {
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

  .source-text {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .target-editor {
    display: flex;
    gap: 8px;
    align-items: center;
    width: 100%;
  }

  .target-editor .el-button {
    flex: 0 0 auto;
  }

  .target-editor .el-button :deep(.el-icon) {
    margin-right: 4px;
  }

  .redeem-alert {
    margin-bottom: 18px;
  }

  .redeem-summary {
    display: grid;
    gap: 6px;
    color: var(--el-text-color-regular);
  }

  .license-key-result {
    width: 100%;
    min-width: 360px;
  }
</style>
