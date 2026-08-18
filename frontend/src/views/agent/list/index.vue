<template>
  <div class="agent-list">
    <!-- 搜索栏 -->
    <el-card shadow="hover" class="mb-4">
      <el-form :model="searchForm" inline>
        <el-form-item label="代理商账号">
          <el-input v-model="searchForm.keyword" placeholder="账号/手机/邮箱" clearable style="width: 180px" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="searchForm.level" placeholder="全部" clearable style="width: 120px">
            <el-option v-for="level in levelOptions" :key="level.code" :label="level.name" :value="level.code" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 110px">
            <el-option label="正常" value="active" />
            <el-option label="冻结" value="frozen" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">查询</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 表格 -->
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">代理商列表</span>
          <el-button type="primary" @click="handleAdd">新增代理商</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe v-loading="loading">
        <el-table-column prop="name" label="账号" min-width="120" />
        <el-table-column prop="contact" label="联系方式" min-width="140" />
        <el-table-column prop="levelLabel" label="等级" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="levelTagType(row.discount)" size="small">{{ row.levelLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="discount" label="折扣" width="80" align="center">
          <template #default="{ row }">
            <span>{{ row.discount }}折</span>
          </template>
        </el-table-column>
        <el-table-column prop="balance" label="余额(元)" width="110" align="right">
          <template #default="{ row }">
            <span class="balance-text">¥{{ row.balance.toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="totalLicenses" label="已开授权" width="100" align="center" />
        <el-table-column prop="statusLabel" label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ row.statusLabel }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="注册时间" width="160" />
        <el-table-column label="操作" width="290" fixed="right">
          <template #default="{ row }">
            <el-button link type="success" size="small" @click="loginAsAgent(row)">登录</el-button>
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" size="small" @click="handleRecharge(row)">充值</el-button>
            <el-button link type="primary" size="small" @click="handleToggle(row)">
              {{ row.status === 'active' ? '冻结' : '解冻' }}
            </el-button>
            <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
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
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="520px" destroy-on-close>
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="90px">
        <el-form-item label="账号" prop="name">
          <el-input v-model="formData.name" placeholder="代理商账号" />
        </el-form-item>
        <el-form-item label="联系方式" prop="contact">
          <el-input v-model="formData.contact" placeholder="手机号或邮箱" />
        </el-form-item>
        <el-form-item label="登录密码" prop="password">
          <el-input
            v-model="formData.password"
            type="password"
            :placeholder="isEdit ? '留空则不修改密码' : '代理商后台登录密码'"
            show-password
          />
        </el-form-item>
        <el-form-item label="等级" prop="level">
          <el-select v-model="formData.level" style="width: 100%" @change="syncDiscountByLevel">
            <el-option
              v-for="level in levelOptions"
              :key="level.code"
              :label="`${level.name} (${level.discount}折)`"
              :value="level.code"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="代理商折扣">
          <el-input-number v-model="formData.discount" :min="1" :max="10" :step="0.1" :precision="1" />
          <span class="ml-2 text-gray-400">默认使用等级折扣，可单独调整</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>

    <!-- 充值弹窗 -->
    <el-dialog v-model="rechargeVisible" title="代理商充值" width="400px" destroy-on-close>
      <el-form :model="rechargeForm" label-width="80px">
        <el-form-item label="代理商">
          <el-input :model-value="rechargeForm.name" disabled />
        </el-form-item>
        <el-form-item label="当前余额">
          <el-input :model-value="'¥' + rechargeForm.currentBalance.toFixed(2)" disabled />
        </el-form-item>
        <el-form-item label="充值金额">
          <el-input-number v-model="rechargeForm.amount" :min="1" :max="999999" :step="100" style="width: 100%" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="rechargeForm.remark" placeholder="充值备注（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRechargeSubmit">确认充值</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/http'

const loading = ref(false)
const dialogVisible = ref(false)
const rechargeVisible = ref(false)
const isEdit = ref(false)
const dialogTitle = computed(() => (isEdit.value ? '编辑代理商' : '新增代理商'))

const searchForm = reactive({ keyword: '', level: '', status: '' })
const pagination = reactive({ page: 1, pageSize: 10, total: 0 })

type AgentLevelOption = { code: string; name: string; discount: number }

const levelOptions = ref<AgentLevelOption[]>([])

const tableData = ref<any[]>([])

const formRef = ref()
const formData = reactive({ id: 0, name: '', contact: '', password: '', level: 'bronze', discount: 9, remark: '' })
const formRules = {
  name: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  contact: [{ required: true, message: '请输入联系方式', trigger: 'blur' }],
  password: [
    {
      validator: (_rule: any, value: string, callback: (error?: Error) => void) => {
        if (!isEdit.value && !value?.trim()) {
          callback(new Error('请输入密码'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  level: [{ required: true, message: '请选择等级', trigger: 'change' }]
}

const rechargeForm = reactive({ id: 0, name: '', currentBalance: 0, amount: 100, remark: '' })

async function fetchLevelOptions() {
  const data = await request.get<AgentLevelOption[]>({ url: '/api/agent-level/select-list' })
  levelOptions.value = data || []
}

async function handleSearch() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.page, pageSize: pagination.pageSize }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.level) params.level = searchForm.level
    if (searchForm.status) params.status = searchForm.status

    const data = await request.get<{ list: any[]; total: number }>({ url: '/api/agent/list', params })
    tableData.value = data.list || []
    pagination.total = data.total || 0
  } catch (e) {
    console.error('[AgentList] 查询失败:', e)
  } finally {
    loading.value = false
  }
}

function handleReset() {
  searchForm.keyword = ''
  searchForm.level = ''
  searchForm.status = ''
  pagination.page = 1
  handleSearch()
}

/**
 * 管理员代登录代理商账号（新窗口打开代理端，不影响当前管理员登录态）
 */
async function loginAsAgent(row: any) {
  if (!row?.id) {
    ElMessage.error('缺少代理商ID')
    return
  }
  try {
    const data = await request.post<any>({ url: `/api/agent/${row.id}/impersonate` })
    const info = {
      agentId: data.agentId,
      email: data.email,
      name: data.name,
      balance: data.balance
    }
    sessionStorage.setItem('impersonate_agent_token', data.accessToken)
    sessionStorage.setItem('impersonate_agent_info', JSON.stringify(info))
    window.open(`${location.origin}/agent-panel/login?impersonate=1`, '_blank')
  } catch {
    ElMessage.error('登录失败')
  }
}

function levelTagType(discount: number) {
  if (discount <= 7) return 'warning'
  if (discount <= 8) return 'success'
  return 'info'
}

function syncDiscountByLevel(level = formData.level) {
  const option = levelOptions.value.find(item => item.code === level)
  formData.discount = Number(option?.discount || 9)
}

function getDefaultLevel() {
  return levelOptions.value[0]?.code || 'bronze'
}

function handleAdd() {
  isEdit.value = false
  const defaultLevel = getDefaultLevel()
  Object.assign(formData, { id: 0, name: '', contact: '', password: '', level: defaultLevel, discount: 9, remark: '' })
  syncDiscountByLevel(defaultLevel)
  dialogVisible.value = true
}

function handleEdit(row: any) {
  isEdit.value = true
  Object.assign(formData, { id: row.id, name: row.name, contact: row.contact, password: '', level: row.level, discount: row.discount, remark: row.remark })
  dialogVisible.value = true
}

function handleRecharge(row: any) {
  Object.assign(rechargeForm, { id: row.id, name: row.name, currentBalance: row.balance, amount: 100, remark: '' })
  rechargeVisible.value = true
}

async function handleRechargeSubmit() {
  try {
    await request.post({
      url: `/api/agent/${rechargeForm.id}/recharge`,
      params: { amount: rechargeForm.amount, remark: rechargeForm.remark }
    })
    ElMessage.success(`成功充值 ¥${rechargeForm.amount}`)
    rechargeVisible.value = false
    handleSearch()
  } catch (e) {
    console.error('[AgentList] 充值失败:', e)
  }
}

async function handleToggle(row: any) {
  const newStatus = row.status === 'active' ? 'frozen' : 'active'
  const action = row.status === 'active' ? '冻结' : '解冻'
  try {
    await ElMessageBox.confirm(`确定${action}代理商「${row.name}」？`, '提示', { type: 'warning' })
    await request.put({ url: `/api/agent/${row.id}/toggle`, params: { status: newStatus } })
    ElMessage.success(`${action}成功`)
    handleSearch()
  } catch {}
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`删除代理商「${row.name}」将清除其所有数据，确定？`, '危险操作', { type: 'error' })
    await request.del({ url: `/api/agent/${row.id}` })
    ElMessage.success('删除成功')
    handleSearch()
  } catch {}
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  try {
    if (isEdit.value) {
      await request.put({
        url: `/api/agent/${formData.id}`,
        params: {
          name: formData.name,
          contact: formData.contact,
          password: formData.password,
          level: formData.level,
          discount: formData.discount,
          remark: formData.remark
        }
      })
      ElMessage.success('编辑成功')
    } else {
      await request.post({
        url: '/api/agent/create',
        params: {
          name: formData.name,
          contact: formData.contact,
          password: formData.password,
          level: formData.level,
          discount: formData.discount,
          remark: formData.remark
        }
      })
      ElMessage.success('新增成功')
    }
    dialogVisible.value = false
    handleSearch()
  } catch (e) {
    console.error('[AgentList] 提交失败:', e)
  }
}

onMounted(async () => {
  await fetchLevelOptions()
  handleSearch()
})
</script>

<style scoped lang="scss">
.mb-4 { margin-bottom: 16px; }
.ml-2 { margin-left: 8px; }
.table-header { display: flex; align-items: center; justify-content: space-between; }
.card-title { font-weight: 600; }
.pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 16px; }
.balance-text { font-weight: 600; color: var(--el-color-primary); }
</style>
