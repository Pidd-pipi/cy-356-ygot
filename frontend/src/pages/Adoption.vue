<template>
  <div class="page-card">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h3 class="page-title">{{ isAdmin ? '认养申请审核（管理员）' : '我的认养申请' }}</h3>
      <el-select v-model="statusFilter" placeholder="全部状态" clearable style="width: 140px" @change="onFilter">
        <el-option v-for="(m, s) in ApplicationStatusMeta" :key="s" :label="m.label" :value="s" />
      </el-select>
    </div>

    <el-alert
      v-if="!isAdmin"
      type="info"
      :closable="false"
      show-icon
      style="margin-bottom: 12px"
      title="认养申请需管理员审核；待审核时可撤回。若被拒绝，可前往「地块认养」选择其他空闲地块重新申请。"
    />

    <DataTable :data="store.applications" :loading="store.loading" :total="store.total" :page-size="pagination.size.value" :current-page="pagination.page.value" @update:current-page="onPage">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="地块" min-width="140">
        <template #default="{ row }">{{ row.plot ? `${row.plot.name}（${row.plot.code}）` : `地块 #${row.plot_id}` }}</template>
      </el-table-column>
      <el-table-column v-if="isAdmin" label="申请人" width="130">
        <template #default="{ row }">{{ row.user?.nickname || row.user?.username || `用户 #${row.user_id}` }}</template>
      </el-table-column>
      <el-table-column prop="message" label="认养留言" min-width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><StatusBadge :value="row.status" :meta-map="ApplicationStatusMeta" /></template>
      </el-table-column>
      <el-table-column label="审核备注" min-width="160">
        <template #default="{ row }">
          <span v-if="row.review_note">{{ row.review_note }}</span>
          <span v-else>-</span>
          <span v-if="row.reviewer" class="reviewer">（{{ row.reviewer.nickname || row.reviewer.username }}）</span>
        </template>
      </el-table-column>
      <el-table-column label="申请时间" width="160">
        <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <template v-if="isAdmin && row.status === 'pending'">
            <el-button type="success" size="small" @click="approve(row)">批准</el-button>
            <el-button type="danger" size="small" @click="reject(row)">拒绝</el-button>
          </template>
          <el-button v-if="!isAdmin && row.status === 'pending'" type="warning" size="small" @click="withdraw(row)">撤回</el-button>
          <el-button v-if="!isAdmin && row.status === 'rejected'" type="primary" size="small" @click="goPlots">选其他地块</el-button>
        </template>
      </el-table-column>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAdoptionStore } from '@/stores/adoption'
import { type AdoptionApplication } from '@/api/adoption'
import { useAuth } from '@/hooks/useAuth'
import { usePagination } from '@/hooks/usePagination'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { ApplicationStatusMeta } from '@/constants'
import { formatDateTime } from '@/utils/format'

const store = useAdoptionStore()
const pagination = usePagination()
const { isAdmin } = useAuth()
const router = useRouter()

const statusFilter = ref('')

async function fetch() {
  await store.fetchApplications({
    page: pagination.page.value,
    page_size: pagination.size.value,
    status: statusFilter.value || undefined
  })
}

function onPage(page: number) {
  pagination.page.value = page
  fetch()
}

function onFilter() {
  pagination.page.value = 1
  fetch()
}

async function approve(row: AdoptionApplication) {
  let note = ''
  try {
    const { value } = await ElMessageBox.prompt(
      `批准 ${row.user?.nickname || row.user?.username} 对地块 ${row.plot?.name || row.plot_id} 的认养申请？同地块其余待审申请将自动拒绝。`,
      '批准认养申请',
      { confirmButtonText: '批准', cancelButtonText: '取消', inputPlaceholder: '审核备注（可选）', type: 'success' }
    )
    note = value || ''
  } catch {
    return
  }
  await store.approve(row.id, note)
  ElMessage.success('已批准，地块归该居民认养')
  await fetch()
}

async function reject(row: AdoptionApplication) {
  let note = ''
  try {
    const { value } = await ElMessageBox.prompt(
      `拒绝 ${row.user?.nickname || row.user?.username} 对地块 ${row.plot?.name || row.plot_id} 的认养申请？`,
      '拒绝认养申请',
      { confirmButtonText: '拒绝', cancelButtonText: '取消', inputPlaceholder: '拒绝原因（可选）', type: 'warning' }
    )
    note = value || ''
  } catch {
    return
  }
  await store.reject(row.id, note)
  ElMessage.success('已拒绝该申请')
  await fetch()
}

async function withdraw(row: AdoptionApplication) {
  try {
    await ElMessageBox.confirm(`确认撤回对地块 ${row.plot?.name || row.plot_id} 的认养申请吗？`, '撤回确认', { type: 'warning' })
  } catch {
    return
  }
  await store.withdraw(row.id)
  ElMessage.success('申请已撤回')
  await fetch()
}

function goPlots() {
  router.push('/plots')
}

onMounted(fetch)
</script>

<style scoped>
.reviewer { color: #909399; font-size: 12px; }
</style>
