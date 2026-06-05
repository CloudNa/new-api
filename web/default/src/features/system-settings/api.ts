/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import type {
  ConfirmPaymentComplianceResponse,
  DeleteLogsResponse,
  FetchUpstreamRatiosRequest,
  SystemOptionsResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true }
  )
  return res.data
}

export async function deleteLogsBefore(targetTimestamp: number) {
  const res = await api.delete<DeleteLogsResponse>('/api/log/', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

export async function getSystemUpdateStatus() {
  const res = await api.get('/api/system_update/status', {
    disableDuplicate: true,
  })
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateStatus
  }
}

export async function precheckSystemUpdate() {
  const res = await api.get('/api/system_update/precheck', {
    disableDuplicate: true,
  })
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdatePrecheck
  }
}

export async function smokeSystemUpdate() {
  const res = await api.post(
    '/api/system_update/smoke',
    {},
    {
      disableDuplicate: true,
    }
  )
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateSmoke
  }
}

export async function startSystemUpdate() {
  const res = await api.post('/api/system_update/start', {})
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateStatus
  }
}

export type SystemUpdateStatus = {
  enabled: boolean
  running: boolean
  last_exit?: number | null
  started_at?: string
  finished_at?: string
  message?: string
  log_tail?: string[]
}

export type SystemUpdatePrecheckItem = {
  name: string
  ok: boolean
  message?: string
}

export type SystemUpdatePrecheck = {
  enabled: boolean
  ok: boolean
  message?: string
  checked_at?: string
  checks?: SystemUpdatePrecheckItem[]
}

export type SystemUpdateSmoke = {
  enabled: boolean
  ok: boolean
  message?: string
  checked_at?: string
  status?: number | null
  content_type?: string
  new_api_healthy: boolean
  gpt_load_healthy: boolean
  cliproxyapi_ready: boolean
  error?: string
}
