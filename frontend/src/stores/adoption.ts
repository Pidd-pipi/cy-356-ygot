import { defineStore } from 'pinia'
import {
  approveApplication,
  listApplications,
  rejectApplication,
  submitApplication,
  withdrawApplication,
  type AdoptionApplication
} from '@/api/adoption'

interface AdoptionState {
  applications: AdoptionApplication[]
  total: number
  loading: boolean
}

export const useAdoptionStore = defineStore('adoption', {
  state: (): AdoptionState => ({ applications: [], total: 0, loading: false }),
  actions: {
    async fetchApplications(params?: Record<string, any>) {
      this.loading = true
      try {
        const data = await listApplications(params)
        this.applications = data.list
        this.total = data.total
      } finally {
        this.loading = false
      }
    },
    async submit(plotId: number, message: string) {
      await submitApplication({ plot_id: plotId, message })
    },
    async withdraw(id: number) {
      await withdrawApplication(id)
    },
    async approve(id: number, note?: string) {
      await approveApplication(id, note)
    },
    async reject(id: number, note?: string) {
      await rejectApplication(id, note)
    }
  }
})
