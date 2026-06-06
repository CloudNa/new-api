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

export async function startSystemUpdate(
  component: SystemUpdateComponent = 'all'
) {
  const res = await api.post('/api/system_update/start', { component })
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateStatus
  }
}

export async function rollbackSystemUpdate(
  request: SystemUpdateRollbackRequest
) {
  const res = await api.post('/api/system_update/rollback', request)
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateStatus
  }
}

export async function getSystemUpdateBackups(
  component: SystemRollbackComponent
) {
  const res = await api.get('/api/system_update/backups', {
    params: { component },
    disableDuplicate: true,
  })
  return res.data as {
    success: boolean
    message?: string
    data?: SystemUpdateBackups
  }
}

export async function getCompressionSettings() {
  const res = await api.get('/api/settings/compression')
  return res.data as {
    success: boolean
    message?: string
    data?: PromptCompressionSettings
  }
}

export async function updateCompressionSettings(
  settings: PromptCompressionSettings
) {
  const res = await api.put('/api/settings/compression', settings)
  return res.data as {
    success: boolean
    message?: string
    data?: PromptCompressionSettings
  }
}

export async function previewCompression(request: CompressionPreviewRequest) {
  const res = await api.post('/api/compression/preview', request)
  return res.data as {
    success: boolean
    message?: string
    data?: CompressionPreviewResponse
  }
}

export async function getRtkFilters() {
  const res = await api.get('/api/context/rtk/filters')
  return res.data as {
    success: boolean
    message?: string
    data?: RtkFiltersResponse
  }
}

export async function testRtkCompression(text: string) {
  const res = await api.post('/api/context/rtk/test', { text })
  return res.data as {
    success: boolean
    message?: string
    data?: CompressionPreviewResponse
  }
}

export async function getProfitSettings() {
  const res = await api.get('/api/profit/settings')
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitSettings
  }
}

export async function updateProfitSettings(settings: ProfitSettings) {
  const res = await api.put('/api/profit/settings', settings)
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitSettings
  }
}

export async function getProfitCostProfiles() {
  const res = await api.get('/api/profit/cost-profiles')
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitCostProfiles
  }
}

export async function updateProfitCostProfiles(profiles: ProfitCostProfiles) {
  const res = await api.put('/api/profit/cost-profiles', profiles)
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitCostProfiles
  }
}

export async function getProfitAnalytics(params?: ProfitAnalyticsParams) {
  const res = await api.get('/api/profit/analytics', {
    params,
    disableDuplicate: true,
  })
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitAnalytics
  }
}

export async function getProfitEvents(params?: ProfitAnalyticsParams) {
  const res = await api.get('/api/profit/events', {
    params,
    disableDuplicate: true,
  })
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitEventsPage
  }
}

export async function previewProfitRoute(request: ProfitRoutePreviewRequest) {
  const res = await api.post('/api/profit/route-preview', request)
  return res.data as {
    success: boolean
    message?: string
    data?: ProfitRoutePreviewResponse
  }
}

export type SystemUpdateComponent =
  | 'all'
  | 'new-api'
  | 'gpt-load'
  | 'cliproxyapi'

export type SystemRollbackComponent = Exclude<SystemUpdateComponent, 'all'>

export type SystemUpdateRollbackRequest = {
  component: SystemRollbackComponent
  backup_id?: string
  restore_runtime?: boolean
}

export type SystemUpdateStatus = {
  enabled: boolean
  running: boolean
  last_exit?: number | null
  started_at?: string
  finished_at?: string
  message?: string
  current_action?: string
  current_component?: SystemUpdateComponent
  current_backup_id?: string
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
  sidecar_bridge_sources_ok?: boolean
  proxy_test_chat_checked?: boolean
  proxy_test_chat_ok?: boolean
  proxy_test_chat_skipped?: boolean
  error?: string
}

export type SystemUpdateBackup = {
  id: string
  component: SystemRollbackComponent
  created_at?: string
  before_commit?: string
  service_image?: string
  rollback_tag?: string
  runtime_path?: string
}

export type SystemUpdateBackups = {
  enabled: boolean
  component: SystemRollbackComponent
  backups?: SystemUpdateBackup[]
  message?: string
}

export type CompressionMode =
  | 'off'
  | 'lite'
  | 'standard'
  | 'aggressive'
  | 'ultra'
  | 'rtk'
  | 'stacked'

export type CavemanIntensity = 'lite' | 'standard' | 'aggressive' | 'ultra'

export type PromptCompressionSettings = {
  enabled: boolean
  default_mode: CompressionMode
  auto_trigger_mode: CompressionMode
  auto_trigger_tokens: number
  min_tokens: number
  preserve_system_prompt: boolean
  allowed_groups: string[]
  group_modes: Record<string, CompressionMode>
  model_modes: Record<string, CompressionMode>
  channel_modes: Record<string, CompressionMode>
  global_kill_switch: boolean
  rtk: {
    max_lines: number
    max_chars: number
    deduplicate_threshold: number
    enabled_filters: string[]
    disabled_filters: string[]
  }
  caveman: {
    intensity: CavemanIntensity
    compress_roles: string[]
    min_message_length: number
  }
  attribution: string
}

export type CompressionStats = {
  original_tokens: number
  compressed_tokens: number
  savings_percent: number
  mode: CompressionMode
  techniques_used?: string[]
  rules_applied?: string[]
  preserved_block_count: number
  redacted_secret_count: number
  compression_saved_tokens: number
  duration_ms: number
  bypassed: boolean
  bypass_reason?: string
  omniroute_compatible_mode?: string
}

export type CompressionPreviewRequest = {
  mode?: CompressionMode
  text: string
}

export type CompressionPreviewResponse = {
  text: string
  compressed: boolean
  stats: CompressionStats
}

export type RtkFiltersResponse = {
  filters: Array<{
    id: string
    category: string
  }>
  attribution: string
}

export type ProfitMode = 'off' | 'observe'
export type ProfitRiskMode = 'off' | 'alert'

export type ProfitSettings = {
  version: number
  enabled: boolean
  observe_only: boolean
  observe_groups: string[]
  global_kill_switch: boolean
  cost_routing_mode: ProfitMode
  cache_mode: ProfitMode
  output_cap_mode: ProfitMode
  risk_enforcement: ProfitRiskMode
  settings_writable: boolean
  cost_profiles_used: boolean
}

export type ProfitCostProfile = {
  id: string
  name: string
  enabled: boolean
  priority: number
  provider: string
  channel_id: number
  channel_name: string
  model_name: string
  input_usd_per_million: number
  output_usd_per_million: number
  cache_read_usd_per_million: number
  cache_write_usd_per_million: number
  fixed_request_usd: number
  failure_penalty_usd: number
  latency_penalty_usd_per_second: number
  risk_penalty_usd: number
  notes: string
}

export type ProfitCostProfiles = {
  version: number
  items: ProfitCostProfile[]
}

export type ProfitAnalyticsParams = {
  group?: string
  model_name?: string
  username?: string
  channel?: number
  start_timestamp?: number
  end_timestamp?: number
  p?: number
  page_size?: number
}

export type ProfitAnalytics = {
  request_count: number
  scanned_events: number
  total_matching_logs: number
  is_partial: boolean
  scan_limit: number
  cost_known_count: number
  missing_cost_profile_count: number
  billable_prompt_tokens: number
  billable_completion_tokens: number
  upstream_actual_prompt_tokens: number
  upstream_actual_completion_tokens: number
  estimated_revenue_usd: number
  estimated_upstream_cost_usd?: number | null
  gross_margin_usd?: number | null
  gross_margin_pct?: number | null
  expected_cost_usd?: number | null
  expected_margin_usd?: number | null
  expected_margin_pct?: number | null
  compression_saved_tokens: number
  cache_saved_usd?: number | null
  retry_cost_usd?: number | null
}

export type ProfitEvent = {
  id: number
  created_at: number
  user_id: number
  username: string
  token_name: string
  model_name: string
  channel: number
  channel_name: string
  group: string
  request_id?: string
  upstream_request_id?: string
  prompt_tokens: number
  completion_tokens: number
  quota: number
  use_time: number
  is_stream: boolean
  profit_cost_status: string
  cost_known: boolean
  cost_profile_id?: string
  cost_profile_name?: string
  billable_prompt_tokens: number
  billable_completion_tokens: number
  upstream_actual_prompt_tokens: number
  upstream_actual_completion_tokens: number
  estimated_revenue_usd: number
  estimated_upstream_cost_usd?: number | null
  gross_margin_usd?: number | null
  gross_margin_pct?: number | null
  expected_cost_usd?: number | null
  expected_margin_usd?: number | null
  expected_margin_pct?: number | null
  compression_saved_tokens: number
  compression_mode?: string
  compression_savings_percent?: number | null
  compression_bypassed?: boolean
  compression_bypass_reason?: string
  compression_rules_version?: string
  compression_rules_applied?: string[]
  compression_preserved_blocks?: number
  compression_redacted_secrets?: number
  profit_route_mode?: string
  profit_route_candidate_count?: number
  profit_route_selected_channel_id?: number
  profit_route_selected_margin_rank?: number
  profit_route_best_channel_id?: number
  profit_route_best_channel_name?: string
  profit_route_best_cost_profile_id?: string
  profit_route_best_expected_margin_usd?: number | null
  profit_route_would_prefer_different?: boolean
  profit_route_candidates?: ProfitRouteDecisionCandidate[]
  cache_saved_usd?: number | null
  retry_cost_usd?: number | null
}

export type ProfitRouteDecisionCandidate = {
  channel_id: number
  channel_name?: string
  cost_known: boolean
  profit_cost_status: string
  cost_profile_id?: string
  cost_profile_name?: string
  expected_cost_usd?: number | null
  expected_margin_usd?: number | null
  expected_margin_pct?: number | null
  selected: boolean
  would_prefer: boolean
  margin_rank: number
  priority?: number
  weight?: number
}

export type ProfitEventsPage = {
  total: number
  items: ProfitEvent[]
  page?: number
  page_size?: number
}

export type ProfitRoutePreviewCostInput = {
  group?: string
  provider?: string
  channel_id?: number
  channel_name?: string
  model_name?: string
  billable_prompt_tokens?: number
  billable_completion_tokens?: number
  upstream_actual_prompt_tokens?: number
  upstream_actual_completion_tokens?: number
  cache_read_tokens?: number
  cache_write_tokens?: number
  estimated_revenue_usd?: number
  latency_ms?: number
  failure_rate?: number
}

export type ProfitRoutePreviewRequest = {
  group?: string
  model_name?: string
  provider?: string
  channel_id?: number
  channel_name?: string
  prompt_tokens?: number
  output_tokens?: number
  estimated_revenue_usd?: number
  candidates?: ProfitRoutePreviewCostInput[]
}

export type ProfitRoutePreviewEstimate = {
  cost_known: boolean
  profit_cost_status: string
  cost_profile_id?: string
  cost_profile_name?: string
  estimated_revenue_usd: number
  estimated_upstream_cost_usd?: number | null
  token_cost_usd?: number | null
  fixed_request_usd: number
  failure_penalty_usd: number
  latency_penalty_usd: number
  risk_penalty_usd: number
  expected_cost_usd?: number | null
  gross_margin_usd?: number | null
  gross_margin_pct?: number | null
  expected_margin_usd?: number | null
  expected_margin_pct?: number | null
}

export type ProfitRoutePreviewCandidate = {
  input: ProfitRoutePreviewCostInput
  estimate: ProfitRoutePreviewEstimate
  would_prefer: boolean
}

export type ProfitRoutePreviewResponse = {
  observe_only: boolean
  live_routing_used: boolean
  routing_mode: ProfitMode
  selected_index: number
  message: string
  candidates: ProfitRoutePreviewCandidate[]
}
