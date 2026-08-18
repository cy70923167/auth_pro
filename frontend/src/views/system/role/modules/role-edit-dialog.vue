<template>
  <ElDialog
    v-model="visible"
    :title="dialogType === 'add' ? '新增角色' : '编辑角色'"
    width="560px"
    align-center
    @close="handleClose"
  >
    <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
      <ElFormItem label="角色名称" prop="roleName">
        <ElInput v-model="form.roleName" placeholder="请输入角色名称" />
      </ElFormItem>
      <ElFormItem label="应用权限" prop="appPermissions">
        <div class="app-permission-section">
          <ElCheckbox
            v-model="form.allApps"
            @change="handleAllAppsChange"
          >所有应用权限</ElCheckbox>
          <div class="app-list" v-if="!form.allApps">
            <ElCheckboxGroup v-model="form.appPermissions">
              <ElCheckbox
                v-for="app in appList"
                :key="app.id"
                :value="app.id"
                class="app-checkbox"
              >
                <div class="app-check-item">
                  <span class="app-check-name">{{ app.name }}</span>
                  <ElTag size="small" type="info">{{ app.code }}</ElTag>
                </div>
              </ElCheckbox>
            </ElCheckboxGroup>
          </div>
          <div class="all-apps-hint" v-else>
            <ElTag type="success" size="small">拥有所有应用的完整权限</ElTag>
          </div>
        </div>
      </ElFormItem>
      <ElFormItem label="描述" prop="description">
        <ElInput
          v-model="form.description"
          type="textarea"
          :rows="3"
          placeholder="请输入角色描述"
        />
      </ElFormItem>
      <ElFormItem label="折扣" prop="discount">
        <ElInputNumber v-model="form.discount" :min="1" :max="10" :step="0.5" :precision="1" />
        <span style="margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary)">1~10，10为无折扣</span>
      </ElFormItem>
      <ElFormItem label="启用">
        <ElSwitch v-model="form.enabled" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="handleClose">取消</ElButton>
      <ElButton type="primary" @click="handleSubmit">提交</ElButton>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'

  import { fetchCreateRole, fetchUpdateRole } from '@/api/system-manage'

  type RoleListItem = Api.SystemManage.RoleListItem

  interface Props {
    modelValue: boolean
    dialogType: 'add' | 'edit'
    roleData?: RoleListItem
  }

  interface Emits {
    (e: 'update:modelValue', value: boolean): void
    (e: 'success'): void
  }

  const props = withDefaults(defineProps<Props>(), {
    modelValue: false,
    dialogType: 'add',
    roleData: undefined
  })

  const emit = defineEmits<Emits>()

  const formRef = ref<FormInstance>()

  const visible = computed({
    get: () => props.modelValue,
    set: (value) => emit('update:modelValue', value)
  })

  const appList = ref([
    { id: 'cms_pro', name: 'CMS Pro', code: 'APP_CMS' },
    { id: 'shop', name: 'Shop系统', code: 'APP_SHOP' },
    { id: 'erp', name: 'ERP内部', code: 'APP_ERP' },
    { id: 'desktop', name: '桌面工具', code: 'APP_DESKTOP' }
  ])

  const rules = reactive<FormRules>({
    roleName: [
      { required: true, message: '请输入角色名称', trigger: 'blur' },
      { min: 2, max: 20, message: '长度在 2 到 20 个字符', trigger: 'blur' }
    ]
  })

  const form = reactive({
    roleId: 0,
    roleName: '',
    roleCode: '',
    description: '',
    discount: 10.0,
    createTime: '',
    enabled: true,
    allApps: false,
    appPermissions: [] as string[]
  })

  function generateCode(): string {
    const prefix = 'ROLE_'
    const random = Math.random().toString(36).substring(2, 8).toUpperCase()
    return prefix + random
  }

  function handleAllAppsChange(val: boolean | string | number) {
    if (val) {
      form.appPermissions = appList.value.map(a => a.id)
    } else {
      form.appPermissions = []
    }
  }

  watch(
    () => props.modelValue,
    (newVal) => {
      if (newVal) initForm()
    }
  )

  watch(
    () => props.roleData,
    (newData) => {
      if (newData && props.modelValue) initForm()
    },
    { deep: true }
  )

  const initForm = () => {
    if (props.dialogType === 'edit' && props.roleData) {
      Object.assign(form, {
        ...props.roleData,
        discount: props.roleData.discount || 10.0,
        allApps: false,
        appPermissions: []
      })
    } else {
      Object.assign(form, {
        roleId: 0,
        roleName: '',
        roleCode: generateCode(),
        description: '',
        discount: 10.0,
        createTime: '',
        enabled: true,
        allApps: false,
        appPermissions: []
      })
    }
  }

  const handleClose = () => {
    visible.value = false
    formRef.value?.resetFields()
  }

  const handleSubmit = async () => {
    if (!formRef.value) return

    try {
      await formRef.value.validate()
      if (props.dialogType === 'add') {
        await fetchCreateRole({
          roleName: form.roleName,
          roleCode: form.roleCode,
          description: form.description,
          discount: form.discount,
          enabled: form.enabled,
          appPermissions: form.appPermissions
        })
      } else {
        await fetchUpdateRole(form.roleId, {
          roleName: form.roleName,
          description: form.description,
          discount: form.discount,
          enabled: form.enabled
        })
      }
      emit('success')
      handleClose()
    } catch (error) {
      console.log('表单验证失败:', error)
    }
  }
</script>

<style scoped lang="scss">
  .app-permission-section {
    width: 100%;

    .app-list {
      margin-top: 12px;
      padding: 12px;
      border-radius: 8px;
      background: var(--el-fill-color-light);
      border: 1px solid var(--el-border-color-lighter);

      .app-checkbox {
        display: flex;
        margin-bottom: 8px;
        margin-right: 0;
        width: 100%;

        &:last-child { margin-bottom: 0; }
      }

      .app-check-item {
        display: flex;
        align-items: center;
        gap: 8px;

        .app-check-name {
          font-weight: 500;
        }
      }
    }

    .all-apps-hint {
      margin-top: 8px;
    }
  }
</style>