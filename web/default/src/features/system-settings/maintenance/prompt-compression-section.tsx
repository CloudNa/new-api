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
  FlaskConicalIcon,
  RefreshCcwIcon,
  RotateCcwIcon,
  WandSparklesIcon,
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
import { Textarea } from '@/components/ui/textarea'
import {
  getCompressionSettings,
  getRtkFilters,
  previewCompression,
  testRtkCompression,
  updateCompressionSettings,
  type CavemanIntensity,
  type CompressionPipelineStep,
  type CompressionMode,
  type CompressionPreviewResponse,
  type CompressionStats,
  type PromptCompressionSettings,
  type RtkIntensity,
  type RtkFiltersResponse,
  type RtkRawOutputRetention,
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

const MODE_OPTIONS: Array<{ value: CompressionMode; label: string }> = [
  { value: 'off', label: 'Off' },
  { value: 'lite', label: 'Lite' },
  { value: 'standard', label: 'Standard' },
  { value: 'aggressive', label: 'Aggressive' },
  { value: 'ultra', label: 'Ultra' },
  { value: 'rtk', label: 'RTK' },
  { value: 'stacked', label: 'Stacked' },
]

const INTENSITY_OPTIONS: Array<{ value: CavemanIntensity; label: string }> = [
  { value: 'lite', label: 'Lite' },
  { value: 'full', label: 'Full' },
  { value: 'standard', label: 'Standard' },
  { value: 'aggressive', label: 'Aggressive' },
  { value: 'ultra', label: 'Ultra' },
]

const RTK_INTENSITY_OPTIONS: Array<{ value: RtkIntensity; label: string }> = [
  { value: 'minimal', label: 'Minimal' },
  { value: 'standard', label: 'Standard' },
  { value: 'aggressive', label: 'Aggressive' },
]

const RTK_RAW_OUTPUT_OPTIONS: Array<{
  value: RtkRawOutputRetention
  label: string
}> = [
  { value: 'never', label: 'Never' },
  { value: 'failures', label: 'Failures only' },
  { value: 'always', label: 'Always' },
]

const DEFAULT_SETTINGS: PromptCompressionSettings = {
  enabled: false,
  default_mode: 'off',
  auto_trigger_mode: 'lite',
  auto_trigger_tokens: 0,
  min_tokens: 0,
  preserve_system_prompt: true,
  allowed_groups: ['proxy-test'],
  group_modes: {},
  model_modes: {},
  channel_modes: {},
  global_kill_switch: false,
  rtk: {
    intensity: 'minimal',
    raw_output_retention: 'never',
    raw_output_max_bytes: 1048576,
    custom_filters_enabled: true,
    trust_project_filters: false,
    max_lines: 120,
    max_chars: 12000,
    deduplicate_threshold: 3,
    enabled_filters: [],
    disabled_filters: [],
    apply_to_tool_results: true,
    apply_to_assistant_messages: false,
    apply_to_code_blocks: false,
  },
  caveman: {
    intensity: 'lite',
    compress_roles: ['user'],
    skip_rules: [],
    min_message_length: 50,
    preserve_patterns: [],
    language: 'en',
    auto_detect_language: false,
    enabled_language_packs: ['en'],
  },
  aggressive: {
    thresholds: {
      full_summary: 5,
      moderate: 3,
      light: 2,
      verbatim: 2,
    },
    tool_strategies: {
      file_content: true,
      grep_search: true,
      shell_output: true,
      json: true,
      error_message: true,
    },
    summarizer_enabled: true,
    max_tokens_per_message: 2048,
    min_savings_threshold: 0.05,
  },
  ultra: {
    compression_rate: 0.5,
    min_score_threshold: 0.3,
    slm_fallback_to_aggressive: true,
    model_path: '',
    max_tokens_per_message: 0,
  },
  stacked_pipeline: [
    { engine: 'rtk', intensity: 'standard' },
    { engine: 'caveman', intensity: 'full' },
  ],
  attribution: '',
}

type DraftTextFields = {
  allowedGroups: string
  groupModes: string
  modelModes: string
  channelModes: string
  enabledFilters: string
  disabledFilters: string
  compressRoles: string
  skipRules: string
  preservePatterns: string
  enabledLanguagePacks: string
  stackedPipeline: string
}

const SAMPLE_PROMPT =
  'Please explain in detail what I need to do, and provide a detailed explanation with repeated repeated repeated wording.'

function formatCsv(items: string[] | undefined): string {
  return (items ?? []).join(', ')
}

function parseCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function formatJson(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2)
}

function parseModeMap(
  value: string,
  label: string
): Record<string, CompressionMode> {
  const trimmed = value.trim()
  if (!trimmed) return {}
  const parsed = JSON.parse(trimmed) as Record<string, unknown>
  if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new SyntaxError(`${label} must be a JSON object`)
  }
  return Object.fromEntries(
    Object.entries(parsed).map(([key, mode]) => [
      key,
      String(mode) as CompressionMode,
    ])
  )
}

function parseStackedPipeline(value: string): CompressionPipelineStep[] {
  const trimmed = value.trim()
  if (!trimmed) return DEFAULT_SETTINGS.stacked_pipeline
  const parsed = JSON.parse(trimmed) as unknown
  if (!Array.isArray(parsed)) {
    throw new SyntaxError('Stacked pipeline must be a JSON array')
  }
  return parsed.map((item) => {
    if (typeof item === 'string') {
      return { engine: item as CompressionPipelineStep['engine'] }
    }
    if (item === null || Array.isArray(item) || typeof item !== 'object') {
      throw new SyntaxError('Stacked pipeline items must be strings or objects')
    }
    const record = item as Record<string, unknown>
    return {
      engine: String(record.engine ?? '') as CompressionPipelineStep['engine'],
      ...(record.intensity !== undefined
        ? { intensity: String(record.intensity) }
        : {}),
    }
  })
}

function numberValue(value: number | undefined, fallback: number): number {
  const next = Number(value)
  return Number.isFinite(next) ? next : fallback
}

function positiveNumberValue(
  value: number | undefined,
  fallback: number
): number {
  const next = numberValue(value, fallback)
  return next > 0 ? next : fallback
}

function nonNegativeNumberValue(
  value: number | undefined,
  fallback = 0
): number {
  const next = numberValue(value, fallback)
  return next >= 0 ? next : fallback
}

function mergeCompressionSettings(
  value?: Partial<PromptCompressionSettings>
): PromptCompressionSettings {
  return {
    ...DEFAULT_SETTINGS,
    ...value,
    rtk: {
      ...DEFAULT_SETTINGS.rtk,
      ...(value?.rtk ?? {}),
    },
    caveman: {
      ...DEFAULT_SETTINGS.caveman,
      ...(value?.caveman ?? {}),
    },
    aggressive: {
      ...DEFAULT_SETTINGS.aggressive,
      ...(value?.aggressive ?? {}),
      thresholds: {
        ...DEFAULT_SETTINGS.aggressive.thresholds,
        ...(value?.aggressive?.thresholds ?? {}),
      },
      tool_strategies: {
        ...DEFAULT_SETTINGS.aggressive.tool_strategies,
        ...(value?.aggressive?.tool_strategies ?? {}),
      },
    },
    ultra: {
      ...DEFAULT_SETTINGS.ultra,
      ...(value?.ultra ?? {}),
    },
    stacked_pipeline:
      value?.stacked_pipeline ?? DEFAULT_SETTINGS.stacked_pipeline,
  }
}

function draftFromSettings(
  settings: PromptCompressionSettings
): DraftTextFields {
  return {
    allowedGroups: formatCsv(settings.allowed_groups),
    groupModes: formatJson(settings.group_modes),
    modelModes: formatJson(settings.model_modes),
    channelModes: formatJson(settings.channel_modes),
    enabledFilters: formatCsv(settings.rtk.enabled_filters),
    disabledFilters: formatCsv(settings.rtk.disabled_filters),
    compressRoles: formatCsv(settings.caveman.compress_roles),
    skipRules: formatCsv(settings.caveman.skip_rules),
    preservePatterns: formatCsv(settings.caveman.preserve_patterns),
    enabledLanguagePacks: formatCsv(settings.caveman.enabled_language_packs),
    stackedPipeline: formatJson(settings.stacked_pipeline),
  }
}

function normalizeSettings(
  settings: PromptCompressionSettings,
  draft: DraftTextFields
): PromptCompressionSettings {
  const merged = mergeCompressionSettings(settings)
  return {
    ...merged,
    auto_trigger_tokens: nonNegativeNumberValue(merged.auto_trigger_tokens),
    min_tokens: nonNegativeNumberValue(merged.min_tokens),
    allowed_groups: parseCsv(draft.allowedGroups),
    group_modes: parseModeMap(draft.groupModes, 'Group modes'),
    model_modes: parseModeMap(draft.modelModes, 'Model modes'),
    channel_modes: parseModeMap(draft.channelModes, 'Channel modes'),
    rtk: {
      ...merged.rtk,
      raw_output_max_bytes: positiveNumberValue(
        merged.rtk.raw_output_max_bytes,
        DEFAULT_SETTINGS.rtk.raw_output_max_bytes
      ),
      max_lines: positiveNumberValue(
        merged.rtk.max_lines,
        DEFAULT_SETTINGS.rtk.max_lines
      ),
      max_chars: positiveNumberValue(
        merged.rtk.max_chars,
        DEFAULT_SETTINGS.rtk.max_chars
      ),
      deduplicate_threshold: positiveNumberValue(
        merged.rtk.deduplicate_threshold,
        DEFAULT_SETTINGS.rtk.deduplicate_threshold
      ),
      enabled_filters: parseCsv(draft.enabledFilters),
      disabled_filters: parseCsv(draft.disabledFilters),
    },
    caveman: {
      ...merged.caveman,
      compress_roles: parseCsv(draft.compressRoles),
      skip_rules: parseCsv(draft.skipRules),
      preserve_patterns: parseCsv(draft.preservePatterns),
      enabled_language_packs: parseCsv(draft.enabledLanguagePacks),
      min_message_length: positiveNumberValue(
        merged.caveman.min_message_length,
        DEFAULT_SETTINGS.caveman.min_message_length
      ),
      language:
        merged.caveman.language.trim() || DEFAULT_SETTINGS.caveman.language,
    },
    aggressive: {
      ...merged.aggressive,
      thresholds: {
        full_summary: positiveNumberValue(
          merged.aggressive.thresholds.full_summary,
          DEFAULT_SETTINGS.aggressive.thresholds.full_summary
        ),
        moderate: positiveNumberValue(
          merged.aggressive.thresholds.moderate,
          DEFAULT_SETTINGS.aggressive.thresholds.moderate
        ),
        light: positiveNumberValue(
          merged.aggressive.thresholds.light,
          DEFAULT_SETTINGS.aggressive.thresholds.light
        ),
        verbatim: positiveNumberValue(
          merged.aggressive.thresholds.verbatim,
          DEFAULT_SETTINGS.aggressive.thresholds.verbatim
        ),
      },
      max_tokens_per_message: positiveNumberValue(
        merged.aggressive.max_tokens_per_message,
        DEFAULT_SETTINGS.aggressive.max_tokens_per_message
      ),
      min_savings_threshold: numberValue(
        merged.aggressive.min_savings_threshold,
        DEFAULT_SETTINGS.aggressive.min_savings_threshold
      ),
    },
    ultra: {
      ...merged.ultra,
      compression_rate: numberValue(
        merged.ultra.compression_rate,
        DEFAULT_SETTINGS.ultra.compression_rate
      ),
      min_score_threshold: numberValue(
        merged.ultra.min_score_threshold,
        DEFAULT_SETTINGS.ultra.min_score_threshold
      ),
      max_tokens_per_message: nonNegativeNumberValue(
        merged.ultra.max_tokens_per_message
      ),
      model_path: merged.ultra.model_path?.trim() ?? '',
    },
    stacked_pipeline: parseStackedPipeline(draft.stackedPipeline),
  }
}

function statValue(value: number | undefined, digits = 0): string {
  if (value === undefined || Number.isNaN(value)) return '0'
  return value.toFixed(digits)
}

function ModeSelect(props: {
  value: CompressionMode
  onChange: (value: CompressionMode) => void
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
        value !== null && props.onChange(value as CompressionMode)
      }
    >
      <SelectTrigger className='w-full'>
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

function IntensitySelect(props: {
  value: CavemanIntensity
  onChange: (value: CavemanIntensity) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={INTENSITY_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as CavemanIntensity)
      }
    >
      <SelectTrigger className='w-full'>
        <SelectValue placeholder={t('Select intensity')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {INTENSITY_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function RtkIntensitySelect(props: {
  value: RtkIntensity
  onChange: (value: RtkIntensity) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={RTK_INTENSITY_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as RtkIntensity)
      }
    >
      <SelectTrigger className='w-full'>
        <SelectValue placeholder={t('Select intensity')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {RTK_INTENSITY_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function RtkRawOutputSelect(props: {
  value: RtkRawOutputRetention
  onChange: (value: RtkRawOutputRetention) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={RTK_RAW_OUTPUT_OPTIONS.map((option) => ({
        value: option.value,
        label: t(option.label),
      }))}
      value={props.value}
      onValueChange={(value) =>
        value !== null && props.onChange(value as RtkRawOutputRetention)
      }
    >
      <SelectTrigger className='w-full'>
        <SelectValue placeholder={t('Select raw output retention')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {RTK_RAW_OUTPUT_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.label)}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function NumberField(props: {
  label: string
  value: number
  min?: number
  max?: number
  step?: number | string
  onChange: (value: number) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='text-sm font-medium'>{t(props.label)}</Label>
      <Input
        type='number'
        min={props.min ?? 0}
        max={props.max}
        step={props.step}
        value={props.value}
        onChange={(event) => props.onChange(Number(event.target.value))}
      />
    </div>
  )
}

function InputField(props: {
  label: string
  value: string
  placeholder?: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='text-sm font-medium'>{t(props.label)}</Label>
      <Input
        value={props.value}
        placeholder={props.placeholder ? t(props.placeholder) : undefined}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </div>
  )
}

function TextareaField(props: {
  label: string
  value: string
  rows?: number
  placeholder?: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='text-sm font-medium'>{t(props.label)}</Label>
      <Textarea
        rows={props.rows ?? 4}
        value={props.value}
        placeholder={props.placeholder ? t(props.placeholder) : undefined}
        onChange={(event) => props.onChange(event.target.value)}
        className='font-mono text-xs'
      />
    </div>
  )
}

function StatsGrid({ stats }: { stats: CompressionStats }) {
  const { t } = useTranslation()
  const items = [
    ['Original tokens', statValue(stats.original_tokens)],
    ['Compressed tokens', statValue(stats.compressed_tokens)],
    ['Saved tokens', statValue(stats.compression_saved_tokens)],
    ['Savings percent', `${statValue(stats.savings_percent, 2)}%`],
    ['Duration', `${statValue(stats.duration_ms)} ms`],
    ['Preserved blocks', statValue(stats.preserved_block_count)],
    ['Redacted secrets', statValue(stats.redacted_secret_count)],
  ]

  return (
    <div className='grid min-w-0 gap-2 sm:grid-cols-2 lg:grid-cols-4'>
      {items.map(([label, value]) => (
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

function PreviewResult(props: {
  title: string
  source: string
  result: CompressionPreviewResponse | null
}) {
  const { t } = useTranslation()
  if (!props.result) return null

  return (
    <div className='min-w-0 space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <h4 className='text-sm font-semibold'>{t(props.title)}</h4>
        <Badge
          variant={props.result.compressed === true ? 'default' : 'secondary'}
        >
          {props.result.compressed === true ? t('Compressed') : t('Bypassed')}
        </Badge>
        {props.result.stats.bypass_reason ? (
          <Badge variant='outline'>{props.result.stats.bypass_reason}</Badge>
        ) : null}
      </div>
      <StatsGrid stats={props.result.stats} />
      <div className='grid min-w-0 gap-3 lg:grid-cols-2'>
        <div className='min-w-0 space-y-1.5'>
          <Label className='text-xs font-medium'>{t('Before')}</Label>
          <Textarea
            readOnly
            rows={8}
            value={props.source}
            className='text-xs'
          />
        </div>
        <div className='min-w-0 space-y-1.5'>
          <Label className='text-xs font-medium'>{t('After')}</Label>
          <Textarea
            readOnly
            rows={8}
            value={props.result.text ?? ''}
            className='text-xs'
          />
        </div>
      </div>
      {props.result.stats.rules_applied?.length ? (
        <div className='flex flex-wrap gap-1.5'>
          {props.result.stats.rules_applied.map((rule) => (
            <Badge key={rule} variant='outline'>
              {rule}
            </Badge>
          ))}
        </div>
      ) : null}
    </div>
  )
}

export function PromptCompressionSection() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [settings, setSettings] =
    useState<PromptCompressionSettings>(DEFAULT_SETTINGS)
  const [initialSettings, setInitialSettings] =
    useState<PromptCompressionSettings>(DEFAULT_SETTINGS)
  const [draft, setDraft] = useState<DraftTextFields>(
    draftFromSettings(DEFAULT_SETTINGS)
  )
  const [filters, setFilters] = useState<RtkFiltersResponse | null>(null)
  const [previewMode, setPreviewMode] = useState<CompressionMode>('stacked')
  const [previewText, setPreviewText] = useState(SAMPLE_PROMPT)
  const [previewResult, setPreviewResult] =
    useState<CompressionPreviewResponse | null>(null)
  const [rtkText, setRtkText] = useState(
    'go test ./service\nPASS\nok github.com/QuantumNous/new-api/service 1.23s\n'
  )
  const [rtkResult, setRtkResult] = useState<CompressionPreviewResponse | null>(
    null
  )

  const isActive = settings.enabled && !settings.global_kill_switch

  const loadSettings = async () => {
    setLoading(true)
    try {
      const [settingsRes, filtersRes] = await Promise.all([
        getCompressionSettings(),
        getRtkFilters(),
      ])
      if (settingsRes.success && settingsRes.data) {
        const next = mergeCompressionSettings(settingsRes.data)
        setSettings(next)
        setInitialSettings(next)
        setDraft(draftFromSettings(next))
      } else {
        toast.error(
          settingsRes.message || t('Failed to load compression settings')
        )
      }
      if (filtersRes.success && filtersRes.data) {
        setFilters(filtersRes.data)
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Failed to load compression settings')
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadSettings()
  }, [])

  const handleReset = () => {
    setSettings(initialSettings)
    setDraft(draftFromSettings(initialSettings))
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const payload = normalizeSettings(settings, draft)
      const res = await updateCompressionSettings(payload)
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to save compression settings'))
        return
      }
      const next = mergeCompressionSettings(res.data)
      setSettings(next)
      setInitialSettings(next)
      setDraft(draftFromSettings(next))
      toast.success(t('Compression settings saved.'))
    } catch (error) {
      const message =
        error instanceof SyntaxError
          ? t('Invalid compression JSON')
          : error instanceof Error
            ? error.message
            : t('Failed to save compression settings')
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  const runPreview = async () => {
    setTesting(true)
    try {
      const res = await previewCompression({
        mode: previewMode,
        text: previewText,
      })
      if (!res.success || !res.data) {
        toast.error(res.message || t('Compression preview failed'))
        return
      }
      setPreviewResult(res.data)
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Compression preview failed')
      toast.error(message)
    } finally {
      setTesting(false)
    }
  }

  const runRtkTest = async () => {
    setTesting(true)
    try {
      const res = await testRtkCompression(rtkText)
      if (!res.success || !res.data) {
        toast.error(res.message || t('RTK test failed'))
        return
      }
      setRtkResult(res.data)
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('RTK test failed')
      toast.error(message)
    } finally {
      setTesting(false)
    }
  }

  const filterGroups = useMemo(() => {
    const groups = new Map<string, string[]>()
    for (const filter of filters?.filters ?? []) {
      const items = groups.get(filter.category) ?? []
      items.push(filter.id)
      groups.set(filter.category, items)
    }
    return [...groups.entries()]
  }, [filters?.filters])

  const updateRtk = (patch: Partial<PromptCompressionSettings['rtk']>) => {
    setSettings((current) => ({
      ...current,
      rtk: { ...current.rtk, ...patch },
    }))
  }

  const updateCaveman = (
    patch: Partial<PromptCompressionSettings['caveman']>
  ) => {
    setSettings((current) => ({
      ...current,
      caveman: { ...current.caveman, ...patch },
    }))
  }

  const updateAggressive = (
    patch: Partial<PromptCompressionSettings['aggressive']>
  ) => {
    setSettings((current) => ({
      ...current,
      aggressive: { ...current.aggressive, ...patch },
    }))
  }

  const updateAggressiveThresholds = (
    patch: Partial<PromptCompressionSettings['aggressive']['thresholds']>
  ) => {
    setSettings((current) => ({
      ...current,
      aggressive: {
        ...current.aggressive,
        thresholds: { ...current.aggressive.thresholds, ...patch },
      },
    }))
  }

  const updateToolStrategies = (
    patch: Partial<PromptCompressionSettings['aggressive']['tool_strategies']>
  ) => {
    setSettings((current) => ({
      ...current,
      aggressive: {
        ...current.aggressive,
        tool_strategies: {
          ...current.aggressive.tool_strategies,
          ...patch,
        },
      },
    }))
  }

  const updateUltra = (patch: Partial<PromptCompressionSettings['ultra']>) => {
    setSettings((current) => ({
      ...current,
      ultra: { ...current.ultra, ...patch },
    }))
  }

  if (loading) {
    return (
      <SettingsSection title={t('Prompt Compression')}>
        <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
          {t('Loading compression settings...')}
        </div>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Prompt Compression')}>
      <SettingsPageTitleStatusPortal>
        <Badge variant={isActive ? 'default' : 'secondary'}>
          {isActive ? t('Enabled') : t('Disabled')}
        </Badge>
      </SettingsPageTitleStatusPortal>
      <SettingsPageFormActions
        onSave={handleSave}
        onReset={handleReset}
        isSaving={saving}
        isSaveDisabled={loading}
      />

      <SettingsForm>
        <SettingsControlGroup>
          <SettingsSwitchField
            label={t('Enable compression')}
            checked={settings.enabled}
            onCheckedChange={(enabled) =>
              setSettings((current) => ({ ...current, enabled }))
            }
          />
          <SettingsSwitchField
            label={t('Global kill switch')}
            checked={settings.global_kill_switch}
            onCheckedChange={(global_kill_switch) =>
              setSettings((current) => ({ ...current, global_kill_switch }))
            }
          />
          <SettingsSwitchField
            label={t('Preserve system prompt')}
            checked={settings.preserve_system_prompt}
            onCheckedChange={(preserve_system_prompt) =>
              setSettings((current) => ({
                ...current,
                preserve_system_prompt,
              }))
            }
          />
        </SettingsControlGroup>

        <SettingsFormGrid>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>{t('Default mode')}</Label>
            <div className='mt-1.5'>
              <ModeSelect
                value={settings.default_mode}
                onChange={(default_mode) =>
                  setSettings((current) => ({ ...current, default_mode }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <SettingsFormGridItem>
            <Label className='text-sm font-medium'>
              {t('Auto trigger mode')}
            </Label>
            <div className='mt-1.5'>
              <ModeSelect
                value={settings.auto_trigger_mode}
                onChange={(auto_trigger_mode) =>
                  setSettings((current) => ({ ...current, auto_trigger_mode }))
                }
              />
            </div>
          </SettingsFormGridItem>
          <NumberField
            label='Auto trigger tokens'
            value={settings.auto_trigger_tokens}
            onChange={(auto_trigger_tokens) =>
              setSettings((current) => ({
                ...current,
                auto_trigger_tokens,
              }))
            }
          />
          <NumberField
            label='Minimum tokens'
            value={settings.min_tokens}
            onChange={(min_tokens) =>
              setSettings((current) => ({ ...current, min_tokens }))
            }
          />
        </SettingsFormGrid>

        <TextareaField
          label='Allowed groups'
          value={draft.allowedGroups}
          placeholder='proxy-test'
          onChange={(allowedGroups) =>
            setDraft((current) => ({ ...current, allowedGroups }))
          }
        />

        <SettingsFormGrid>
          <TextareaField
            label='Group modes JSON'
            value={draft.groupModes}
            rows={6}
            placeholder='{"proxy-test":"stacked"}'
            onChange={(groupModes) =>
              setDraft((current) => ({ ...current, groupModes }))
            }
          />
          <TextareaField
            label='Model modes JSON'
            value={draft.modelModes}
            rows={6}
            placeholder='{"glart-coder":"rtk"}'
            onChange={(modelModes) =>
              setDraft((current) => ({ ...current, modelModes }))
            }
          />
          <TextareaField
            label='Channel modes JSON'
            value={draft.channelModes}
            rows={6}
            placeholder='{"12":"standard"}'
            onChange={(channelModes) =>
              setDraft((current) => ({ ...current, channelModes }))
            }
          />
        </SettingsFormGrid>

        <Separator />

        <div className='space-y-4'>
          <Label className='text-sm font-semibold'>{t('RTK settings')}</Label>
          <SettingsFormGrid>
            <SettingsFormGridItem>
              <Label className='text-sm font-medium'>
                {t('RTK intensity')}
              </Label>
              <div className='mt-1.5'>
                <RtkIntensitySelect
                  value={settings.rtk.intensity}
                  onChange={(intensity) => updateRtk({ intensity })}
                />
              </div>
            </SettingsFormGridItem>
            <SettingsFormGridItem>
              <Label className='text-sm font-medium'>
                {t('RTK raw output retention')}
              </Label>
              <div className='mt-1.5'>
                <RtkRawOutputSelect
                  value={settings.rtk.raw_output_retention}
                  onChange={(raw_output_retention) =>
                    updateRtk({ raw_output_retention })
                  }
                />
              </div>
            </SettingsFormGridItem>
            <NumberField
              label='RTK raw output max bytes'
              value={settings.rtk.raw_output_max_bytes}
              min={1024}
              onChange={(raw_output_max_bytes) =>
                updateRtk({ raw_output_max_bytes })
              }
            />
            <NumberField
              label='RTK max lines'
              value={settings.rtk.max_lines}
              min={1}
              onChange={(max_lines) => updateRtk({ max_lines })}
            />
            <NumberField
              label='RTK max chars'
              value={settings.rtk.max_chars}
              min={1}
              onChange={(max_chars) => updateRtk({ max_chars })}
            />
            <NumberField
              label='RTK deduplicate threshold'
              value={settings.rtk.deduplicate_threshold}
              min={2}
              onChange={(deduplicate_threshold) =>
                updateRtk({ deduplicate_threshold })
              }
            />
            <TextareaField
              label='RTK enabled filters'
              value={draft.enabledFilters}
              placeholder='git-diff, go-test'
              onChange={(enabledFilters) =>
                setDraft((current) => ({ ...current, enabledFilters }))
              }
            />
            <TextareaField
              label='RTK disabled filters'
              value={draft.disabledFilters}
              placeholder='docker-logs'
              onChange={(disabledFilters) =>
                setDraft((current) => ({ ...current, disabledFilters }))
              }
            />
          </SettingsFormGrid>
          <SettingsControlGroup>
            <SettingsSwitchField
              label={t('RTK custom filters')}
              checked={settings.rtk.custom_filters_enabled}
              onCheckedChange={(custom_filters_enabled) =>
                updateRtk({ custom_filters_enabled })
              }
            />
            <SettingsSwitchField
              label={t('RTK trust project filters')}
              checked={settings.rtk.trust_project_filters}
              onCheckedChange={(trust_project_filters) =>
                updateRtk({ trust_project_filters })
              }
            />
            <SettingsSwitchField
              label={t('RTK apply to tool results')}
              checked={settings.rtk.apply_to_tool_results}
              onCheckedChange={(apply_to_tool_results) =>
                updateRtk({ apply_to_tool_results })
              }
            />
            <SettingsSwitchField
              label={t('RTK apply to assistant messages')}
              checked={settings.rtk.apply_to_assistant_messages}
              onCheckedChange={(apply_to_assistant_messages) =>
                updateRtk({ apply_to_assistant_messages })
              }
            />
            <SettingsSwitchField
              label={t('RTK apply to code blocks')}
              checked={settings.rtk.apply_to_code_blocks}
              onCheckedChange={(apply_to_code_blocks) =>
                updateRtk({ apply_to_code_blocks })
              }
            />
          </SettingsControlGroup>
        </div>

        <Separator />

        <div className='space-y-4'>
          <Label className='text-sm font-semibold'>
            {t('Caveman settings')}
          </Label>
          <SettingsFormGrid>
            <SettingsFormGridItem>
              <Label className='text-sm font-medium'>
                {t('Caveman intensity')}
              </Label>
              <div className='mt-1.5'>
                <IntensitySelect
                  value={settings.caveman.intensity}
                  onChange={(intensity) => updateCaveman({ intensity })}
                />
              </div>
            </SettingsFormGridItem>
            <NumberField
              label='Caveman minimum message length'
              value={settings.caveman.min_message_length}
              min={1}
              onChange={(min_message_length) =>
                updateCaveman({ min_message_length })
              }
            />
            <InputField
              label='Caveman language'
              value={settings.caveman.language}
              placeholder='en'
              onChange={(language) => updateCaveman({ language })}
            />
            <TextareaField
              label='Caveman roles'
              value={draft.compressRoles}
              placeholder='user'
              onChange={(compressRoles) =>
                setDraft((current) => ({ ...current, compressRoles }))
              }
            />
            <TextareaField
              label='Caveman skip rules'
              value={draft.skipRules}
              placeholder='rule-id'
              onChange={(skipRules) =>
                setDraft((current) => ({ ...current, skipRules }))
              }
            />
            <TextareaField
              label='Caveman preserve patterns'
              value={draft.preservePatterns}
              placeholder='^ERROR_[A-Z0-9_]+$'
              onChange={(preservePatterns) =>
                setDraft((current) => ({ ...current, preservePatterns }))
              }
            />
            <TextareaField
              label='Caveman enabled language packs'
              value={draft.enabledLanguagePacks}
              placeholder='en, zh'
              onChange={(enabledLanguagePacks) =>
                setDraft((current) => ({ ...current, enabledLanguagePacks }))
              }
            />
          </SettingsFormGrid>
          <SettingsControlGroup>
            <SettingsSwitchField
              label={t('Caveman auto detect language')}
              checked={settings.caveman.auto_detect_language}
              onCheckedChange={(auto_detect_language) =>
                updateCaveman({ auto_detect_language })
              }
            />
          </SettingsControlGroup>
        </div>

        <Separator />

        <div className='space-y-4'>
          <Label className='text-sm font-semibold'>
            {t('Aggressive settings')}
          </Label>
          <SettingsControlGroup>
            <SettingsSwitchField
              label={t('Aggressive summarizer')}
              checked={settings.aggressive.summarizer_enabled}
              onCheckedChange={(summarizer_enabled) =>
                updateAggressive({ summarizer_enabled })
              }
            />
            <SettingsSwitchField
              label={t('Aggressive file content strategy')}
              checked={settings.aggressive.tool_strategies.file_content}
              onCheckedChange={(file_content) =>
                updateToolStrategies({ file_content })
              }
            />
            <SettingsSwitchField
              label={t('Aggressive grep search strategy')}
              checked={settings.aggressive.tool_strategies.grep_search}
              onCheckedChange={(grep_search) =>
                updateToolStrategies({ grep_search })
              }
            />
            <SettingsSwitchField
              label={t('Aggressive shell output strategy')}
              checked={settings.aggressive.tool_strategies.shell_output}
              onCheckedChange={(shell_output) =>
                updateToolStrategies({ shell_output })
              }
            />
            <SettingsSwitchField
              label={t('Aggressive JSON strategy')}
              checked={settings.aggressive.tool_strategies.json}
              onCheckedChange={(json) => updateToolStrategies({ json })}
            />
            <SettingsSwitchField
              label={t('Aggressive error message strategy')}
              checked={settings.aggressive.tool_strategies.error_message}
              onCheckedChange={(error_message) =>
                updateToolStrategies({ error_message })
              }
            />
          </SettingsControlGroup>
          <SettingsFormGrid>
            <NumberField
              label='Aggressive full summary threshold'
              value={settings.aggressive.thresholds.full_summary}
              min={1}
              onChange={(full_summary) =>
                updateAggressiveThresholds({ full_summary })
              }
            />
            <NumberField
              label='Aggressive moderate threshold'
              value={settings.aggressive.thresholds.moderate}
              min={1}
              onChange={(moderate) => updateAggressiveThresholds({ moderate })}
            />
            <NumberField
              label='Aggressive light threshold'
              value={settings.aggressive.thresholds.light}
              min={1}
              onChange={(light) => updateAggressiveThresholds({ light })}
            />
            <NumberField
              label='Aggressive verbatim threshold'
              value={settings.aggressive.thresholds.verbatim}
              min={1}
              onChange={(verbatim) => updateAggressiveThresholds({ verbatim })}
            />
            <NumberField
              label='Aggressive max tokens per message'
              value={settings.aggressive.max_tokens_per_message}
              min={1}
              onChange={(max_tokens_per_message) =>
                updateAggressive({ max_tokens_per_message })
              }
            />
            <NumberField
              label='Aggressive minimum savings threshold'
              value={settings.aggressive.min_savings_threshold}
              min={0}
              max={1}
              step='0.01'
              onChange={(min_savings_threshold) =>
                updateAggressive({ min_savings_threshold })
              }
            />
          </SettingsFormGrid>
        </div>

        <Separator />

        <div className='space-y-4'>
          <Label className='text-sm font-semibold'>{t('Ultra settings')}</Label>
          <SettingsControlGroup>
            <SettingsSwitchField
              label={t('Ultra fallback to aggressive')}
              checked={settings.ultra.slm_fallback_to_aggressive}
              onCheckedChange={(slm_fallback_to_aggressive) =>
                updateUltra({ slm_fallback_to_aggressive })
              }
            />
          </SettingsControlGroup>
          <SettingsFormGrid>
            <NumberField
              label='Ultra compression rate'
              value={settings.ultra.compression_rate}
              min={0}
              max={1}
              step='0.01'
              onChange={(compression_rate) => updateUltra({ compression_rate })}
            />
            <NumberField
              label='Ultra minimum score threshold'
              value={settings.ultra.min_score_threshold}
              min={0}
              max={1}
              step='0.01'
              onChange={(min_score_threshold) =>
                updateUltra({ min_score_threshold })
              }
            />
            <NumberField
              label='Ultra max tokens per message'
              value={settings.ultra.max_tokens_per_message}
              min={0}
              onChange={(max_tokens_per_message) =>
                updateUltra({ max_tokens_per_message })
              }
            />
            <InputField
              label='Ultra model path'
              value={settings.ultra.model_path ?? ''}
              placeholder='/opt/glart-api/runtime/models/slm.gguf'
              onChange={(model_path) => updateUltra({ model_path })}
            />
          </SettingsFormGrid>
        </div>

        <Separator />

        <TextareaField
          label='Stacked pipeline JSON'
          value={draft.stackedPipeline}
          rows={7}
          placeholder='[{"engine":"rtk","intensity":"standard"},{"engine":"caveman","intensity":"full"}]'
          onChange={(stackedPipeline) =>
            setDraft((current) => ({ ...current, stackedPipeline }))
          }
        />
      </SettingsForm>

      {filterGroups.length > 0 ? (
        <div className='space-y-2'>
          <Label className='text-sm font-medium'>{t('RTK filters')}</Label>
          <div className='flex flex-wrap gap-2'>
            {filterGroups.map(([category, items]) => (
              <Badge key={category} variant='outline'>
                {category}: {items.join(', ')}
              </Badge>
            ))}
          </div>
        </div>
      ) : null}

      <Separator />

      <div className='grid min-w-0 gap-4 xl:grid-cols-2'>
        <div className='min-w-0 space-y-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Label className='text-sm font-semibold'>
              {t('Compression preview')}
            </Label>
            <div className='flex items-center gap-2'>
              <ModeSelect value={previewMode} onChange={setPreviewMode} />
              <Button size='sm' onClick={runPreview} disabled={testing}>
                <WandSparklesIcon data-icon='inline-start' />
                <span>{t('Preview')}</span>
              </Button>
            </div>
          </div>
          <Textarea
            rows={9}
            value={previewText}
            onChange={(event) => setPreviewText(event.target.value)}
          />
          <PreviewResult
            title='Preview result'
            source={previewText}
            result={previewResult}
          />
        </div>

        <div className='min-w-0 space-y-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Label className='text-sm font-semibold'>{t('RTK test')}</Label>
            <div className='flex items-center gap-2'>
              <Button
                size='sm'
                variant='outline'
                onClick={() => setRtkText('')}
                disabled={testing}
              >
                <RotateCcwIcon data-icon='inline-start' />
                <span>{t('Clear')}</span>
              </Button>
              <Button size='sm' onClick={runRtkTest} disabled={testing}>
                <FlaskConicalIcon data-icon='inline-start' />
                <span>{t('Run test')}</span>
              </Button>
            </div>
          </div>
          <Textarea
            rows={9}
            value={rtkText}
            onChange={(event) => setRtkText(event.target.value)}
            className='font-mono text-xs'
          />
          <PreviewResult
            title='RTK result'
            source={rtkText}
            result={rtkResult}
          />
        </div>
      </div>

      <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
        <RefreshCcwIcon className='size-3.5' />
        <span>{settings.attribution || filters?.attribution}</span>
      </div>
    </SettingsSection>
  )
}
