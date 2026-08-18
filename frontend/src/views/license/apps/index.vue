<template>
  <div class="license-apps">
    <el-card shadow="hover">
      <template #header>
        <div class="table-header">
          <span class="card-title">应用管理</span>
          <el-button type="primary" @click="handleAdd">新增应用</el-button>
        </div>
      </template>

      <el-table :data="tableData" stripe>
        <el-table-column prop="name" label="应用名称" min-width="150" />
        <el-table-column prop="appKey" label="AppKey" min-width="220" show-overflow-tooltip />
        <el-table-column label="授权方式" min-width="250">
          <template #default="{ row }">
            <div v-if="row.purchaseLicenseTypes?.length" class="license-type-tags">
              <el-tag
                v-for="licenseType in orderedPurchaseLicenseTypes(row.purchaseLicenseTypes)"
                :key="licenseType"
                :type="purchaseLicenseTypeMeta[licenseType]?.tagType"
                size="small"
                effect="plain"
              >
                {{ purchaseLicenseTypeMeta[licenseType]?.label || licenseType }}
              </el-tag>
            </div>
            <el-tag v-else type="info" size="small">已关闭购买</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="appSecret" label="AppSecret" min-width="220">
          <template #default="{ row }">
            <span v-if="!row.showSecret">••••••••••••••••</span>
            <span v-else>{{ row.appSecret }}</span>
            <el-button
              link
              type="primary"
              size="small"
              @click="row.showSecret = !row.showSecret"
              style="margin-left: 8px"
            >
              {{ row.showSecret ? '隐藏' : '查看' }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="licenseCount" label="授权数" width="90" align="center" />
        <el-table-column label="版本" min-width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleVersions(row)">
              {{ row.recentVersion || '未发布' }}
            </el-button>
            <span v-if="row.versionCount" class="version-count">{{ row.versionCount }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="授权校验" width="100" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.licenseRequired"
              :loading="row.licenseRequiredChanging"
              :before-change="() => handleLicenseRequiredChange(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="160" />
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleVersions(row)"
              >版本</el-button
            >
            <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" size="small" @click="handleResetSecret(row)"
              >重置密钥</el-button
            >
            <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px" destroy-on-close>
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="100px">
        <el-form-item label="应用名称" prop="name">
          <el-input v-model="formData.name" placeholder="请输入应用名称" />
        </el-form-item>
        <el-form-item label="授权方式">
          <el-checkbox-group v-model="formData.purchaseLicenseTypes" class="license-type-options">
            <el-checkbox
              v-for="licenseType in purchaseLicenseTypeOrder"
              :key="licenseType"
              :value="licenseType"
            >
              {{ purchaseLicenseTypeMeta[licenseType].label }}
            </el-checkbox>
          </el-checkbox-group>
          <div class="form-tip">全部取消后，用户端和代理端将不再显示该应用。</div>
        </el-form-item>
        <el-form-item label="回调地址">
          <el-input v-model="formData.callbackUrl" placeholder="授权验证回调URL（可选）" />
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
  import { ref, reactive, computed, onActivated } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import request from '@/utils/http'

  const dialogVisible = ref(false)
  const router = useRouter()
  const isEdit = ref(false)
  const dialogTitle = computed(() => (isEdit.value ? '编辑应用' : '新增应用'))

  const tableData = ref<any[]>([])
  const purchaseLicenseTypeOrder = ['domain', 'wildcard', 'ip', 'key'] as const
  const purchaseLicenseTypeMeta: Record<
    string,
    { label: string; tagType: 'primary' | 'success' | 'warning' | 'info' }
  > = {
    domain: { label: '单域名', tagType: 'primary' },
    wildcard: { label: '泛域名', tagType: 'success' },
    ip: { label: 'IP', tagType: 'warning' },
    key: { label: '密钥', tagType: 'info' }
  }

  const formRef = ref()
  const formData = reactive({
    id: 0,
    name: '',
    callbackUrl: '',
    enabled: true,
    remark: '',
    purchaseLicenseTypes: [...purchaseLicenseTypeOrder] as string[]
  })

  const formRules = {
    name: [{ required: true, message: '请输入应用名称', trigger: 'blur' }]
  }

  function orderedPurchaseLicenseTypes(types: string[] = []) {
    return purchaseLicenseTypeOrder.filter((licenseType) => types.includes(licenseType))
  }

  async function fetchList() {
    try {
      const data = await request.get<any[]>({ url: '/api/app/list' })
      tableData.value = (data || []).map((item: any) => ({
        ...item,
        licenseRequired: item.licenseRequired !== false,
        showSecret: false,
        licenseRequiredChanging: false
      }))
    } catch (e) {
      console.error('[AppManage] 加载失败:', e)
    }
  }

  function handleAdd() {
    isEdit.value = false
    formData.id = 0
    formData.name = ''
    formData.callbackUrl = ''
    formData.enabled = true
    formData.remark = ''
    formData.purchaseLicenseTypes = [...purchaseLicenseTypeOrder]
    dialogVisible.value = true
  }

  function handleEdit(row: any) {
    isEdit.value = true
    formData.id = row.id
    formData.name = row.name
    formData.callbackUrl = ''
    formData.enabled = row.enabled
    formData.remark = row.remark
    formData.purchaseLicenseTypes = Array.isArray(row.purchaseLicenseTypes)
      ? [...row.purchaseLicenseTypes]
      : [...purchaseLicenseTypeOrder]
    dialogVisible.value = true
  }

  async function handleLicenseRequiredChange(row: any) {
    const licenseRequired = !row.licenseRequired
    if (!licenseRequired) {
      try {
        await ElMessageBox.confirm(
          `关闭应用「${row.name}」的授权校验后，客户端无需许可证即可通过验证和版本检查。应用签名与启用状态校验仍然有效，是否继续？`,
          '关闭授权校验',
          {
            type: 'warning',
            confirmButtonText: '确认关闭',
            cancelButtonText: '取消'
          }
        )
      } catch {
        return false
      }
    }

    row.licenseRequiredChanging = true
    try {
      await request.put<{ licenseRequired: boolean }>({
        url: `/api/app/${row.id}/license-required`,
        params: { licenseRequired }
      })
      ElMessage.success(licenseRequired ? '已要求授权校验' : '已关闭授权校验')
      return true
    } catch (e) {
      console.error('[AppManage] 更新授权校验失败:', e)
      return false
    } finally {
      row.licenseRequiredChanging = false
    }
  }

  async function handleResetSecret(row: any) {
    try {
      await ElMessageBox.confirm(
        `确定重置应用「${row.name}」的AppSecret？旧密钥将立即失效`,
        '警告',
        { type: 'warning' }
      )
      const data = await request.put<{ appSecret: string }>({
        url: `/api/app/${row.id}/reset-secret`
      })
      row.appSecret = data.appSecret
      ElMessage.success('密钥已重置')
    } catch {
      // 用户取消操作时保留当前数据。
    }
  }

  function handleVersions(row: any) {
    router.push(`/license/apps/${row.id}/versions`)
  }

  async function handleDelete(row: any) {
    try {
      await ElMessageBox.confirm(
        `删除应用「${row.name}」将同时清除其所有授权记录，确定？`,
        '危险操作',
        { type: 'error' }
      )
      await request.del({ url: `/api/app/${row.id}` })
      ElMessage.success('删除成功')
      fetchList()
    } catch {
      // 用户取消操作时保留当前数据。
    }
  }

  async function handleSubmit() {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    try {
      if (isEdit.value) {
        await request.put({
          url: `/api/app/${formData.id}`,
          params: {
            name: formData.name,
            enabled: formData.enabled,
            remark: formData.remark,
            purchaseLicenseTypes: formData.purchaseLicenseTypes
          }
        })
        ElMessage.success('编辑成功')
      } else {
        await request.post({
          url: '/api/app/create',
          params: {
            name: formData.name,
            enabled: formData.enabled,
            remark: formData.remark,
            purchaseLicenseTypes: formData.purchaseLicenseTypes
          }
        })
        ElMessage.success('新增成功')
      }
      dialogVisible.value = false
      fetchList()
    } catch (e) {
      console.error('[AppManage] 提交失败:', e)
    }
  }

  onActivated(() => {
    fetchList()
  })
</script>

<style scoped lang="scss">
  .table-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .card-title {
    font-weight: 600;
  }

  .version-count {
    margin-left: 6px;
    font-size: 12px;
    color: var(--art-gray-600);
  }

  .license-type-tags,
  .license-type-options {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .license-type-options :deep(.el-checkbox) {
    margin-right: 12px;
  }

  .form-tip {
    width: 100%;
    margin-top: 4px;
    color: var(--el-text-color-secondary);
    font-size: 12px;
    line-height: 1.5;
  }
</style>
