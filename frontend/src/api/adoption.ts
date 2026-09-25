import { get, post } from '@/utils/request'
import type { Plot } from './plot'
import type { UserInfo } from './auth'

export interface AdoptionApplication {
  id: number
  plot_id: number
  plot: Plot | null
  user_id: number
  user: UserInfo | null
  message: string
  status: string
  reviewer_id: number | null
  reviewer: UserInfo | null
  review_note: string
  reviewed_at: string
  created_at: string
}

export function listApplications(params?: Record<string, any>): Promise<{ list: AdoptionApplication[]; total: number; page: number; page_size: number }> {
  return get('/applications', { params })
}

export function submitApplication(payload: { plot_id: number; message: string }): Promise<AdoptionApplication> {
  return post('/applications', payload)
}

export function withdrawApplication(id: number): Promise<AdoptionApplication> {
  return post(`/applications/${id}/withdraw`)
}

export function approveApplication(id: number, note?: string): Promise<AdoptionApplication> {
  return post(`/applications/${id}/approve`, { note })
}

export function rejectApplication(id: number, note?: string): Promise<AdoptionApplication> {
  return post(`/applications/${id}/reject`, { note })
}
