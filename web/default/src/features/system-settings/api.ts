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

export async function testRtkCompression(request: RtkTestRequest) {
  const res = await api.post('/api/context/rtk/test', request)
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
  response_cache_sources_ok?: boolean
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

export type CavemanIntensity =
  | 'lite'
  | 'full'
  | 'standard'
  | 'aggressive'
  | 'ultra'
export type RtkIntensity = 'minimal' | 'standard' | 'aggressive'
export type RtkRawOutputRetention = 'never' | 'failures' | 'always'

export type CompressionPipelineStep = {
  engine: 'rtk' | 'caveman' | 'lite' | 'aggressive' | 'ultra' | 'standard'
  intensity?: string
}

export type CompressionToolStrategies = {
  file_content: boolean
  grep_search: boolean
  shell_output: boolean
  json: boolean
  error_message: boolean
}

export type CompressionAggressiveThresholds = {
  full_summary: number
  moderate: number
  light: number
  verbatim: number
}

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
    intensity: RtkIntensity
    raw_output_retention: RtkRawOutputRetention
    raw_output_max_bytes: number
    custom_filters_enabled: boolean
    trust_project_filters: boolean
    max_lines: number
    max_chars: number
    deduplicate_threshold: number
    enabled_filters: string[]
    disabled_filters: string[]
    apply_to_tool_results: boolean
    apply_to_assistant_messages: boolean
    apply_to_code_blocks: boolean
  }
  caveman: {
    intensity: CavemanIntensity
    compress_roles: string[]
    skip_rules: string[]
    min_message_length: number
    preserve_patterns: string[]
    language: string
    auto_detect_language: boolean
    enabled_language_packs: string[]
  }
  aggressive: {
    thresholds: CompressionAggressiveThresholds
    tool_strategies: CompressionToolStrategies
    summarizer_enabled: boolean
    max_tokens_per_message: number
    min_savings_threshold: number
  }
  ultra: {
    compression_rate: number
    min_score_threshold: number
    slm_fallback_to_aggressive: boolean
    model_path?: string
    max_tokens_per_message: number
  }
  stacked_pipeline: CompressionPipelineStep[]
  attribution: string
}

export type CompressionEngineBreakdownItem = {
  engine: string
  original_tokens: number
  compressed_tokens: number
  savings_percent: number
  techniques_used?: string[]
  rules_applied?: string[]
  duration_ms?: number
}

export type CompressionStats = {
  original_tokens: number
  compressed_tokens: number
  savings_percent: number
  mode: CompressionMode
  engine?: string
  techniques_used?: string[]
  rules_applied?: string[]
  preserved_block_count: number
  redacted_secret_count: number
  compression_saved_tokens: number
  duration_ms: number
  timestamp?: number
  validation_warnings?: string[]
  validation_errors?: string[]
  fallback_applied?: boolean
  engine_breakdown?: CompressionEngineBreakdownItem[]
  bypassed: boolean
  bypass_reason?: string
  omniroute_compatible_mode?: string
}

export type CompressionPreviewRequest = {
  mode?: CompressionMode
  text?: string
  messages?: Array<{ role: string; content: string }>
}

export type RtkTestRequest = {
  text: string
  command?: string
  skip_filters?: boolean
  code_blocks_only?: boolean
  embedded_outputs_only?: boolean
}

export type CompressionPreviewResponse = {
  text?: string
  messages?: Array<{ role: string; content: string }>
  compressed?: boolean
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
export type ProfitResponseCacheMode = ProfitMode | 'enforce'
export type ProfitCostRoutingMode = ProfitMode | 'prefer_margin'
export type ProfitOutputCapMode = 'off' | 'observe' | 'cap' | 'premium_required'
export type ProfitRetryBudgetMode = ProfitMode | 'enforce'
export type ProfitRiskMode = 'off' | 'alert'

export type ProfitLongContextTier = {
  id: string
  name?: string
  min_context_tokens?: number
  max_context_tokens?: number
  input_multiplier: number
  premium_required?: boolean
}

export type ProfitLongContextPolicy = {
  id: string
  name?: string
  enabled: boolean
  priority?: number
  group?: string
  model_name?: string
  channel_id?: number
  channel_name?: string
  mode?: ProfitMode
  premium_group?: string
  notes?: string
  tiers?: ProfitLongContextTier[]
}

export type ProfitOutputPolicy = {
  id: string
  name?: string
  enabled: boolean
  priority?: number
  group?: string
  model_name?: string
  channel_id?: number
  channel_name?: string
  mode?: ProfitOutputCapMode
  default_max_tokens?: number
  hard_max_tokens?: number
  rewrite_over_limit?: boolean
  premium_required?: boolean
  premium_group?: string
  notes?: string
}

export type ProfitModelAliasTarget = {
  model_name: string
  channel_id?: number
  channel_name?: string
  priority?: number
  weight?: number
  notes?: string
}

export type ProfitModelAlias = {
  id: string
  name?: string
  enabled: boolean
  priority?: number
  group?: string
  sku: string
  mode?: ProfitMode
  targets?: ProfitModelAliasTarget[]
  notes?: string
}

export type ProfitResponseCacheRule = {
  id: string
  name?: string
  enabled: boolean
  priority?: number
  group?: string
  model_name?: string
  channel_id?: number
  channel_name?: string
  mode?: ProfitResponseCacheMode
  scope?: 'global' | 'user' | 'session'
  ttl_seconds?: number
  max_body_bytes?: number
  public_static?: boolean
  notes?: string
}

export type ProfitSettings = {
  version: number
  enabled: boolean
  observe_only: boolean
  observe_groups: string[]
  global_kill_switch: boolean
  cost_routing_mode: ProfitCostRoutingMode
  cost_routing_min_samples?: number
  cost_routing_min_success_rate_pct?: number
  cost_routing_health_window_hours?: number
  cache_mode: ProfitResponseCacheMode
  long_context_mode: ProfitMode
  output_cap_mode: ProfitOutputCapMode
  model_alias_mode: ProfitMode
  retry_budget_mode: ProfitRetryBudgetMode
  max_retry_cost_usd?: number
  retry_low_margin_skip?: boolean
  risk_enforcement: ProfitRiskMode
  risk_min_gross_margin_usd?: number
  risk_min_gross_margin_pct?: number
  risk_min_expected_margin_usd?: number
  risk_min_expected_margin_pct?: number
  settings_writable: boolean
  cost_profiles_used: boolean
  long_context_policies?: ProfitLongContextPolicy[]
  output_policies?: ProfitOutputPolicy[]
  model_aliases?: ProfitModelAlias[]
  response_cache_rules?: ProfitResponseCacheRule[]
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

export type ProfitGuardrailRecommendation = {
  id: string
  action: string
  reason: string
  confidence: string
  mode?: ProfitOutputCapMode | ProfitRiskMode | ProfitMode | ''
  default_max_tokens?: number
  hard_max_tokens?: number
  notes?: string[]
  output_policy_template?: ProfitOutputPolicy | null
  template_json?: string
}

export type ProfitSubscriptionQuotaPlan = {
  plan_id: number
  plan_title: string
  plan_price_amount: number
  plan_currency?: string
  active_subscription_count: number
  active_user_count: number
  unlimited_subscription_count: number
  paid_quota: number
  used_quota: number
  unused_quota: number
  overused_quota: number
  unused_quota_usd: number
  unused_quota_pct: number
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
  output_policy_observed_count: number
  output_policy_exceeded_default_count: number
  output_policy_exceeded_hard_count: number
  output_policy_would_cap_count: number
  output_policy_premium_required_count: number
  output_policy_completion_tokens: number
  output_policy_completion_sample_count: number
  output_policy_completion_avg_tokens: number
  output_policy_completion_p95_tokens: number
  output_policy_completion_p99_tokens: number
  output_policy_completion_max_tokens: number
  output_policy_recommended_default_max_tokens: number
  output_policy_recommended_hard_max_tokens: number
  output_policy_recommendation_confidence: string
  output_policy_recommendation_reason: string
  profit_guardrail_action: string
  profit_guardrail_reason: string
  profit_guardrail_confidence: string
  profit_guardrail_output_mode: ProfitOutputCapMode | ''
  profit_guardrail_default_max_tokens: number
  profit_guardrail_hard_max_tokens: number
  profit_guardrail_policy_template?: ProfitOutputPolicy | null
  profit_guardrail_policy_template_json?: string
  profit_guardrail_recommendations?: ProfitGuardrailRecommendation[]
  profit_risk_observed_count: number
  profit_risk_alert_count: number
  profit_risk_loss_making_count: number
  profit_risk_low_gross_margin_count: number
  profit_risk_low_expected_margin_count: number
  long_context_observed_count: number
  long_context_premium_required_count: number
  long_context_tokens: number
  long_context_suggested_extra_revenue_usd: number
  cache_read_tokens: number
  cache_write_tokens: number
  cache_saved_usd?: number | null
  response_cache_observed_count?: number
  response_cache_hit_count?: number
  response_cache_would_hit_count?: number
  response_cache_live_served_count?: number
  response_cache_stored_count?: number
  response_cache_saved_usd?: number | null
  sku_alias_observed_count?: number
  retry_cost_usd?: number | null
  profit_retry_attempt_count?: number
  profit_retry_budget_would_skip_count?: number
  profit_retry_budget_live_enforced_count?: number
  subscription_active_count?: number
  subscription_active_user_count?: number
  subscription_unlimited_count?: number
  subscription_paid_quota?: number
  subscription_used_quota?: number
  subscription_unused_quota?: number
  subscription_overused_quota?: number
  subscription_unused_quota_usd?: number
  subscription_unused_quota_pct?: number
  subscription_unused_quota_plans?: ProfitSubscriptionQuotaPlan[]
}

export type ProfitRetryAttempt = {
  index: number
  channel_id: number
  channel_name?: string
  model_name?: string
  prompt_tokens?: number
  completion_tokens?: number
  status_code?: number
  error_type?: string
  error_code?: string
  cost_known: boolean
  profit_cost_status: string
  cost_profile_id?: string
  cost_profile_name?: string
  estimated_upstream_cost_usd?: number | null
  expected_retry_cost_usd?: number | null
  base_will_retry?: boolean
  will_retry: boolean
  platform_borne: boolean
  retry_budget_mode?: ProfitRetryBudgetMode
  max_retry_cost_usd?: number
  current_retry_cost_usd?: number
  retry_budget_exceeded?: boolean
  retry_budget_low_margin?: boolean
  retry_budget_would_skip?: boolean
  retry_budget_observe_only?: boolean
  retry_budget_live_enforced?: boolean
  retry_budget_bypass_reason?: string
  retry_budget_reason?: string
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
  compression_engine?: string
  compression_timestamp?: number
  compression_fallback_applied?: boolean
  compression_savings_percent?: number | null
  compression_bypassed?: boolean
  compression_bypass_reason?: string
  compression_rules_version?: string
  compression_rules_applied?: string[]
  compression_preserved_blocks?: number
  compression_redacted_secrets?: number
  compression_validation_warnings?: string[]
  compression_validation_errors?: string[]
  compression_engine_breakdown?: CompressionEngineBreakdownItem[]
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
  profit_route_live_routing_used?: boolean
  profit_route_bypass_reason?: string
  profit_route_health_min_samples?: number
  profit_route_health_min_success_rate_pct?: number
  output_policy_mode?: ProfitOutputCapMode
  output_policy_id?: string
  output_policy_name?: string
  output_policy_completion_tokens?: number
  output_policy_default_max_tokens?: number
  output_policy_hard_max_tokens?: number
  output_policy_exceeded_default?: boolean
  output_policy_exceeded_hard?: boolean
  output_policy_rewrite_over_limit?: boolean
  output_policy_would_cap?: boolean
  output_policy_premium_required?: boolean
  output_policy_premium_group?: string
  output_policy_observe_only?: boolean
  output_policy_live_enforced?: boolean
  profit_risk_mode?: ProfitRiskMode
  profit_risk_alert?: boolean
  profit_risk_reasons?: string[]
  profit_risk_min_gross_margin_usd?: number
  profit_risk_min_gross_margin_pct?: number
  profit_risk_min_expected_margin_usd?: number
  profit_risk_min_expected_margin_pct?: number
  profit_risk_observe_only?: boolean
  profit_risk_live_enforced?: boolean
  long_context_mode?: ProfitMode
  long_context_policy_id?: string
  long_context_policy_name?: string
  long_context_tier_id?: string
  long_context_tier_name?: string
  long_context_tokens?: number
  long_context_min_tokens?: number
  long_context_max_tokens?: number
  long_context_input_multiplier?: number
  long_context_input_revenue_usd?: number
  long_context_suggested_extra_revenue_usd?: number
  long_context_suggested_revenue_usd?: number
  long_context_premium_required?: boolean
  long_context_premium_group?: string
  long_context_observe_only?: boolean
  long_context_live_enforced?: boolean
  cache_read_tokens?: number
  cache_write_tokens?: number
  cache_saved_usd?: number | null
  response_cache_mode?: string
  response_cache_eligible?: boolean
  response_cache_hit?: boolean
  response_cache_would_hit?: boolean
  response_cache_live_served?: boolean
  response_cache_stored?: boolean
  response_cache_rule_id?: string
  response_cache_rule_name?: string
  response_cache_key_hash?: string
  response_cache_scope?: string
  response_cache_saved_usd?: number | null
  sku_alias_applied?: boolean
  sku_alias_mode?: ProfitMode
  sku_alias_id?: string
  sku_alias_name?: string
  sku_alias_sku?: string
  sku_alias_upstream_model?: string
  sku_alias_target_channel_id?: number
  sku_alias_target_channel_name?: string
  sku_alias_candidate_count?: number
  sku_alias_observe_only?: boolean
  retry_cost_usd?: number | null
  profit_retry_attempt_count?: number
  profit_retry_attempts?: ProfitRetryAttempt[]
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
  failure_rate?: number
  health_request_count?: number
  health_success_rate_pct?: number
  health_status?: string
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
  routing_mode: ProfitCostRoutingMode
  selected_index: number
  message: string
  candidates: ProfitRoutePreviewCandidate[]
}
