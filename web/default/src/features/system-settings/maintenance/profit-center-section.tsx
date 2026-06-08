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
import { useEffect, useMemo, useState } from 'react'
import {
  CalculatorIcon,
  CopyIcon,
  RefreshCcwIcon,
  RouteIcon,
  TrendingUpIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  getProfitAnalytics,
  getProfitCostProfiles,
  getProfitEvents,
  getProfitSettings,
  previewProfitRoute,
  updateProfitCostProfiles,
  updateProfitSettings,
  type ProfitAnalytics,
  type ProfitCostRoutingMode,
  type ProfitCostProfiles,
  type ProfitEventsPage,
  type ProfitGuardrailRecommendation,
  type ProfitLongContextPolicy,
  type ProfitMode,
  type ProfitOutputCapMode,
  type ProfitOutputPolicy,
  type ProfitRetryBudgetMode,
  type ProfitRiskMode,
  type ProfitRoutePreviewRequest,
  type ProfitRoutePreviewResponse,
  type ProfitSettings,
} from '../api'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import {
  SettingsPageFormActions,
  SettingsPageTitleStatusPortal,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

const MODE_OPTIONS: Array<{ value: ProfitMode; label: string }> = [
  { value: 'off', label: 'Off' },
  { value: 'observe', label: 'Observe' },
]

const COST_ROUTING_OPTIONS: Array<{
  value: ProfitCostRoutingMode
  label: string
}> = [
  { value: 'off', label: 'Off' },
  { value: 'observe', label: 'Observe' },
  { value: 'prefer_margin', label: '毛利优先预演' },
]

const PROFIT_SAFE_GROUP = 'proxy-test'

const OUTPUT_CAP_OPTIONS: Array<{ value: ProfitOutputCapMode; label: string }> =
  [
    { value: 'off', label: 'Off' },
    { value: 'observe', label: 'Observe' },
    { value: 'cap', label: 'Cap' },
    { value: 'premium_required', label: 'Premium required' },
  ]

const RETRY_BUDGET_OPTIONS: Array<{
  value: ProfitRetryBudgetMode
  label: string
}> = [
  { value: 'off', label: 'Off' },
  { value: 'observe', label: 'Observe' },
  { value: 'enforce', label: 'Enforce' },
]

const RISK_OPTIONS: Array<{ value: ProfitRiskMode; label: string }> = [
  { value: 'off', label: 'Off' },
  { value: 'alert', label: 'Alert' },
]

const DEFAULT_SETTINGS: ProfitSettings = {
  version: 1,
  enabled: true,
  observe_only: true,
  observe_groups: ['proxy-test'],
  global_kill_switch: false,
  cost_routing_mode: 'observe',
  cost_routing_min_samples: 20,
  cost_routing_min_success_rate_pct: 95,
  cost_routing_health_window_hours: 24,
  cache_mode: 'off',
  long_context_mode: 'off',
  output_cap_mode: 'off',
  retry_budget_mode: 'off',
  max_retry_cost_usd: 0,
  retry_low_margin_skip: false,
  risk_enforcement: 'off',
  risk_min_gross_margin_usd: 0,
  risk_min_gross_margin_pct: 0,
  risk_min_expected_margin_usd: 0,
  risk_min_expected_margin_pct: 0,
  settings_writable: true,
  cost_profiles_used: true,
  long_context_policies: [],
  output_policies: [],
}

const DEFAULT_PROFILES: ProfitCostProfiles = {
  version: 1,
  items: [
    {
      id: 'generic-gpt-5-premium',
      name: 'Generic GPT-5 family fallback estimate',
      enabled: true,
      priority: 10,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'gpt-5*',
      input_usd_per_million: 1.25,
      output_usd_per_million: 10,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.01,
      latency_penalty_usd_per_second: 0.00005,
      risk_penalty_usd: 0,
      notes:
        'Generic fallback for GPT-5 family across any channel/provider; use channel-specific profiles only when real cost differs.',
    },
    {
      id: 'generic-gemini-family',
      name: 'Generic Gemini family fallback estimate',
      enabled: true,
      priority: 9,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'gemini*',
      input_usd_per_million: 0.2,
      output_usd_per_million: 1,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.004,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0002,
      notes:
        'Generic fallback for Gemini family across any channel/provider; override per channel only when needed.',
    },
    {
      id: 'generic-claude-family',
      name: 'Generic Claude family fallback estimate',
      enabled: true,
      priority: 8,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'claude*',
      input_usd_per_million: 3,
      output_usd_per_million: 15,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.02,
      latency_penalty_usd_per_second: 0.00008,
      risk_penalty_usd: 0.0005,
      notes:
        'Generic fallback for Claude family across any channel/provider; override per channel only when needed.',
    },
    {
      id: 'generic-gpt-oss-family',
      name: 'Generic gpt-oss family fallback estimate',
      enabled: true,
      priority: 8,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'gpt-oss*',
      input_usd_per_million: 0.1,
      output_usd_per_million: 0.5,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.003,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0001,
      notes:
        'Generic fallback for gpt-oss family across any channel/provider; override per channel only when needed.',
    },
    {
      id: 'generic-deepseek-family',
      name: 'Generic DeepSeek family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'deepseek*',
      input_usd_per_million: 0.2,
      output_usd_per_million: 0.8,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.004,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0002,
      notes:
        'Generic fallback for DeepSeek family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-qwen-family',
      name: 'Generic Qwen family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'qwen*',
      input_usd_per_million: 0.3,
      output_usd_per_million: 1.2,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.004,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0002,
      notes:
        'Generic fallback for Qwen family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-kimi-family',
      name: 'Generic Kimi/Moonshot family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'kimi*',
      input_usd_per_million: 0.6,
      output_usd_per_million: 2,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.006,
      latency_penalty_usd_per_second: 0.00004,
      risk_penalty_usd: 0.0003,
      notes:
        'Generic fallback for Kimi family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-moonshot-family',
      name: 'Generic Moonshot family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'moonshot*',
      input_usd_per_million: 0.6,
      output_usd_per_million: 2,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.006,
      latency_penalty_usd_per_second: 0.00004,
      risk_penalty_usd: 0.0003,
      notes:
        'Generic fallback for Moonshot family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-kiro-family',
      name: 'Generic Kiro family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'kiro*',
      input_usd_per_million: 3,
      output_usd_per_million: 15,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.02,
      latency_penalty_usd_per_second: 0.00008,
      risk_penalty_usd: 0.0005,
      notes:
        'Generic fallback for Kiro-style CLI/OAuth models across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-grok-family',
      name: 'Generic Grok/xAI family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'grok*',
      input_usd_per_million: 3,
      output_usd_per_million: 15,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.015,
      latency_penalty_usd_per_second: 0.00006,
      risk_penalty_usd: 0.0005,
      notes:
        'Generic fallback for Grok family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-mistral-family',
      name: 'Generic Mistral family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'mistral*',
      input_usd_per_million: 0.5,
      output_usd_per_million: 1.5,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.004,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0002,
      notes:
        'Generic fallback for Mistral family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-llama-family',
      name: 'Generic Llama family fallback estimate',
      enabled: true,
      priority: 7,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'llama*',
      input_usd_per_million: 0.2,
      output_usd_per_million: 0.8,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.003,
      latency_penalty_usd_per_second: 0.00003,
      risk_penalty_usd: 0.0001,
      notes:
        'Generic fallback for Llama family across any channel/provider; replace with exact upstream prices when available.',
    },
    {
      id: 'generic-any-model',
      name: 'Generic any-model fallback estimate',
      enabled: true,
      priority: 1,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: '*',
      input_usd_per_million: 2,
      output_usd_per_million: 8,
      cache_read_usd_per_million: 0,
      cache_write_usd_per_million: 0,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.01,
      latency_penalty_usd_per_second: 0.00005,
      risk_penalty_usd: 0.001,
      notes:
        'Last-resort fallback for any model without a more specific cost profile; use only as an estimate and override exact high-volume channels.',
    },
  ],
}

const SAMPLE_PROFILE: ProfitCostProfiles = DEFAULT_PROFILES

const SAMPLE_PREVIEW: ProfitRoutePreviewRequest = {
  group: 'proxy-test',
  model_name: 'gemini-2.5-flash',
  prompt_tokens: 32000,
  output_tokens: 1000,
  estimated_revenue_usd: 0.08,
  candidates: [
    {
      provider: 'cliproxyapi',
      channel_id: 0,
      model_name: 'gemini-2.5-flash',
      upstream_actual_prompt_tokens: 24000,
      upstream_actual_completion_tokens: 1000,
      latency_ms: 1800,
      failure_rate: 0.02,
    },
  ],
}

const SAMPLE_OUTPUT_POLICIES: ProfitOutputPolicy[] = [
  {
    id: 'proxy-test-long-output',
    name: 'proxy-test long output observe',
    enabled: true,
    priority: 10,
    group: 'proxy-test',
    model_name: '*',
    mode: 'observe',
    default_max_tokens: 4096,
    hard_max_tokens: 8192,
    rewrite_over_limit: false,
    premium_required: false,
    premium_group: 'premium',
    notes: 'Observe only. No request rewriting is applied in v1.',
  },
]

const SAMPLE_LONG_CONTEXT_POLICIES: ProfitLongContextPolicy[] = [
  {
    id: 'proxy-test-long-context-premium',
    name: 'proxy-test long context premium observe',
    enabled: true,
    priority: 10,
    group: 'proxy-test',
    model_name: '*',
    mode: 'observe',
    premium_group: 'premium',
    notes:
      'Observe only. Suggested revenue is not charged until billing expressions are changed.',
    tiers: [
      {
        id: 'base',
        name: '<=32k',
        min_context_tokens: 0,
        max_context_tokens: 32000,
        input_multiplier: 1,
      },
      {
        id: '32k-128k',
        name: '32k-128k',
        min_context_tokens: 32001,
        max_context_tokens: 128000,
        input_multiplier: 1.25,
      },
      {
        id: '128k-512k',
        name: '128k-512k',
        min_context_tokens: 128001,
        max_context_tokens: 512000,
        input_multiplier: 1.75,
      },
      {
        id: '512k-plus',
        name: '>512k',
        min_context_tokens: 512001,
        input_multiplier: 2.5,
        premium_required: true,
      },
    ],
  },
]

function formatJson(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

async function copyText(
  value: string,
  successMessage: string,
  failureMessage: string
) {
  try {
    if (typeof navigator === 'undefined' || !navigator.clipboard) {
      throw new Error('Clipboard is not available')
    }
    await navigator.clipboard.writeText(value)
    toast.success(successMessage)
  } catch {
    toast.error(failureMessage)
  }
}

function parseCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function activeProfitPolicy(policy: { enabled?: boolean; mode?: string }) {
  const mode = (policy.mode || 'observe').trim()
  return policy.enabled === true && mode !== 'off'
}

function policyLabel(policy: { id?: string; name?: string }) {
  return policy.id || policy.name || '未命名策略'
}

function buildProfitSettingsSafetyWarnings(
  observeGroups: string[],
  longContextPolicies: ProfitLongContextPolicy[],
  outputPolicies: ProfitOutputPolicy[]
) {
  const warnings: string[] = []
  const unsafeGroups = observeGroups.filter(
    (group) => group !== PROFIT_SAFE_GROUP
  )
  if (unsafeGroups.length > 0) {
    warnings.push(`收益观测分组当前只允许 ${PROFIT_SAFE_GROUP}`)
  }
  for (const policy of longContextPolicies) {
    if (!activeProfitPolicy(policy)) continue
    if ((policy.group || '').trim() !== PROFIT_SAFE_GROUP) {
      warnings.push(
        `长上下文策略 ${policyLabel(policy)} 必须限定 group=${PROFIT_SAFE_GROUP}`
      )
    }
  }
  for (const policy of outputPolicies) {
    if (!activeProfitPolicy(policy)) continue
    if ((policy.group || '').trim() !== PROFIT_SAFE_GROUP) {
      warnings.push(
        `输出策略 ${policyLabel(policy)} 必须限定 group=${PROFIT_SAFE_GROUP}`
      )
    }
  }
  return warnings
}

function normalizeOutputPolicyList(value: unknown): ProfitOutputPolicy[] {
  if (Array.isArray(value)) return value as ProfitOutputPolicy[]
  if (value && typeof value === 'object') return [value as ProfitOutputPolicy]
  return []
}

function mergeOutputPolicyTemplate(
  existingJson: string,
  templateJson: string
): string {
  const existing = normalizeOutputPolicyList(
    safeParseJson<unknown>(existingJson)
  )
  const template = normalizeOutputPolicyList(
    safeParseJson<unknown>(templateJson)
  )
  const merged = [...existing]

  for (const policy of template) {
    const id = (policy.id || '').trim()
    const index = id
      ? merged.findIndex((item) => (item.id || '').trim() === id)
      : -1
    if (index >= 0) {
      merged[index] = policy
    } else {
      merged.push(policy)
    }
  }

  return formatJson(merged)
}

function formatUSD(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return `$${value.toFixed(6)}`
}

function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '0'
  return value.toLocaleString()
}

function formatPercent(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return `${value.toFixed(2)}%`
}

function formatOutputRecommendationConfidence(
  value: string | undefined
): string {
  switch (value) {
    case 'high':
      return '高'
    case 'medium':
      return '中'
    case 'low':
      return '低'
    case 'none':
      return '无样本'
    default:
      return value || '-'
  }
}

function formatOutputRecommendationReason(value: string | undefined): string {
  switch (value) {
    case 'p95_p99_observed':
      return '基于 P95/P99'
    case 'insufficient_samples':
      return '样本不足'
    case 'no_samples':
      return '暂无样本'
    default:
      return value || '-'
  }
}

function formatCostRoutingMode(value: string | undefined): string {
  switch (value) {
    case 'prefer_margin':
      return '毛利优先预演'
    case 'observe':
      return '观测'
    case 'off':
      return '关闭'
    default:
      return value || '-'
  }
}

function formatProfitGuardrailAction(value: string | undefined): string {
  switch (value) {
    case 'none':
      return '无动作'
    case 'keep_observing':
      return '继续观测'
    case 'keep_observing_output_tail':
      return '继续观测输出长尾'
    case 'collect_more_output_samples':
      return '继续采集输出样本'
    case 'enable_output_cap_observe':
      return '建议灰度输出上限'
    case 'review_pricing_or_cost':
      return '复核定价或成本'
    case 'complete_cost_profiles':
      return '补齐成本档案'
    case 'review_long_context_premium':
      return '复核长上下文溢价'
    case 'review_retry_budget':
      return '复核重试预算'
    default:
      return value || '-'
  }
}

function formatProfitGuardrailReason(value: string | undefined): string {
  switch (value) {
    case 'no_requests':
      return '暂无请求'
    case 'no_margin_risk':
      return '暂无毛利风险'
    case 'output_tail_without_margin_risk':
      return '输出长尾但毛利正常'
    case 'loss_with_output_tail':
      return '亏损且存在输出长尾'
    case 'low_margin_with_output_tail':
      return '低毛利且存在输出长尾'
    case 'margin_risk_with_insufficient_output_samples':
      return '毛利风险但输出样本不足'
    case 'margin_risk_without_output_samples':
      return '毛利风险但暂无输出样本'
    case 'missing_cost_profiles':
      return '存在缺失成本档案'
    case 'long_context_extra_revenue_observed':
      return '观察到长上下文增收空间'
    case 'retry_cost_observed':
      return '观察到重试成本'
    case 'risk_alerts_observed':
      return '观察到毛利风险告警'
    case 'loss_making_requests_observed':
      return '观察到亏损请求'
    default:
      return value || '-'
  }
}

function safeParseJson<T>(value: string): T {
  return JSON.parse(value) as T
}

function parseNumberInput(value: string): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function ModeSelect(props: {
  value: ProfitMode
  label: string
  onChange: (value: ProfitMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={MODE_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as ProfitMode)
      }
    >
      <SelectTrigger className='w-full' aria-label={t(props.label)}>
        <SelectValue placeholder={t('Select mode')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {MODE_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function CostRoutingModeSelect(props: {
  value: ProfitCostRoutingMode
  label: string
  onChange: (value: ProfitCostRoutingMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={COST_ROUTING_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as ProfitCostRoutingMode)
      }
    >
      <SelectTrigger className='w-full' aria-label={t(props.label)}>
        <SelectValue placeholder={t('Select mode')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {COST_ROUTING_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function RiskSelect(props: {
  value: ProfitRiskMode
  label: string
  onChange: (value: ProfitRiskMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={RISK_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as ProfitRiskMode)
      }
    >
      <SelectTrigger className='w-full' aria-label={t(props.label)}>
        <SelectValue placeholder={t('Select risk mode')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {RISK_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function OutputCapModeSelect(props: {
  value: ProfitOutputCapMode
  label: string
  onChange: (value: ProfitOutputCapMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={OUTPUT_CAP_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as ProfitOutputCapMode)
      }
    >
      <SelectTrigger className='w-full' aria-label={t(props.label)}>
        <SelectValue placeholder={t('Select output mode')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {OUTPUT_CAP_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function RetryBudgetModeSelect(props: {
  value: ProfitRetryBudgetMode
  label: string
  onChange: (value: ProfitRetryBudgetMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={RETRY_BUDGET_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as ProfitRetryBudgetMode)
      }
    >
      <SelectTrigger className='w-full' aria-label={t(props.label)}>
        <SelectValue placeholder={t('Select retry budget mode')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {RETRY_BUDGET_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function StatGrid({ analytics }: { analytics: ProfitAnalytics | null }) {
  const { t } = useTranslation()
  const stats = [
    ['Observed requests', formatNumber(analytics?.request_count)],
    ['Revenue estimate', formatUSD(analytics?.estimated_revenue_usd)],
    [
      'Upstream cost estimate',
      formatUSD(analytics?.estimated_upstream_cost_usd),
    ],
    ['Gross margin', formatUSD(analytics?.gross_margin_usd)],
    ['Gross margin percent', formatPercent(analytics?.gross_margin_pct)],
    ['Expected margin', formatUSD(analytics?.expected_margin_usd)],
    ['重试尝试', formatNumber(analytics?.profit_retry_attempt_count)],
    ['重试成本', formatUSD(analytics?.retry_cost_usd)],
    [
      '重试预算本应跳过',
      formatNumber(analytics?.profit_retry_budget_would_skip_count),
    ],
    [
      '重试预算真实拦截',
      formatNumber(analytics?.profit_retry_budget_live_enforced_count),
    ],
    [
      'Compression saved tokens',
      formatNumber(analytics?.compression_saved_tokens),
    ],
    ['Cache read tokens', formatNumber(analytics?.cache_read_tokens)],
    ['Cache write tokens', formatNumber(analytics?.cache_write_tokens)],
    ['Cache saved USD', formatUSD(analytics?.cache_saved_usd)],
    [
      'Missing cost profiles',
      formatNumber(analytics?.missing_cost_profile_count),
    ],
    [
      'Output policy observed',
      formatNumber(analytics?.output_policy_observed_count),
    ],
    [
      '输出样本数',
      formatNumber(analytics?.output_policy_completion_sample_count),
    ],
    [
      '输出平均 token',
      formatNumber(analytics?.output_policy_completion_avg_tokens),
    ],
    [
      '输出 P95 token',
      formatNumber(analytics?.output_policy_completion_p95_tokens),
    ],
    [
      '输出 P99 token',
      formatNumber(analytics?.output_policy_completion_p99_tokens),
    ],
    [
      '输出最大 token',
      formatNumber(analytics?.output_policy_completion_max_tokens),
    ],
    [
      '建议默认上限',
      formatNumber(analytics?.output_policy_recommended_default_max_tokens),
    ],
    [
      '建议硬上限',
      formatNumber(analytics?.output_policy_recommended_hard_max_tokens),
    ],
    [
      '建议可信度',
      formatOutputRecommendationConfidence(
        analytics?.output_policy_recommendation_confidence
      ),
    ],
    [
      '建议依据',
      formatOutputRecommendationReason(
        analytics?.output_policy_recommendation_reason
      ),
    ],
    [
      'Output would cap',
      formatNumber(analytics?.output_policy_would_cap_count),
    ],
    [
      'Output over hard limit',
      formatNumber(analytics?.output_policy_exceeded_hard_count),
    ],
    [
      'Premium output required',
      formatNumber(analytics?.output_policy_premium_required_count),
    ],
    ['风险告警', formatNumber(analytics?.profit_risk_alert_count)],
    ['亏损请求', formatNumber(analytics?.profit_risk_loss_making_count)],
    [
      '低毛利请求',
      formatNumber(analytics?.profit_risk_low_expected_margin_count),
    ],
    [
      '策略建议',
      formatProfitGuardrailAction(analytics?.profit_guardrail_action),
    ],
    [
      '策略依据',
      formatProfitGuardrailReason(analytics?.profit_guardrail_reason),
    ],
    [
      '策略可信度',
      formatOutputRecommendationConfidence(
        analytics?.profit_guardrail_confidence
      ),
    ],
    ['策略建议模式', analytics?.profit_guardrail_output_mode || '-'],
    [
      '策略默认上限',
      formatNumber(analytics?.profit_guardrail_default_max_tokens),
    ],
    ['策略硬上限', formatNumber(analytics?.profit_guardrail_hard_max_tokens)],
    ['长上下文观测', formatNumber(analytics?.long_context_observed_count)],
    [
      '长上下文高级组',
      formatNumber(analytics?.long_context_premium_required_count),
    ],
    [
      '长上下文建议增收',
      formatUSD(analytics?.long_context_suggested_extra_revenue_usd),
    ],
  ]

  return (
    <div className='grid min-w-0 gap-2 sm:grid-cols-2 xl:grid-cols-4'>
      {stats.map(([label, value]) => (
        <div key={label} className='bg-muted/20 min-w-0 rounded-lg border p-3'>
          <div className='text-muted-foreground truncate text-xs'>
            {t(label)}
          </div>
          <div className='mt-1 truncate text-sm font-semibold'>{value}</div>
        </div>
      ))}
    </div>
  )
}

function GuardrailPolicyTemplate({
  analytics,
  onUseTemplate,
}: {
  analytics: ProfitAnalytics | null
  onUseTemplate: (value: string) => void
}) {
  const { t } = useTranslation()
  const fallbackTemplateJson = analytics?.profit_guardrail_policy_template
    ? formatJson([analytics.profit_guardrail_policy_template])
    : (analytics?.profit_guardrail_policy_template_json ?? '')
  const recommendations =
    analytics?.profit_guardrail_recommendations?.length
      ? analytics.profit_guardrail_recommendations
      : fallbackTemplateJson.trim().length > 0
        ? [
            {
              id: 'primary',
              action: analytics?.profit_guardrail_action ?? '',
              reason: analytics?.profit_guardrail_reason ?? '',
              confidence: analytics?.profit_guardrail_confidence ?? '',
              mode: analytics?.profit_guardrail_output_mode,
              default_max_tokens:
                analytics?.profit_guardrail_default_max_tokens,
              hard_max_tokens: analytics?.profit_guardrail_hard_max_tokens,
              output_policy_template:
                analytics?.profit_guardrail_policy_template ?? null,
              template_json: fallbackTemplateJson,
            } satisfies ProfitGuardrailRecommendation,
          ]
        : []
  const hasRecommendations = recommendations.length > 0
  return (
    <div className='min-w-0 space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div className='min-w-0 space-y-1'>
          <h4 className='text-sm font-semibold'>{t('保护建议')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t(
              '基于当前 proxy-test 收益观测生成建议；所有建议仍需手动保存配置后才会生效。'
            )}
          </p>
        </div>
        <Badge variant={hasRecommendations ? 'default' : 'secondary'}>
          {hasRecommendations ? t('有建议') : t('暂无建议')}
        </Badge>
      </div>
      {hasRecommendations ? (
        <div className='min-w-0 divide-y'>
          {recommendations.map((recommendation) => {
            const templateJson = recommendation.output_policy_template
              ? formatJson([recommendation.output_policy_template])
              : (recommendation.template_json ?? '')
            const hasTemplate = templateJson.trim().length > 0
            return (
              <div
                key={`${recommendation.id}-${recommendation.action}`}
                className='min-w-0 space-y-2 py-3 first:pt-0 last:pb-0'
              >
                <div className='flex flex-wrap items-start justify-between gap-2'>
                  <div className='min-w-0'>
                    <div className='truncate text-sm font-medium'>
                      {formatProfitGuardrailAction(recommendation.action)}
                    </div>
                    <div className='text-muted-foreground mt-0.5 text-xs'>
                      {formatProfitGuardrailReason(recommendation.reason)}
                    </div>
                  </div>
                  <div className='flex flex-wrap gap-1'>
                    <Badge variant='outline'>
                      {formatOutputRecommendationConfidence(
                        recommendation.confidence
                      )}
                    </Badge>
                    {recommendation.mode ? (
                      <Badge variant='secondary'>{recommendation.mode}</Badge>
                    ) : null}
                  </div>
                </div>
                {recommendation.default_max_tokens ||
                recommendation.hard_max_tokens ? (
                  <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                    <span>
                      {t('默认上限')}:{' '}
                      {formatNumber(recommendation.default_max_tokens)}
                    </span>
                    <span>
                      {t('硬上限')}:{' '}
                      {formatNumber(recommendation.hard_max_tokens)}
                    </span>
                  </div>
                ) : null}
                {recommendation.notes?.length ? (
                  <div className='text-muted-foreground space-y-1 text-xs'>
                    {recommendation.notes.map((note) => (
                      <div key={note}>{note}</div>
                    ))}
                  </div>
                ) : null}
                {hasTemplate ? (
                  <div className='min-w-0 space-y-2'>
                    <Label
                      htmlFor={`profit-guardrail-policy-template-json-${recommendation.id}`}
                      className='sr-only'
                    >
                      {t('策略模板')}
                    </Label>
                    <Textarea
                      id={`profit-guardrail-policy-template-json-${recommendation.id}`}
                      name={`profit-guardrail-policy-template-json-${recommendation.id}`}
                      rows={6}
                      value={templateJson}
                      readOnly
                      className='font-mono text-xs'
                    />
                    <div className='flex flex-wrap gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          copyText(
                            templateJson,
                            t('已复制策略模板'),
                            t('复制策略模板失败')
                          )
                        }
                      >
                        <CopyIcon data-icon='inline-start' />
                        <span>{t('复制模板')}</span>
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => onUseTemplate(templateJson)}
                      >
                        <CalculatorIcon data-icon='inline-start' />
                        <span>{t('合并模板')}</span>
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            )
          })}
        </div>
      ) : (
        <div className='bg-muted/20 text-muted-foreground rounded-md border border-dashed p-3 text-xs'>
          {t('暂无可套用建议')}
        </div>
      )}
    </div>
  )
}

function ProfitEventsTable({ events }: { events: ProfitEventsPage | null }) {
  const { t } = useTranslation()
  const rows = events?.items ?? []
  return (
    <div className='overflow-x-auto rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Model')}</TableHead>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead>{t('Revenue')}</TableHead>
            <TableHead>{t('Upstream cost')}</TableHead>
            <TableHead>{t('Margin')}</TableHead>
            <TableHead>{t('Compression')}</TableHead>
            <TableHead>{t('Savings')}</TableHead>
            <TableHead>{t('Route')}</TableHead>
            <TableHead>{t('Output policy')}</TableHead>
            <TableHead>{t('长上下文')}</TableHead>
            <TableHead>{t('风险')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={12}
                className='text-muted-foreground h-20 text-center text-sm'
              >
                {t('No profit events yet')}
              </TableCell>
            </TableRow>
          ) : (
            rows.slice(0, 8).map((event) => {
              const retryBudgetWouldSkipCount =
                event.profit_retry_attempts?.filter(
                  (attempt) => attempt.retry_budget_would_skip
                ).length ?? 0
              const retryBudgetLiveEnforcedCount =
                event.profit_retry_attempts?.filter(
                  (attempt) => attempt.retry_budget_live_enforced
                ).length ?? 0
              return (
                <TableRow key={event.id}>
                <TableCell className='max-w-52 truncate font-medium'>
                  {event.model_name || '-'}
                </TableCell>
                <TableCell className='max-w-44 truncate'>
                  {event.channel_name || event.channel || '-'}
                </TableCell>
                <TableCell>{formatUSD(event.estimated_revenue_usd)}</TableCell>
                <TableCell>
                  {formatUSD(event.estimated_upstream_cost_usd)}
                </TableCell>
                <TableCell>
                  {formatUSD(event.gross_margin_usd)}{' '}
                  <span className='text-muted-foreground'>
                    {formatPercent(event.gross_margin_pct)}
                  </span>
                </TableCell>
                <TableCell className='min-w-36'>
                  {event.compression_mode ? (
                    <div className='flex flex-col gap-1'>
                      <Badge
                        variant={
                          event.compression_bypassed ? 'secondary' : 'outline'
                        }
                        className='w-fit'
                      >
                        {t(event.compression_mode)}
                      </Badge>
                      {event.compression_bypassed &&
                      event.compression_bypass_reason ? (
                        <span className='text-muted-foreground max-w-40 truncate text-xs'>
                          {t('Bypassed')}: {event.compression_bypass_reason}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
                <TableCell className='min-w-28'>
                  <div className='flex flex-col'>
                    <span>{formatNumber(event.compression_saved_tokens)}</span>
                    <span className='text-muted-foreground text-xs'>
                      {formatPercent(event.compression_savings_percent)}
                    </span>
                  </div>
                </TableCell>
                <TableCell className='min-w-44'>
                  {event.profit_route_mode ? (
                    <div className='flex min-w-0 flex-col gap-1'>
                      <div className='flex flex-wrap items-center gap-1'>
                        <Badge variant='outline'>
                          {formatCostRoutingMode(event.profit_route_mode)}
                        </Badge>
                        {event.profit_route_would_prefer_different ? (
                          <Badge variant='secondary'>{t('Would prefer')}</Badge>
                        ) : null}
                        {event.profit_route_live_routing_used ? (
                          <Badge variant='secondary'>{t('Live routing')}</Badge>
                        ) : event.profit_route_bypass_reason ? (
                          <Badge variant='secondary'>{t('Bypassed')}</Badge>
                        ) : null}
                      </div>
                      <span className='text-muted-foreground text-xs'>
                        {t('Margin rank')}:{' '}
                        {event.profit_route_selected_margin_rank || '-'} /{' '}
                        {event.profit_route_candidate_count || 0}
                      </span>
                      {event.profit_route_bypass_reason ? (
                        <span className='text-muted-foreground max-w-44 truncate text-xs'>
                          {t('Reason')}: {event.profit_route_bypass_reason}
                        </span>
                      ) : null}
                      {event.profit_route_best_channel_name ||
                      event.profit_route_best_channel_id ? (
                        <span className='text-muted-foreground max-w-44 truncate text-xs'>
                          {t('Best channel')}:{' '}
                          {event.profit_route_best_channel_name ||
                            event.profit_route_best_channel_id}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
                <TableCell className='min-w-44'>
                  {event.output_policy_mode ? (
                    <div className='flex min-w-0 flex-col gap-1'>
                      <div className='flex flex-wrap items-center gap-1'>
                        <Badge variant='outline'>
                          {t(event.output_policy_mode)}
                        </Badge>
                        {event.output_policy_would_cap ? (
                          <Badge variant='secondary'>{t('Would cap')}</Badge>
                        ) : null}
                        {event.output_policy_premium_required ? (
                          <Badge variant='secondary'>{t('Premium')}</Badge>
                        ) : null}
                      </div>
                      <span className='text-muted-foreground text-xs'>
                        {formatNumber(event.output_policy_completion_tokens)} /{' '}
                        {formatNumber(event.output_policy_default_max_tokens)} /{' '}
                        {formatNumber(event.output_policy_hard_max_tokens)}
                      </span>
                      {event.output_policy_name || event.output_policy_id ? (
                        <span className='text-muted-foreground max-w-44 truncate text-xs'>
                          {event.output_policy_name || event.output_policy_id}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
                <TableCell className='min-w-44'>
                  {event.long_context_mode ? (
                    <div className='flex min-w-0 flex-col gap-1'>
                      <div className='flex flex-wrap items-center gap-1'>
                        <Badge variant='outline'>
                          {t(event.long_context_mode)}
                        </Badge>
                        {event.long_context_premium_required ? (
                          <Badge variant='secondary'>{t('Premium')}</Badge>
                        ) : null}
                      </div>
                      <span className='text-muted-foreground text-xs'>
                        {formatNumber(event.long_context_tokens)} /{' '}
                        {event.long_context_input_multiplier?.toFixed(2) ??
                          '1.00'}
                        x
                      </span>
                      <span className='text-muted-foreground max-w-44 truncate text-xs'>
                        {event.long_context_tier_name ||
                          event.long_context_tier_id ||
                          '-'}{' '}
                        {formatUSD(
                          event.long_context_suggested_extra_revenue_usd
                        )}
                      </span>
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
                <TableCell className='min-w-44'>
                  {event.profit_risk_mode ? (
                    <div className='flex min-w-0 flex-col gap-1'>
                      <div className='flex flex-wrap items-center gap-1'>
                        <Badge
                          variant={
                            event.profit_risk_alert ? 'destructive' : 'outline'
                          }
                        >
                          {t(event.profit_risk_mode)}
                        </Badge>
                        {event.profit_risk_alert ? (
                          <Badge variant='secondary'>{t('告警')}</Badge>
                        ) : null}
                      </div>
                      {event.profit_risk_reasons?.length ? (
                        <span className='text-muted-foreground max-w-44 truncate text-xs'>
                          {event.profit_risk_reasons.join(', ')}
                        </span>
                      ) : null}
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
                <TableCell>
                  <div className='flex min-w-0 flex-col gap-1'>
                    <Badge
                      className='w-fit'
                      variant={event.cost_known ? 'default' : 'secondary'}
                    >
                      {t(event.profit_cost_status || 'unknown')}
                    </Badge>
                    {event.profit_retry_attempt_count ? (
                      <span className='text-muted-foreground max-w-36 truncate text-xs'>
                        {t('重试')}: {event.profit_retry_attempt_count} /{' '}
                        {formatUSD(event.retry_cost_usd)}
                      </span>
                    ) : null}
                    {retryBudgetWouldSkipCount ? (
                      <span className='text-muted-foreground max-w-36 truncate text-xs'>
                        {t('预算')}: {retryBudgetWouldSkipCount}
                        {retryBudgetLiveEnforcedCount
                          ? ` / ${retryBudgetLiveEnforcedCount}`
                          : ''}
                      </span>
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
              )
            })
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function RoutePreviewResult({
  result,
}: {
  result: ProfitRoutePreviewResponse | null
}) {
  const { t } = useTranslation()
  if (!result) return null

  return (
    <div className='min-w-0 space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Badge variant='secondary'>
          {result.observe_only ? t('Observe only') : t('Live routing')}
        </Badge>
        <Badge variant='outline'>
          {formatCostRoutingMode(result.routing_mode)}
        </Badge>
        <span className='text-muted-foreground text-xs'>{result.message}</span>
      </div>
      <div className='overflow-x-auto rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Candidate')}</TableHead>
              <TableHead>{t('Profile')}</TableHead>
              <TableHead>{t('Expected cost')}</TableHead>
              <TableHead>{t('Expected margin')}</TableHead>
              <TableHead>{t('Gross margin')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {result.candidates.map((candidate, index) => (
              <TableRow key={`${candidate.input.channel_id ?? 0}-${index}`}>
                <TableCell className='max-w-56 truncate'>
                  {candidate.input.channel_name ||
                    candidate.input.provider ||
                    candidate.input.channel_id ||
                    t('Candidate')}
                  {candidate.would_prefer ? (
                    <Badge variant='default' className='ml-2'>
                      {t('Would prefer')}
                    </Badge>
                  ) : null}
                </TableCell>
                <TableCell className='max-w-52 truncate'>
                  {candidate.estimate.cost_profile_name ||
                    candidate.estimate.profit_cost_status}
                </TableCell>
                <TableCell>
                  {formatUSD(candidate.estimate.expected_cost_usd)}
                </TableCell>
                <TableCell>
                  {formatUSD(candidate.estimate.expected_margin_usd)}{' '}
                  <span className='text-muted-foreground'>
                    {formatPercent(candidate.estimate.expected_margin_pct)}
                  </span>
                </TableCell>
                <TableCell>
                  {formatUSD(candidate.estimate.gross_margin_usd)}{' '}
                  <span className='text-muted-foreground'>
                    {formatPercent(candidate.estimate.gross_margin_pct)}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

export function ProfitCenterSection() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [settings, setSettings] = useState<ProfitSettings>(DEFAULT_SETTINGS)
  const [initialSettings, setInitialSettings] =
    useState<ProfitSettings>(DEFAULT_SETTINGS)
  const [observeGroups, setObserveGroups] = useState('proxy-test')
  const [outputPolicyJson, setOutputPolicyJson] = useState(
    formatJson(DEFAULT_SETTINGS.output_policies ?? [])
  )
  const [longContextPolicyJson, setLongContextPolicyJson] = useState(
    formatJson(DEFAULT_SETTINGS.long_context_policies ?? [])
  )
  const [profileJson, setProfileJson] = useState(formatJson(DEFAULT_PROFILES))
  const [analyticsGroup, setAnalyticsGroup] = useState('proxy-test')
  const [analyticsModel, setAnalyticsModel] = useState('')
  const [analytics, setAnalytics] = useState<ProfitAnalytics | null>(null)
  const [events, setEvents] = useState<ProfitEventsPage | null>(null)
  const [previewJson, setPreviewJson] = useState(formatJson(SAMPLE_PREVIEW))
  const [previewResult, setPreviewResult] =
    useState<ProfitRoutePreviewResponse | null>(null)
  const [safetyWarnings, setSafetyWarnings] = useState<string[]>([])

  const isActive = settings.enabled && !settings.global_kill_switch

  const analyticsParams = useMemo(
    () => ({
      group: analyticsGroup || undefined,
      model_name: analyticsModel || undefined,
      p: 1,
      page_size: 8,
    }),
    [analyticsGroup, analyticsModel]
  )

  const loadAnalytics = async () => {
    const [analyticsRes, eventsRes] = await Promise.all([
      getProfitAnalytics(analyticsParams),
      getProfitEvents(analyticsParams),
    ])
    if (analyticsRes.success && analyticsRes.data) {
      setAnalytics(analyticsRes.data)
    } else {
      toast.error(analyticsRes.message || t('Failed to load profit analytics'))
    }
    if (eventsRes.success && eventsRes.data) {
      setEvents(eventsRes.data)
    } else {
      toast.error(eventsRes.message || t('Failed to load profit events'))
    }
  }

  const loadAll = async () => {
    setLoading(true)
    try {
      const [settingsRes, profilesRes] = await Promise.all([
        getProfitSettings(),
        getProfitCostProfiles(),
      ])
      if (settingsRes.success && settingsRes.data) {
        const next = { ...DEFAULT_SETTINGS, ...settingsRes.data }
        setSettings(next)
        setInitialSettings(next)
        setObserveGroups((next.observe_groups ?? ['proxy-test']).join(', '))
        setLongContextPolicyJson(formatJson(next.long_context_policies ?? []))
        setOutputPolicyJson(formatJson(next.output_policies ?? []))
      } else {
        toast.error(settingsRes.message || t('Failed to load profit settings'))
      }
      if (profilesRes.success && profilesRes.data) {
        setProfileJson(formatJson({ ...DEFAULT_PROFILES, ...profilesRes.data }))
      } else {
        toast.error(profilesRes.message || t('Failed to load cost profiles'))
      }
      await loadAnalytics()
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Failed to load profit center')
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadAll()
  }, [])

  const handleReset = () => {
    setSettings(initialSettings)
    setSafetyWarnings([])
    setObserveGroups(
      (initialSettings.observe_groups ?? ['proxy-test']).join(', ')
    )
    setLongContextPolicyJson(
      formatJson(initialSettings.long_context_policies ?? [])
    )
    setOutputPolicyJson(formatJson(initialSettings.output_policies ?? []))
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const profiles = safeParseJson<ProfitCostProfiles>(profileJson)
      const longContextPolicies = safeParseJson<ProfitLongContextPolicy[]>(
        longContextPolicyJson
      )
      const outputPolicies =
        safeParseJson<ProfitOutputPolicy[]>(outputPolicyJson)
      const parsedObserveGroups = parseCsv(observeGroups)
      const warnings = buildProfitSettingsSafetyWarnings(
        parsedObserveGroups,
        longContextPolicies,
        outputPolicies
      )
      setSafetyWarnings(warnings)
      if (warnings.length > 0) {
        toast.error(warnings[0])
        return
      }
      const payload: ProfitSettings = {
        ...settings,
        observe_groups: parsedObserveGroups,
        observe_only: true,
        long_context_policies: longContextPolicies,
        output_policies: outputPolicies,
      }
      const [settingsRes, profilesRes] = await Promise.all([
        updateProfitSettings(payload),
        updateProfitCostProfiles(profiles),
      ])
      if (!settingsRes.success || !settingsRes.data) {
        toast.error(settingsRes.message || t('Failed to save profit settings'))
        return
      }
      if (!profilesRes.success || !profilesRes.data) {
        toast.error(profilesRes.message || t('Failed to save cost profiles'))
        return
      }
      setSettings(settingsRes.data)
      setInitialSettings(settingsRes.data)
      setObserveGroups(settingsRes.data.observe_groups.join(', '))
      setLongContextPolicyJson(
        formatJson(settingsRes.data.long_context_policies ?? [])
      )
      setOutputPolicyJson(formatJson(settingsRes.data.output_policies ?? []))
      setProfileJson(formatJson(profilesRes.data))
      setSafetyWarnings([])
      toast.success(t('Profit settings saved.'))
      await loadAnalytics()
    } catch (error) {
      const message =
        error instanceof SyntaxError
          ? t('Invalid profit JSON')
          : error instanceof Error
            ? error.message
            : t('Failed to save profit settings')
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  const loadSampleProfile = () => {
    setProfileJson(formatJson(SAMPLE_PROFILE))
  }

  const loadSampleOutputPolicies = () => {
    setOutputPolicyJson(formatJson(SAMPLE_OUTPUT_POLICIES))
  }

  const useGuardrailPolicyTemplate = (templateJson: string) => {
    try {
      setOutputPolicyJson(
        mergeOutputPolicyTemplate(outputPolicyJson, templateJson)
      )
      toast.success(t('已合并策略模板，保存后生效'))
    } catch {
      toast.error(t('策略模板合并失败，请检查输出策略 JSON'))
    }
  }

  const loadSampleLongContextPolicies = () => {
    setLongContextPolicyJson(formatJson(SAMPLE_LONG_CONTEXT_POLICIES))
  }

  const runRoutePreview = async () => {
    setTesting(true)
    try {
      const request = safeParseJson<ProfitRoutePreviewRequest>(previewJson)
      const res = await previewProfitRoute(request)
      if (!res.success || !res.data) {
        toast.error(res.message || t('Route preview failed'))
        return
      }
      setPreviewResult(res.data)
    } catch (error) {
      const message =
        error instanceof SyntaxError
          ? t('Invalid route preview JSON')
          : error instanceof Error
            ? error.message
            : t('Route preview failed')
      toast.error(message)
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <SettingsSection title={t('Profit Center')}>
        <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
          {t('Loading profit center...')}
        </div>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Profit Center')}>
      <SettingsPageTitleStatusPortal>
        <Badge variant={isActive ? 'default' : 'secondary'}>
          {isActive ? t('Observe active') : t('Observe disabled')}
        </Badge>
      </SettingsPageTitleStatusPortal>
      <SettingsPageFormActions
        onSave={handleSave}
        onReset={handleReset}
        isSaving={saving}
        isSaveDisabled={loading}
        saveLabel='Save profit settings'
      />

      <SettingsForm>
        <SettingsControlGroup>
          <SettingsSwitchField
            label={t('Enable profit observation')}
            description={t(
              'Records root-only revenue and upstream cost fields without changing user billing.'
            )}
            checked={settings.enabled}
            onCheckedChange={(enabled) =>
              setSettings((current) => ({ ...current, enabled }))
            }
          />
          <SettingsSwitchField
            label={t('Global profit kill switch')}
            description={t(
              'Disables profit observation hooks while leaving normal relay behavior untouched.'
            )}
            checked={settings.global_kill_switch}
            onCheckedChange={(global_kill_switch) =>
              setSettings((current) => ({ ...current, global_kill_switch }))
            }
          />
        </SettingsControlGroup>

        {safetyWarnings.length > 0 ? (
          <div className='border-destructive/40 bg-destructive/5 text-destructive min-w-0 rounded-lg border p-3 text-sm'>
            <div className='font-medium'>{t('保存前保护')}</div>
            <div className='mt-1 space-y-1 text-xs'>
              {safetyWarnings.map((warning) => (
                <div key={warning}>{warning}</div>
              ))}
            </div>
          </div>
        ) : null}

        <SettingsFormGrid>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('Cost routing mode')}
            </Label>
            <div className='mt-1.5'>
              <CostRoutingModeSelect
                value={settings.cost_routing_mode}
                label='Cost routing mode'
                onChange={(cost_routing_mode) =>
                  setSettings((current) => ({ ...current, cost_routing_mode }))
                }
              />
              {settings.cost_routing_mode === 'prefer_margin' ? (
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t(
                    'prefer_margin 当前仅在 proxy-test 预演，不会改变真实路由。'
                  )}
                </p>
              ) : null}
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>{t('Cache mode')}</Label>
            <div className='mt-1.5'>
              <ModeSelect
                value={settings.cache_mode}
                label='Cache mode'
                onChange={(cache_mode) =>
                  setSettings((current) => ({ ...current, cache_mode }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('长上下文溢价模式')}
            </Label>
            <div className='mt-1.5'>
              <ModeSelect
                value={settings.long_context_mode}
                label='长上下文溢价模式'
                onChange={(long_context_mode) =>
                  setSettings((current) => ({
                    ...current,
                    long_context_mode,
                  }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('Output cap mode')}
            </Label>
            <div className='mt-1.5'>
              <OutputCapModeSelect
                value={settings.output_cap_mode}
                label='Output cap mode'
                onChange={(output_cap_mode) =>
                  setSettings((current) => ({ ...current, output_cap_mode }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('重试预算模式')}
            </Label>
            <div className='mt-1.5'>
              <RetryBudgetModeSelect
                value={settings.retry_budget_mode}
                label='重试预算模式'
                onChange={(retry_budget_mode) =>
                  setSettings((current) => ({
                    ...current,
                    retry_budget_mode,
                  }))
                }
              />
              {settings.retry_budget_mode === 'enforce' ? (
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('全局 observe_only 开启时仍只记录，不会真实拦截重试。')}
                </p>
              ) : null}
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label
              htmlFor='profit-max-retry-cost-usd'
              className='text-sm font-medium'
            >
              {t('单请求重试预算 USD')}
            </Label>
            <Input
              id='profit-max-retry-cost-usd'
              name='profit-max-retry-cost-usd'
              type='number'
              min='0'
              step='0.000001'
              className='mt-1.5'
              value={settings.max_retry_cost_usd ?? 0}
              onChange={(event) =>
                setSettings((current) => ({
                  ...current,
                  max_retry_cost_usd: parseNumberInput(event.target.value),
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem span='full'>
            <SettingsSwitchField
              label={t('低毛利重试保护')}
              description={t(
                '记录低毛利请求本应停止重试的次数；observe_only 关闭前不会真实拦截。'
              )}
              checked={settings.retry_low_margin_skip ?? false}
              onCheckedChange={(retry_low_margin_skip) =>
                setSettings((current) => ({
                  ...current,
                  retry_low_margin_skip,
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('Risk enforcement')}
            </Label>
            <div className='mt-1.5'>
              <RiskSelect
                value={settings.risk_enforcement}
                label='Risk enforcement'
                onChange={(risk_enforcement) =>
                  setSettings((current) => ({ ...current, risk_enforcement }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label
              htmlFor='profit-risk-min-gross-usd'
              className='text-sm font-medium'
            >
              {t('最低毛利 USD')}
            </Label>
            <Input
              id='profit-risk-min-gross-usd'
              name='profit-risk-min-gross-usd'
              type='number'
              min='0'
              step='0.000001'
              className='mt-1.5'
              value={settings.risk_min_gross_margin_usd ?? 0}
              onChange={(event) =>
                setSettings((current) => ({
                  ...current,
                  risk_min_gross_margin_usd: parseNumberInput(
                    event.target.value
                  ),
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label
              htmlFor='profit-risk-min-gross-pct'
              className='text-sm font-medium'
            >
              {t('最低毛利率 %')}
            </Label>
            <Input
              id='profit-risk-min-gross-pct'
              name='profit-risk-min-gross-pct'
              type='number'
              min='0'
              step='0.01'
              className='mt-1.5'
              value={settings.risk_min_gross_margin_pct ?? 0}
              onChange={(event) =>
                setSettings((current) => ({
                  ...current,
                  risk_min_gross_margin_pct: parseNumberInput(
                    event.target.value
                  ),
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label
              htmlFor='profit-risk-min-expected-usd'
              className='text-sm font-medium'
            >
              {t('最低预期毛利 USD')}
            </Label>
            <Input
              id='profit-risk-min-expected-usd'
              name='profit-risk-min-expected-usd'
              type='number'
              min='0'
              step='0.000001'
              className='mt-1.5'
              value={settings.risk_min_expected_margin_usd ?? 0}
              onChange={(event) =>
                setSettings((current) => ({
                  ...current,
                  risk_min_expected_margin_usd: parseNumberInput(
                    event.target.value
                  ),
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label
              htmlFor='profit-risk-min-expected-pct'
              className='text-sm font-medium'
            >
              {t('最低预期毛利率 %')}
            </Label>
            <Input
              id='profit-risk-min-expected-pct'
              name='profit-risk-min-expected-pct'
              type='number'
              min='0'
              step='0.01'
              className='mt-1.5'
              value={settings.risk_min_expected_margin_pct ?? 0}
              onChange={(event) =>
                setSettings((current) => ({
                  ...current,
                  risk_min_expected_margin_pct: parseNumberInput(
                    event.target.value
                  ),
                }))
              }
            />
          </SettingsFormGridItem>
          <SettingsFormGridItem span='full'>
            <Label
              htmlFor='profit-observe-groups'
              className='text-sm font-medium'
            >
              {t('Observe groups')}
            </Label>
            <Input
              id='profit-observe-groups'
              name='profit-observe-groups'
              className='mt-1.5'
              value={observeGroups}
              onChange={(event) => setObserveGroups(event.target.value)}
              placeholder='proxy-test'
            />
          </SettingsFormGridItem>
        </SettingsFormGrid>
      </SettingsForm>

      <Separator />

      <div className='min-w-0 space-y-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='min-w-0'>
            <h4 className='text-sm font-semibold'>{t('长上下文溢价策略')}</h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                '仅记录长上下文阶梯、建议倍率和建议增收；不会改变当前用户扣费。'
              )}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={loadSampleLongContextPolicies}
          >
            <CalculatorIcon data-icon='inline-start' />
            <span>{t('加载长上下文示例')}</span>
          </Button>
        </div>
        <Label htmlFor='profit-long-context-policies-json' className='sr-only'>
          {t('长上下文溢价策略')}
        </Label>
        <Textarea
          id='profit-long-context-policies-json'
          name='profit-long-context-policies-json'
          rows={11}
          value={longContextPolicyJson}
          onChange={(event) => setLongContextPolicyJson(event.target.value)}
          className='font-mono text-xs'
        />
      </div>

      <Separator />

      <div className='min-w-0 space-y-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='min-w-0'>
            <h4 className='text-sm font-semibold'>{t('Output policies')}</h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Output limits are observed for proxy-test first. Cap and premium modes only record would-have-happened decisions in v1.'
              )}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={loadSampleOutputPolicies}
          >
            <CalculatorIcon data-icon='inline-start' />
            <span>{t('Load output sample')}</span>
          </Button>
        </div>
        <Label htmlFor='profit-output-policies-json' className='sr-only'>
          {t('Output policies')}
        </Label>
        <Textarea
          id='profit-output-policies-json'
          name='profit-output-policies-json'
          rows={8}
          value={outputPolicyJson}
          onChange={(event) => setOutputPolicyJson(event.target.value)}
          className='font-mono text-xs'
        />
      </div>

      <Separator />

      <div className='min-w-0 space-y-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='min-w-0'>
            <h4 className='text-sm font-semibold'>{t('Cost profiles')}</h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Profiles are root-only estimates. Unknown profiles stay observable and never alter routing.'
              )}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={loadSampleProfile}
          >
            <CalculatorIcon data-icon='inline-start' />
            <span>{t('Load sample')}</span>
          </Button>
        </div>
        <Label htmlFor='profit-cost-profiles-json' className='sr-only'>
          {t('Cost profiles')}
        </Label>
        <Textarea
          id='profit-cost-profiles-json'
          name='profit-cost-profiles-json'
          rows={13}
          value={profileJson}
          onChange={(event) => setProfileJson(event.target.value)}
          className='font-mono text-xs'
        />
      </div>

      <Separator />

      <div className='min-w-0 space-y-3'>
        <div className='flex flex-wrap items-end justify-between gap-3'>
          <div className='min-w-0'>
            <h4 className='flex items-center gap-2 text-sm font-semibold'>
              <TrendingUpIcon className='size-4' />
              {t('Profit analytics')}
            </h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Root-only view of platform billing, upstream cost, and margin.'
              )}
            </p>
          </div>
          <div className='flex min-w-0 flex-wrap items-end gap-2'>
            <div className='min-w-36 space-y-1'>
              <Label htmlFor='profit-analytics-group' className='text-xs'>
                {t('Group')}
              </Label>
              <Input
                id='profit-analytics-group'
                name='profit-analytics-group'
                value={analyticsGroup}
                onChange={(event) => setAnalyticsGroup(event.target.value)}
                placeholder='proxy-test'
              />
            </div>
            <div className='min-w-44 space-y-1'>
              <Label htmlFor='profit-analytics-model' className='text-xs'>
                {t('Model')}
              </Label>
              <Input
                id='profit-analytics-model'
                name='profit-analytics-model'
                value={analyticsModel}
                onChange={(event) => setAnalyticsModel(event.target.value)}
                placeholder='gemini'
              />
            </div>
            <Button type='button' variant='outline' onClick={loadAnalytics}>
              <RefreshCcwIcon data-icon='inline-start' />
              <span>{t('Refresh')}</span>
            </Button>
          </div>
        </div>
        <StatGrid analytics={analytics} />
        <GuardrailPolicyTemplate
          analytics={analytics}
          onUseTemplate={useGuardrailPolicyTemplate}
        />
        <ProfitEventsTable events={events} />
      </div>

      <Separator />

      <div className='min-w-0 space-y-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='min-w-0'>
            <h4 className='flex items-center gap-2 text-sm font-semibold'>
              <RouteIcon className='size-4' />
              {t('Route preview')}
            </h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Computes expected margin diagnostics only; live routing is unchanged.'
              )}
            </p>
          </div>
          <Button type='button' onClick={runRoutePreview} disabled={testing}>
            <RouteIcon data-icon='inline-start' />
            <span>{t('Preview route')}</span>
          </Button>
        </div>
        <Label htmlFor='profit-route-preview-json' className='sr-only'>
          {t('Route preview')}
        </Label>
        <Textarea
          id='profit-route-preview-json'
          name='profit-route-preview-json'
          rows={10}
          value={previewJson}
          onChange={(event) => setPreviewJson(event.target.value)}
          className='font-mono text-xs'
        />
        <RoutePreviewResult result={previewResult} />
      </div>
    </SettingsSection>
  )
}
