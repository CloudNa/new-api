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
  type ProfitCostProfiles,
  type ProfitEventsPage,
  type ProfitLongContextPolicy,
  type ProfitMode,
  type ProfitOutputCapMode,
  type ProfitOutputPolicy,
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

const OUTPUT_CAP_OPTIONS: Array<{ value: ProfitOutputCapMode; label: string }> =
  [
    { value: 'off', label: 'Off' },
    { value: 'observe', label: 'Observe' },
    { value: 'cap', label: 'Cap' },
    { value: 'premium_required', label: 'Premium required' },
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
  cache_mode: 'off',
  long_context_mode: 'off',
  output_cap_mode: 'off',
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
  items: [],
}

const SAMPLE_PROFILE: ProfitCostProfiles = {
  version: 1,
  items: [
    {
      id: 'proxy-test-cliproxyapi-gemini',
      name: 'proxy-test CLIProxyAPI Gemini',
      enabled: false,
      priority: 10,
      provider: '',
      channel_id: 0,
      channel_name: '',
      model_name: 'gemini*',
      input_usd_per_million: 0.3,
      output_usd_per_million: 2.5,
      cache_read_usd_per_million: 0.03,
      cache_write_usd_per_million: 0.3,
      fixed_request_usd: 0,
      failure_penalty_usd: 0.002,
      latency_penalty_usd_per_second: 0,
      risk_penalty_usd: 0,
      notes: 'Enable and set channel_id after confirming the upstream channel.',
    },
  ],
}

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
    notes: 'Observe only. Suggested revenue is not charged until billing expressions are changed.',
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

function parseCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
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
    [
      'Compression saved tokens',
      formatNumber(analytics?.compression_saved_tokens),
    ],
    [
      'Missing cost profiles',
      formatNumber(analytics?.missing_cost_profile_count),
    ],
    [
      'Output policy observed',
      formatNumber(analytics?.output_policy_observed_count),
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
      '长上下文观测',
      formatNumber(analytics?.long_context_observed_count),
    ],
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
            rows.slice(0, 8).map((event) => (
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
                          {t(event.profit_route_mode)}
                        </Badge>
                        {event.profit_route_would_prefer_different ? (
                          <Badge variant='secondary'>
                            {t('Would prefer')}
                          </Badge>
                        ) : null}
                      </div>
                      <span className='text-muted-foreground text-xs'>
                        {t('Margin rank')}:{' '}
                        {event.profit_route_selected_margin_rank || '-'} /{' '}
                        {event.profit_route_candidate_count || 0}
                      </span>
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
                          <Badge variant='secondary'>
                            {t('Would cap')}
                          </Badge>
                        ) : null}
                        {event.output_policy_premium_required ? (
                          <Badge variant='secondary'>
                            {t('Premium')}
                          </Badge>
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
                  <Badge variant={event.cost_known ? 'default' : 'secondary'}>
                    {t(event.profit_cost_status || 'unknown')}
                  </Badge>
                </TableCell>
              </TableRow>
            ))
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
        <Badge variant='outline'>{t(result.routing_mode)}</Badge>
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
      const payload: ProfitSettings = {
        ...settings,
        observe_groups: parseCsv(observeGroups),
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

        <SettingsFormGrid>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('Cost routing mode')}
            </Label>
            <div className='mt-1.5'>
              <ModeSelect
                value={settings.cost_routing_mode}
                label='Cost routing mode'
                onChange={(cost_routing_mode) =>
                  setSettings((current) => ({ ...current, cost_routing_mode }))
                }
              />
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
            <h4 className='text-sm font-semibold'>
              {t('长上下文溢价策略')}
            </h4>
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
