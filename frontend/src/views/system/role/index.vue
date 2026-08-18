<!-- 角色管理页面 -->
<template>
  <div class="art-full-height">
    <RoleSearch
      v-show="showSearchBar"
      v-model="searchForm"
      @search="handleSearch"
      @reset="resetSearchParams"
    ></RoleSearch>

    <ElCard class="art-table-card" :style="{ 'margin-top': showSearchBar ? '12px' : '0' }">
      <ArtTableHeader
        v-model:columns="columnChecks"
        v-model:showSearchBar="showSearchBar"
        :loading="loading"
        @refresh="refreshData"
      >
        <template #left>
          <ElSpace wrap>
            <ElButton @click="showDialog('add')" v-ripple>新增角色</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <!-- 表格 -->
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      >
      </ArtTable>
    </ElCard>

    <!-- 角色编辑弹窗 -->
    <RoleEditDialog
      v-model="dialogVisible"
      :dialog-type="dialogType"
      :role-data="currentRoleData"
      @success="refreshData"
    />

    <!-- 应用权限查看弹窗 -->
    <ElDialog
      v-model="permissionViewVisible"
      title="应用权限详情"
      width="480px"
      align-center
      class="el-dialog-border"
    >
      <div class="permission-view" v-if="viewRoleData">
        <div class="pv-apps">
          <div class="pv-app-item" v-for="app in mockRoleApps" :key="app.id">
            <div class="pv-app-icon">
              <i :class="app.icon" />
            </div>
            <span class="pv-app-name">{{ app.name }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <ElButton @click="permissionViewVisible = false">关闭</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { fetchGetRoleList, fetchDeleteRole } from '@/api/system-manage'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import RoleSearch from './modules/role-search.vue'
  import RoleEditDialog from './modules/role-edit-dialog.vue'
  import { ElTag, ElButton, ElMessageBox } from 'element-plus'

  defineOptions({ name: 'Role' })

  type RoleListItem = Api.SystemManage.RoleListItem
  type RoleSearchFormParams = Api.SystemManage.RoleSearchParams & {
    daterange?: string[]
  }

  // 搜索表单
  const searchForm = ref<RoleSearchFormParams>({
    roleName: undefined,
    roleCode: undefined,
    description: undefined,
    enabled: undefined,
    daterange: undefined
  })

  const showSearchBar = ref(false)

  const dialogVisible = ref(false)
  const currentRoleData = ref<RoleListItem | undefined>(undefined)

  const {
    columns,
    columnChecks,
    data,
    loading,
    pagination,
    getData,
    replaceSearchParams,
    resetSearchParams,
    handleSizeChange,
    handleCurrentChange,
    refreshData
  } = useTable({
    // 核心配置
    core: {
      apiFn: fetchGetRoleList,
      apiParams: {
        current: 1,
        size: 20
      },
      // 排除 apiParams 中的属性
      excludeParams: ['daterange'],
      columnsFactory: () => [
        {
          prop: 'roleId',
          label: '角色ID',
          width: 100
        },
        {
          prop: 'roleName',
          label: '角色名称',
          minWidth: 120
        },
        {
          prop: 'roleCode',
          label: '角色编码',
          minWidth: 120
        },
        {
          prop: 'description',
          label: '角色描述',
          minWidth: 150,
          showOverflowTooltip: true
        },
        {
          prop: 'discount',
          label: '折扣',
          width: 100,
          formatter: (row) => {
            const d = row.discount
            const color = d <= 7 ? '#67c23a' : d <= 8 ? '#e6a23c' : ''
            const text = d >= 10 ? '无折扣' : `${d}折`
            return h(ElTag, { type: color === '#67c23a' ? 'success' : color === '#e6a23c' ? 'warning' : 'info', size: 'small' }, () => text)
          }
        },
        {
          prop: 'enabled',
          label: '角色状态',
          width: 100,
          formatter: (row) => {
            const statusConfig = row.enabled
              ? { type: 'success', text: '启用' }
              : { type: 'warning', text: '禁用' }
            return h(
              ElTag,
              { type: statusConfig.type as 'success' | 'warning' },
              () => statusConfig.text
            )
          }
        },
        {
          prop: 'appPermissions',
          label: '应用权限',
          minWidth: 120,
          formatter: (row) =>
            h(
              ElButton,
              {
                link: true,
                type: 'primary',
                size: 'small',
                onClick: () => showPermissionView(row)
              },
              () => '查看权限'
            )
        },
        {
          prop: 'createTime',
          label: '创建日期',
          width: 180,
          sortable: true
        },
        {
          prop: 'operation',
          label: '操作',
          width: 80,
          fixed: 'right',
          formatter: (row) =>
            h('div', [
              h(ArtButtonMore, {
                list: [
                  {
                    key: 'edit',
                    label: '编辑角色',
                    icon: 'ri:edit-2-line'
                  },
                  {
                    key: 'delete',
                    label: '删除角色',
                    icon: 'ri:delete-bin-4-line',
                    color: '#f56c6c'
                  }
                ],
                onClick: (item: ButtonMoreItem) => buttonMoreClick(item, row)
              })
            ])
        }
      ]
    }
  })

  const dialogType = ref<'add' | 'edit'>('add')

  const showDialog = (type: 'add' | 'edit', row?: RoleListItem) => {
    dialogVisible.value = true
    dialogType.value = type
    currentRoleData.value = row
  }

  /**
   * 搜索处理
   * @param params 搜索参数
   */
  const handleSearch = (params: RoleSearchFormParams) => {
    // 处理日期区间参数，把 daterange 转换为 startTime 和 endTime
    const { daterange, ...filtersParams } = params
    const [startTime, endTime] = Array.isArray(daterange) ? daterange : [null, null]

    replaceSearchParams({ ...filtersParams, startTime, endTime })
    getData()
  }

  const buttonMoreClick = (item: ButtonMoreItem, row: RoleListItem) => {
    switch (item.key) {
      case 'edit':
        showDialog('edit', row)
        break
      case 'delete':
        deleteRole(row)
        break
    }
  }

  const permissionViewVisible = ref(false)
  const viewRoleData = ref<RoleListItem | undefined>(undefined)

  const showPermissionView = (row: RoleListItem) => {
    viewRoleData.value = row
    permissionViewVisible.value = true
  }

  const deleteRole = (row: RoleListItem) => {
    ElMessageBox.confirm(`确定删除角色"${row.roleName}"吗？此操作不可恢复！`, '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
      .then(async () => {
        await fetchDeleteRole(row.roleId)
        refreshData()
      })
      .catch(() => {
        ElMessage.info('已取消删除')
      })
  }

  const mockRoleApps = ref([
    { id: '1', name: 'CMS Pro', code: 'APP_CMS', icon: 'ri-pages-line' },
    { id: '2', name: 'Shop系统', code: 'APP_SHOP', icon: 'ri-shopping-bag-line' },
    { id: '3', name: 'ERP内部', code: 'APP_ERP', icon: 'ri-building-2-line' }
  ])
</script>

<style scoped lang="scss">
.permission-view {
  .pv-apps {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .pv-app-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    border-radius: 10px;
    background: var(--el-fill-color-light);
    border: 1px solid var(--el-border-color-extra-light);

    .pv-app-icon {
      width: 36px;
      height: 36px;
      border-radius: 8px;
      background: var(--el-color-primary-light-9);
      color: var(--el-color-primary);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 18px;
    }

    .pv-app-name {
      font-size: 14px;
      font-weight: 500;
    }
  }
}
</style>