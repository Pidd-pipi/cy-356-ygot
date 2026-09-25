<template>
  <div class="page-card">
    <h3 class="page-title">认养申请</h3>

    <el-card v-if="isAdmin" shadow="never" style="margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>审核队列（管理员）</span>
          <el-radio-group v-model="queueStatus" size="small" @change="onQueueFilter">
            <el-radio-button value="pending">待审核</el-radio-button>
            <el-radio-button value="approved">已批准</el-radio-button>
            <el-radio-button value="rejected">已拒绝</el-radio-button>
            <el-radio-button value="withdrawn">已撤回</el-radio-button>
            <el-radio-button value="">全部</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <DataTable :data="store.queue" :loading="store.loading" :total="store.queueTotal" :page-size="queuePagination.size.value" :current-page="queuePagination.page.value" @update:current-page="onQueuePage">
        <el-table-column label="申请人" width="130">
          <template #default="{ row }">{{ row.nickname || row.username }}</template>
        </el-table-column>
        <el-table-column label="地块" min-width="140">
          <template #default="{ row }">{{ row.plot_code }} {{ row.plot_name }}</template>
        </el-table-column>
        <el-table-column prop="message" label="申请留言" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><StatusBadge :value="row.status" :meta-map="AdoptionStatusMeta" /></template>
        </el-table-column>
        <el-table-column label="申请时间" width="160">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button type="success" size="small" @click="approve(row)">批准</el-button>
              <el-button type="danger" size="small" @click="reject(row)">拒绝</el-button>
            </template>
            <span v-else class="muted">{{ row.reviewer_name ? `已由 ${row.reviewer_name} 审核` : '已处理' }}</span>
          </template>
        </el-table-column>
      </DataTable>
    </el-card>

    <el-card shadow="never">
      <template #header>我的认养申请（审核结果实时可见，待审可撤回）</template>
      <DataTable :data="store.mine" :loading="store.loading" :total="store.mineTotal" :page-size="pagination.size.value" :current-page="pagination.page.value" @update:current-page="onPage">
        <el-table-column label="地块" min-width="150">
          <template #default="{ row }">{{ row.plot_code }} {{ row.plot_name }}</template>
        </el-table-column>
        <el-table-column prop="message" label="我的留言" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><StatusBadge :value="row.status" :meta-map="AdoptionStatusMeta" /></template>
        </el-table-column>
        <el-table-column label="审核备注" min-width="160">
          <template #default="{ row }">{{ row.review_note || '-' }}</template>
        </el-table-column>
        <el-table-column label="审核人" width="110">
          <template #default="{ row }">{{ row.reviewer_name || '-' }}</template>
        </el-table-column>
        <el-table-column label="申请时间" width="160">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending'" type="warning" size="small" @click="withdraw(row)">撤回</el-button>
            <el-button v-else-if="row.status === 'rejected'" type="primary" size="small" @click="goPlots">去选择其他地块</el-button>
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
      </DataTable>
    </el-card>
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
import { AdoptionStatusMeta } from '@/constants'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const store = useAdoptionStore()
const { isAdmin } = useAuth()
const pagination = usePagination()
const queuePagination = usePagination()
const queueStatus = ref('pending')

async function fetchMine() {
  await store.fetchMine({ page: pagination.page.value, page_size: pagination.size.value })
}

async function fetchQueue() {
  if (!isAdmin.value) return
  await store.fetchQueue({ page: queuePagination.page.value, page_size: queuePagination.size.value, status: queueStatus.value })
}

function onPage(page: number) {
  pagination.page.value = page
  fetchMine()
}

function onQueuePage(page: number) {
  queuePagination.page.value = page
  fetchQueue()
}

function onQueueFilter() {
  queuePagination.page.value = 1
  fetchQueue()
}

async function withdraw(row: AdoptionApplication) {
  try {
    await ElMessageBox.confirm(`确认撤回对地块 ${row.plot_name}（${row.plot_code}）的认养申请吗？`, '撤回确认', { type: 'warning' })
  } catch {
    return
  }
  await store.withdraw(row.id)
  ElMessage.success('认养申请已撤回')
}

async function approve(row: AdoptionApplication) {
  try {
    await ElMessageBox.confirm(
      `批准 ${row.nickname || row.username} 对地块 ${row.plot_code} 的认养申请后，地块将归其认养，同地块其余待审申请将自动拒绝。确认批准吗？`,
      '批准确认',
      { type: 'success', confirmButtonText: '批准' }
    )
  } catch {
    return
  }
  await store.review(row.id, 'approve')
  ElMessage.success('已批准，地块已归该居民认养')
  await Promise.all([fetchQueue(), fetchMine()])
}

async function reject(row: AdoptionApplication) {
  let note = ''
  try {
    const { value } = await ElMessageBox.prompt(`拒绝 ${row.nickname || row.username} 对地块 ${row.plot_code} 的认养申请，可填写拒绝原因：`, '拒绝确认', {
      confirmButtonText: '确认拒绝',
      cancelButtonText: '取消',
      inputPlaceholder: '拒绝原因（可选，申请人可见）',
      inputValidator: (v: string) => !v || v.length <= 500 || '备注不能超过 500 字'
    })
    note = value || ''
  } catch {
    return
  }
  await store.review(row.id, 'reject', note)
  ElMessage.success('已拒绝该认养申请')
  await fetchQueue()
}

function goPlots() {
  router.push('/plots')
}

onMounted(() => {
  fetchMine()
  fetchQueue()
})
</script>
