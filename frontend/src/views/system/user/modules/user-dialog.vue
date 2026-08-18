<template>
  <ElDialog
    v-model="dialogVisible"
    :title="dialogType === 'add' ? '添加用户' : '编辑用户'"
    width="30%"
    align-center
  >
    <ElForm ref="formRef" :model="formData" :rules="rules" label-width="80px">
      <ElFormItem label="邮箱" prop="email">
        <ElInput v-model="formData.email" placeholder="请输入邮箱" />
      </ElFormItem>
      <ElFormItem label="账号" prop="nickname">
        <ElInput v-model="formData.nickname" placeholder="请输入账号" />
      </ElFormItem>
      <ElFormItem v-if="dialogType === 'add'" label="密码" prop="password">
        <ElInput v-model="formData.password" type="password" show-password placeholder="请输入密码（至少6位）" />
      </ElFormItem>
      <ElFormItem v-else label="密码" prop="password">
        <ElInput v-model="formData.password" type="password" show-password placeholder="留空则不修改密码" />
      </ElFormItem>
      <ElFormItem label="余额" prop="balance">
        <ElInputNumber v-model="formData.balance" :min="0" :precision="2" :step="100" controls-position="right" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <div class="dialog-footer">
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import request from '@/utils/http'

  interface Props {
    visible: boolean
    type: string
    userData?: Record<string, any>
  }

  interface Emits {
    (e: 'update:visible', value: boolean): void
    (e: 'submit'): void
  }

  const props = defineProps<Props>()
  const emit = defineEmits<Emits>()

  const dialogVisible = computed({
    get: () => props.visible,
    set: (value) => emit('update:visible', value)
  })

  const dialogType = computed(() => props.type)

  const formRef = ref<FormInstance>()
  const submitting = ref(false)

  const formData = reactive({
    email: '',
    nickname: '',
    password: '',
    balance: 0
  })

  const rules = computed<FormRules>(() => {
    const base: FormRules = {
      email: [
        { required: true, message: '请输入邮箱', trigger: 'blur' },
        { type: 'email', message: '请输入有效的邮箱', trigger: 'blur' }
      ],
      nickname: [
        { required: true, message: '请输入账号', trigger: 'blur' }
      ]
    }
    if (dialogType.value === 'add') {
      base.password = [
        { required: true, message: '请输入密码', trigger: 'blur' },
        { min: 6, message: '密码至少6位', trigger: 'blur' }
      ]
    } else {
      base.password = [
        {
          validator: (_rule, value, callback) => {
            if (value && value.length < 6) {
              callback(new Error('密码至少6位'))
            } else {
              callback()
            }
          },
          trigger: 'blur'
        }
      ]
    }
    return base
  })

  const initFormData = () => {
    const isEdit = props.type === 'edit' && props.userData
    const row = props.userData

    Object.assign(formData, {
      email: isEdit && row ? row.userEmail || '' : '',
      nickname: isEdit && row ? row.userName || '' : '',
      password: '',
      balance: isEdit && row ? row.balance || 0 : 0
    })
  }

  watch(
    () => [props.visible, props.type, props.userData],
    ([visible]) => {
      if (visible) {
        initFormData()
        nextTick(() => {
          formRef.value?.clearValidate()
        })
      }
    },
    { immediate: true }
  )

  const handleSubmit = async () => {
    if (!formRef.value) return

    await formRef.value.validate(async (valid) => {
      if (!valid) return
      submitting.value = true
      try {
        if (dialogType.value === 'add') {
          await request.post({
            url: '/api/user/create',
            data: {
              email: formData.email,
              nickname: formData.nickname,
              password: formData.password
            }
          })
          ElMessage.success('添加成功')
          dialogVisible.value = false
          emit('submit')
        } else {
          const userId = props.userData?.userId
          const payload: any = {
            email: formData.email,
            nickname: formData.nickname,
            balance: formData.balance
          }
          if (formData.password) {
            payload.password = formData.password
          }
          await request.put({ url: `/api/user/${userId}`, data: payload })
          ElMessage.success('更新成功')
          dialogVisible.value = false
          emit('submit')
        }
      } catch {
      } finally {
        submitting.value = false
      }
    })
  }
</script>
