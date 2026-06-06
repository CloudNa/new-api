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
  type ProfitMode,
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
  output_cap_mode: 'off',
  risk_enforcement: 'off',
  settings_writable: true,
  cost_profiles_used: true,
}

const DEFAULT_PROFILES: ProfitCostProfiles = {
  version: 1,
  items: [],
}

const SAMPLE_PROFILE: ProfitCostProfiles = {
  version: 1,
  items: [
    {
      id: 'proxy-test-gpt-load-gemini',
      name: 'proxy-test GPT-Load Gemini',
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
      provider: 'gpt-load',
      channel_id: 0,
      model_name: 'gemini-2.5-flash',
      upstream_actual_prompt_tokens: 24000,
      upstream_actual_completion_tokens: 1000,
      latency_ms: 1800,
      failure_rate: 0.02,
    },
  ],
}

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
            <TableHead>{t('Status')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={6}
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
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const profiles = safeParseJson<ProfitCostProfiles>(profileJson)
      const payload: ProfitSettings = {
        ...settings,
        observe_groups: parseCsv(observeGroups),
        observe_only: true,
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
              {t('Output cap mode')}
            </Label>
            <div className='mt-1.5'>
              <ModeSelect
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
