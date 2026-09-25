import { defineStore } from 'pinia'
import {
  applyAdoption,
  listAdoptions,
  listMyAdoptions,
  reviewAdoption,
  withdrawAdoption,
  type AdoptionApplication
} from '@/api/adoption'

interface AdoptionState {
  mine: AdoptionApplication[]
  mineTotal: number
  queue: AdoptionApplication[]
  queueTotal: number
  loading: boolean
}

export const useAdoptionStore = defineStore('adoption', {
  state: (): AdoptionState => ({ mine: [], mineTotal: 0, queue: [], queueTotal: 0, loading: false }),
  getters: {
    // 我对待审地块的集合（地块认养页按钮显隐复用）
    pendingPlotIds: (state): Set<number> => new Set(state.mine.filter((a) => a.status === 'pending').map((a) => a.plot_id))
  },
  actions: {
    async fetchMine(params?: Record<string, any>) {
      this.loading = true
      try {
        const data = await listMyAdoptions(params)
        this.mine = data.list
        this.mineTotal = data.total
      } finally {
        this.loading = false
      }
    },
    async fetchQueue(params?: Record<string, any>) {
      this.loading = true
      try {
        const data = await listAdoptions(params)
        this.queue = data.list
        this.queueTotal = data.total
      } finally {
        this.loading = false
      }
    },
    async apply(plotId: number, message: string) {
      await applyAdoption({ plot_id: plotId, message })
      await this.fetchMine()
    },
    async withdraw(id: number) {
      await withdrawAdoption(id)
      await this.fetchMine()
    },
    async review(id: number, action: 'approve' | 'reject', reviewNote?: string) {
      await reviewAdoption(id, { action, review_note: reviewNote })
    }
  }
})
