<template>
  <ElDialog
    :title="dialogTitle"
    :model-value="visible"
    @update:model-value="handleCancel"
    width="860px"
    align-center
    class="menu-dialog"
    @closed="handleClosed"
  >
    <ArtForm
      ref="formRef"
      v-model="form"
      :items="formItems"
      :rules="rules"
      :span="width > 640 ? 12 : 24"
      :gutter="20"
      label-width="100px"
      :show-reset="false"
      :show-submit="false"
    >
      <template #menuType>
        <ElRadioGroup v-model="form.menuType" :disabled="disableMenuType">
          <ElRadioButton value="menu" label="menu">菜单</ElRadioButton>
          <ElRadioButton value="button" label="button">按钮</ElRadioButton>
        </ElRadioGroup>
      </template>
    </ArtForm>

    <template #footer>
      <span class="dialog-footer">
        <ElButton @click="handleCancel">取 消</ElButton>
        <ElButton type="primary" @click="handleSubmit">确 定</ElButton>
      </span>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormRules } from 'element-plus'
  import { ElIcon, ElTooltip } from 'element-plus'
  import { QuestionFilled } from '@element-plus/icons-vue'
  import { formatMenuTitle } from '@/utils/router'
  import type { AppRouteRecord } from '@/types/router'
  import type { FormItem } from '@/components/core/forms/art-form/index.vue'
  import ArtForm from '@/components/core/forms/art-form/index.vue'
  import { useWindowSize } from '@vueuse/core'

  const { width } = useWindowSize()

  const createLabelTooltip = (label: string, tooltip: string) => {
    return () =>
      h('span', { class: 'flex items-center' }, [
        h('span', label),
        h(
          ElTooltip,
          { content: tooltip, placement: 'top' },
          () => h(ElIcon, { class: 'ml-0.5 cursor-help' }, () => h(QuestionFilled))
        )
      ])
  }

  interface MenuFormData {
    id: number; name: string; path: string; label: string; component: string; icon: string
    isEnable: boolean; sort: number; isMenu: boolean; keepAlive: boolean; isHide: boolean
    isHideTab: boolean; link: string; isIframe: boolean; showBadge: boolean; showTextBadge: string
    fixedTab: boolean; activePath: string; roles: string[]; isFullPage: boolean
    authName: string; authLabel: string; authIcon: string; authSort: number
  }

  interface Props {
    visible: boolean
    editData?: AppRouteRecord | any
    type?: 'menu' | 'button'
    lockType?: boolean
  }

  interface Emits {
    (e: 'update:visible', value: boolean): void
    (e: 'submit', data: MenuFormData): void
  }

  const props = withDefaults(defineProps<Props>(), { visible: false, type: 'menu', lockType: false })
  const emit = defineEmits<Emits>()

  const formRef = ref()
  const isEdit = ref(false)

  const form = reactive<MenuFormData & { menuType: 'menu' | 'button' }>({
    menuType: 'menu', id: 0, name: '', path: '', label: '', component: '', icon: '',
    isEnable: true, sort: 1, isMenu: true, keepAlive: true, isHide: false, isHideTab: false,
    link: '', isIframe: false, showBadge: false, showTextBadge: '', fixedTab: false,
    activePath: '', roles: [], isFullPage: false, authName: '', authLabel: '', authIcon: '', authSort: 1
  })

  const rules = reactive<FormRules>({
    name: [{ required: true, message: '请输入菜单名称', trigger: 'blur' }, { min: 2, max: 20, message: '长度在 2 到 20 个字符', trigger: 'blur' }],
    path: [{ required: true, message: '请输入路由地址', trigger: 'blur' }],
    label: [{ required: true, message: '输入权限标识', trigger: 'blur' }],
    authName: [{ required: true, message: '请输入权限名称', trigger: 'blur' }],
    authLabel: [{ required: true, message: '请输入权限标识', trigger: 'blur' }]
  })

  const switchSpan = computed(() => width.value < 640 ? 12 : 6)

  const formItems = computed<FormItem[]>(() => {
    const baseItems: FormItem[] = [{ label: '菜单类型', key: 'menuType', span: 24 }]
    if (form.menuType === 'menu') {
      return [
        ...baseItems,
        { label: '菜单名称', key: 'name', type: 'input', props: { placeholder: '菜单名称' } },
        { label: createLabelTooltip('路由地址', '一级菜单：以 / 开头的绝对路径\n二级及以下：相对路径'), key: 'path', type: 'input', props: { placeholder: '如：/dashboard 或 console' } },
        { label: '权限标识', key: 'label', type: 'input', props: { placeholder: '如：User' } },
        { label: createLabelTooltip('组件路径', '一级父级菜单：填写 /index/index\n具体页面：填写组件路径\n目录菜单：留空'), key: 'component', type: 'input', props: { placeholder: '如：/system/user 或留空' } },
        { label: '图标', key: 'icon', type: 'input', props: { placeholder: '如：ri:user-line' } },
        { label: createLabelTooltip('角色权限', '仅用于前端权限模式'), key: 'roles', type: 'inputtag', props: { placeholder: '输入角色标识后按回车' } },
        { label: '菜单排序', key: 'sort', type: 'number', props: { min: 1, controlsPosition: 'right', style: { width: '100%' } } },
        { label: '外部链接', key: 'link', type: 'input', props: { placeholder: '如：https://www.example.com' } },
        { label: '文本徽章', key: 'showTextBadge', type: 'input', props: { placeholder: '如：New、Hot' } },
        { label: createLabelTooltip('激活路径', '用于详情页等隐藏菜单，指定高亮显示的父级菜单路径'), key: 'activePath', type: 'input', props: { placeholder: '如：/system/user' } },
        { label: '是否启用', key: 'isEnable', type: 'switch', span: switchSpan.value },
        { label: '页面缓存', key: 'keepAlive', type: 'switch', span: switchSpan.value },
        { label: '隐藏菜单', key: 'isHide', type: 'switch', span: switchSpan.value },
        { label: '是否内嵌', key: 'isIframe', type: 'switch', span: switchSpan.value },
        { label: '显示徽章', key: 'showBadge', type: 'switch', span: switchSpan.value },
        { label: '固定标签', key: 'fixedTab', type: 'switch', span: switchSpan.value },
        { label: '标签隐藏', key: 'isHideTab', type: 'switch', span: switchSpan.value },
        { label: '全屏页面', key: 'isFullPage', type: 'switch', span: switchSpan.value }
      ]
    } else {
      return [
        ...baseItems,
        { label: '权限名称', key: 'authName', type: 'input', props: { placeholder: '如：新增、编辑、删除' } },
        { label: '权限标识', key: 'authLabel', type: 'input', props: { placeholder: '如：add、edit、delete' } },
        { label: '权限排序', key: 'authSort', type: 'number', props: { min: 1, controlsPosition: 'right', style: { width: '100%' } } }
      ]
    }
  })

  const dialogTitle = computed(() => {
    const type = form.menuType === 'menu' ? '菜单' : '按钮'
    return isEdit.value ? `编辑${type}` : `新建${type}`
  })

  const disableMenuType = computed(() => {
    if (isEdit.value) return true
    if (!isEdit.value && form.menuType === 'menu' && props.lockType) return true
    return false
  })

  const resetForm = (): void => { formRef.value?.reset(); form.menuType = 'menu' }

  const loadFormData = (): void => {
    if (!props.editData) return
    isEdit.value = true
    if (form.menuType === 'menu') {
      const row = props.editData
      form.id = row.id || 0; form.name = formatMenuTitle(row.meta?.title || ''); form.path = row.path || ''
      form.label = row.name || ''; form.component = row.component || ''; form.icon = row.meta?.icon || ''
      form.sort = row.meta?.sort || 1; form.isMenu = row.meta?.isMenu ?? true
      form.keepAlive = row.meta?.keepAlive ?? false; form.isHide = row.meta?.isHide ?? false
      form.isHideTab = row.meta?.isHideTab ?? false; form.isEnable = row.meta?.isEnable ?? true
      form.link = row.meta?.link || ''; form.isIframe = row.meta?.isIframe ?? false
      form.showBadge = row.meta?.showBadge ?? false; form.showTextBadge = row.meta?.showTextBadge || ''
      form.fixedTab = row.meta?.fixedTab ?? false; form.activePath = row.meta?.activePath || ''
      form.roles = row.meta?.roles || []; form.isFullPage = row.meta?.isFullPage ?? false
    } else {
      const row = props.editData
      form.authName = row.title || ''; form.authLabel = row.authMark || ''
      form.authIcon = row.icon || ''; form.authSort = row.sort || 1
    }
  }

  const handleSubmit = async (): Promise<void> => {
    if (!formRef.value) return
    try {
      await formRef.value.validate()
      emit('submit', { ...form })
      ElMessage.success(`${isEdit.value ? '编辑' : '新增'}成功`)
      handleCancel()
    } catch {
      ElMessage.error('表单校验失败，请检查输入')
    }
  }

  const handleCancel = (): void => { emit('update:visible', false) }
  const handleClosed = (): void => { resetForm(); isEdit.value = false }

  watch(() => props.visible, (newVal) => {
    if (newVal) {
      form.menuType = props.type
      nextTick(() => { if (props.editData) loadFormData() })
    }
  })

  watch(() => props.type, (newType) => { if (props.visible) form.menuType = newType })
</script>