import { get, post } from '@/utils/request'

export interface AdoptionApplication {
  id: number
  plot_id: number
  plot_code: string
  plot_name: string
  user_id: number
  username: string
  nickname: string
  message: string
  status: string
  review_note: string
  reviewed_by: number | null
  reviewer_name: string
  reviewed_at: string | null
  created_at: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

// 居民提交认养申请（附留言）
export function applyAdoption(payload: { plot_id: number; message: string }): Promise<AdoptionApplication> {
  return post('/adoption-applications', payload)
}

// 我的认养申请（审核结果可见）
export function listMyAdoptions(params?: Record<string, any>): Promise<PageResult<AdoptionApplication>> {
  return get('/adoption-applications/mine', { params })
}

// 撤回待审申请
export function withdrawAdoption(id: number): Promise<AdoptionApplication> {
  return post(`/adoption-applications/${id}/withdraw`)
}

// 管理员查看全部申请（可按状态/地块过滤）
export function listAdoptions(params?: Record<string, any>): Promise<PageResult<AdoptionApplication>> {
  return get('/adoption-applications', { params })
}

// 管理员审核（approve 后同地块其余待审申请自动拒绝）
export function reviewAdoption(id: number, payload: { action: 'approve' | 'reject'; review_note?: string }): Promise<AdoptionApplication> {
  return post(`/adoption-applications/${id}/review`, payload)
}
