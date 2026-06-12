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
import { useEffect, useMemo, useState, type ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  BarChart3Icon,
  CheckCircle2Icon,
  EyeIcon,
  InfoIcon,
  RefreshCcwIcon,
  Settings2Icon,
  SparklesIcon,
  ZapIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import {
  getCompressionSettings,
  getProfitAnalytics,
  getProfitSettings,
  getSystemGroups,
  updateCompressionSettings,
  updateProfitSettings,
  type CompressionMode,
  type ProfitAnalytics,
  type ProfitOutputPolicy,
  type ProfitSettings,
  type PromptCompressionSettings,
} from '../api'
import {
  SettingsPageTitleStatusPortal,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

type ManagedMode = 'off' | 'observe' | 'managed'
type RecommendationAction =
  | 'observe'
  | 'compression'
  | 'output_cap'
  | 'advanced_profit'
  | 'advanced_compression'
  | 'none'

type Recommendation = {
  id: string
  title: string
  body: string
  reason: string
  action: RecommendationAction
  confidence: 'low' | 'medium' | 'high'
}

type PendingAction =
  | { type: 'mode'; mode: ManagedMode }
  | { type: 'recommendation'; recommendation: Recommendation }

const FALLBACK_GROUPS = ['default', 'proxy-test']

const MODE_META: Record<
  ManagedMode,
  {
    label: string
    description: string
    icon: ComponentType<{ className?: string }>
  }
> = {
  off: {
    label: '关闭',
    description: '不启用收益优化和压缩，只保留普通日志。',
    icon: AlertTriangleIcon,
  },
  observe: {
    label: '只观察',
    description: '记录收益、压缩、风险和推荐，不改写请求。',
    icon: EyeIcon,
  },
  managed: {
    label: '托管推荐',
    description: '启用低风险观察策略，并对该分组开启 stacked 压缩观察。',
    icon: SparklesIcon,
  },
}

function formatUSD(value?: number | null) {
  if (value === null || value === undefined) return '未知'
  return `$${value.toFixed(value >= 1 ? 2 : 4)}`
}

function formatPct(value?: number | null) {
  if (value === null || value === undefined) return '未知'
  return `${value.toFixed(1)}%`
}

function formatInteger(value?: number | null) {
  return new Intl.NumberFormat('zh-CN').format(value ?? 0)
}

function uniqueGroups(groups?: string[]) {
  return Array.from(
    new Set([...(groups ?? []), ...FALLBACK_GROUPS].filter(Boolean))
  )
}

function addGroup(groups: string[] | undefined, group: string) {
  return Array.from(new Set([...(groups ?? []), group].filter(Boolean)))
}

function removeGroup(groups: string[] | undefined, group: string) {
  return (groups ?? []).filter((item) => item !== group)
}

function safePolicyID(group: string, suffix: string) {
  return `managed-${group.replace(/[^a-zA-Z0-9_-]/g, '-')}-${suffix}`
}

function deriveMode(
  group: string,
  profit?: ProfitSettings | null,
  compression?: PromptCompressionSettings | null
): ManagedMode {
  const observed =
    Boolean(profit?.enabled) &&
    !profit?.global_kill_switch &&
    Boolean(profit?.observe_groups?.includes(group))
  const compressionMode = compression?.group_modes?.[group]
  const hasGroupPolicy =
    Boolean(
      profit?.output_policies?.some(
        (policy) => policy.enabled && policy.group === group
      )
    ) ||
    Boolean(
      profit?.long_context_policies?.some(
        (policy) => policy.enabled && policy.group === group
      )
    ) ||
    Boolean(
      profit?.response_cache_rules?.some(
        (rule) => rule.enabled && rule.group === group
      )
    )
  if (
    observed &&
    (compressionMode === 'stacked' || hasGroupPolicy)
  ) {
    return 'managed'
  }
  if (observed) return 'observe'
  return 'off'
}

function buildOutputPolicy(
  group: string,
  analytics?: ProfitAnalytics | null
): ProfitOutputPolicy {
  const defaultMax =
    analytics?.output_policy_recommended_default_max_tokens ||
    analytics?.output_policy_completion_p95_tokens ||
    0
  const hardMax =
    analytics?.output_policy_recommended_hard_max_tokens ||
    analytics?.output_policy_completion_p99_tokens ||
    0
  const policy: ProfitOutputPolicy = {
    id: safePolicyID(group, 'output-observe'),
    name: `${group} 输出长度观察`,
    enabled: true,
    priority: 10,
    group,
    mode: 'observe',
    rewrite_over_limit: false,
    notes: '收益托管页自动创建，仅观察，不强制截断。',
  }
  if (defaultMax > 0) {
    policy.default_max_tokens = defaultMax
  }
  if (hardMax > 0) {
    policy.hard_max_tokens = hardMax
  }
  return policy
}

function upsertOutputPolicy(
  policies: ProfitOutputPolicy[] | undefined,
  policy: ProfitOutputPolicy
) {
  const items = [...(policies ?? [])]
  const index = items.findIndex((item) => item.id === policy.id)
  if (index >= 0) {
    items[index] = { ...items[index], ...policy }
    return items
  }
  return [policy, ...items]
}

function buildProfitForMode(
  settings: ProfitSettings,
  group: string,
  mode: ManagedMode,
  analytics?: ProfitAnalytics | null
): ProfitSettings {
  if (mode === 'off') {
    return {
      ...settings,
      observe_groups: removeGroup(settings.observe_groups, group),
    }
  }
  const base: ProfitSettings = {
    ...settings,
    enabled: true,
    observe_only: true,
    global_kill_switch: false,
    observe_groups: addGroup(settings.observe_groups, group),
  }
  if (mode === 'observe') return base
  return {
    ...base,
    cost_routing_mode: 'observe',
    long_context_mode: 'observe',
    output_cap_mode: 'observe',
    retry_budget_mode: 'observe',
    risk_enforcement: 'alert',
    output_policies: upsertOutputPolicy(
      base.output_policies,
      buildOutputPolicy(group, analytics)
    ),
  }
}

function buildCompressionForMode(
  settings: PromptCompressionSettings,
  group: string,
  mode: ManagedMode
): PromptCompressionSettings {
  const groupModes = { ...(settings.group_modes ?? {}) }
  if (mode === 'managed') {
    groupModes[group] = 'stacked' as CompressionMode
    return {
      ...settings,
      enabled: true,
      global_kill_switch: false,
      preserve_system_prompt: true,
      allowed_groups: addGroup(settings.allowed_groups, group),
      group_modes: groupModes,
    }
  }
  groupModes[group] = 'off' as CompressionMode
  return {
    ...settings,
    allowed_groups: removeGroup(settings.allowed_groups, group),
    group_modes: groupModes,
  }
}

function buildRecommendations(
  group: string,
  analytics?: ProfitAnalytics | null,
  currentMode?: ManagedMode
): Recommendation[] {
  if (!analytics || analytics.request_count === 0) {
    return [
      {
        id: 'no-samples',
        title: '建议继续观察，样本不足。',
        body: `当前 ${group} 分组还没有足够的真实请求样本。`,
        reason: '没有样本时不自动开启请求改写。',
        action: currentMode === 'off' ? 'observe' : 'none',
        confidence: 'low',
      },
    ]
  }

  const items: Recommendation[] = []
  if (analytics.missing_cost_profile_count > 0) {
    items.push({
      id: 'cost-profile',
      title: '建议补充成本档案，部分请求无法计算毛利。',
      body: `${analytics.missing_cost_profile_count} 条请求缺少成本档案，毛利判断会偏保守。`,
      reason: '成本档案缺失时只能观察，不能可靠做毛利优先路由。',
      action: 'advanced_profit',
      confidence: 'high',
    })
  }
  if (currentMode !== 'managed' && analytics.scanned_events >= 5) {
    items.push({
      id: 'compression',
      title: '建议对该分组启用 stacked 压缩观察。',
      body: '压缩节省归平台，用户侧仍按平台计费用消耗结算。',
      reason: '该动作只对所选分组生效，并保留 system prompt 保护。',
      action: 'compression',
      confidence: 'medium',
    })
  }
  if ((analytics.output_policy_completion_sample_count ?? 0) < 20) {
    items.push({
      id: 'output-samples',
      title: '建议继续观察输出长度，样本不足。',
      body: '达到 20 条以上输出样本后，再判断是否需要输出上限观察。',
      reason: '样本不足时不建议直接限制输出。',
      action: 'none',
      confidence: 'low',
    })
  } else if (
    analytics.output_policy_recommended_default_max_tokens > 0 ||
    analytics.output_policy_completion_p99_tokens > 4096
  ) {
    items.push({
      id: 'output-cap',
      title: '建议对低毛利模型启用输出上限观察。',
      body: `当前 P99 输出约 ${formatInteger(analytics.output_policy_completion_p99_tokens)} tokens。`,
      reason: '第一步仅观察，不会截断用户输出。',
      action: 'output_cap',
      confidence: 'medium',
    })
  }
  if ((analytics.long_context_suggested_extra_revenue_usd ?? 0) > 0) {
    items.push({
      id: 'long-context',
      title: '建议检查长上下文溢价。',
      body: `观察到可评估的长上下文增收空间：${formatUSD(analytics.long_context_suggested_extra_revenue_usd)}。`,
      reason: '长上下文成本波动大，建议在高级设置确认阶梯。',
      action: 'advanced_profit',
      confidence: 'medium',
    })
  }
  if ((analytics.retry_cost_usd ?? 0) > 0) {
    items.push({
      id: 'retry-cost',
      title: '建议检查重试成本。',
      body: `近期重试估算成本为 ${formatUSD(analytics.retry_cost_usd)}。`,
      reason: '重试会增加平台成本，应先观察低毛利请求。',
      action: 'advanced_profit',
      confidence: 'medium',
    })
  }
  if ((analytics.profit_risk_loss_making_count ?? 0) > 0) {
    items.push({
      id: 'loss-making',
      title: '建议检查亏损请求。',
      body: `${analytics.profit_risk_loss_making_count} 条请求被标记为亏损或低毛利风险。`,
      reason: '需要结合成本档案、模型定价和长上下文溢价一起判断。',
      action: 'advanced_profit',
      confidence: 'high',
    })
  }

  if (items.length === 0) {
    items.push({
      id: 'healthy',
      title: '当前没有需要立即处理的高置信建议。',
      body: '继续保持只观察即可，系统会随真实请求更新判断。',
      reason: '未发现明显亏损、长输出或成本档案缺失。',
      action: 'none',
      confidence: 'medium',
    })
  }
  return items.slice(0, 5)
}

export function ProfitManagedSection() {
  const [groups, setGroups] = useState<string[]>(FALLBACK_GROUPS)
  const [selectedGroup, setSelectedGroup] = useState('proxy-test')
  const [profitSettings, setProfitSettings] = useState<ProfitSettings | null>(
    null
  )
  const [compressionSettings, setCompressionSettings] =
    useState<PromptCompressionSettings | null>(null)
  const [analytics, setAnalytics] = useState<ProfitAnalytics | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [detailsID, setDetailsID] = useState<string | null>(null)
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null)
  const [ignoredIDs, setIgnoredIDs] = useState<string[]>([])

  const loadBase = async () => {
    setIsLoading(true)
    try {
      const [groupsRes, profitRes, compressionRes] = await Promise.all([
        getSystemGroups(),
        getProfitSettings(),
        getCompressionSettings(),
      ])
      const nextGroups = uniqueGroups(groupsRes.data)
      setGroups(nextGroups)
      if (!nextGroups.includes(selectedGroup)) {
        setSelectedGroup(
          nextGroups.includes('proxy-test')
            ? 'proxy-test'
            : (nextGroups[0] ?? 'default')
        )
      }
      if (profitRes.data) setProfitSettings(profitRes.data)
      if (compressionRes.data) setCompressionSettings(compressionRes.data)
    } catch (error) {
      toast.error('加载收益托管配置失败')
    } finally {
      setIsLoading(false)
    }
  }

  const refreshAnalytics = async () => {
    setIsRefreshing(true)
    try {
      const res = await getProfitAnalytics({ group: selectedGroup })
      setAnalytics(res.data ?? null)
    } catch (error) {
      toast.error('加载分组收益判断失败')
    } finally {
      setIsRefreshing(false)
    }
  }

  useEffect(() => {
    void loadBase()
  }, [])

  useEffect(() => {
    void refreshAnalytics()
  }, [selectedGroup])

  const currentMode = useMemo(
    () => deriveMode(selectedGroup, profitSettings, compressionSettings),
    [selectedGroup, profitSettings, compressionSettings]
  )

  const recommendations = useMemo(
    () =>
      buildRecommendations(selectedGroup, analytics, currentMode).filter(
        (item) => !ignoredIDs.includes(item.id)
      ),
    [analytics, currentMode, ignoredIDs, selectedGroup]
  )

  const judgmentItems = useMemo(
    () => [
      {
        label: '毛利',
        value:
          analytics?.gross_margin_pct === null ||
          analytics?.gross_margin_pct === undefined
            ? '未知'
            : formatPct(analytics.gross_margin_pct),
        tone:
          (analytics?.gross_margin_pct ?? 0) < 0
            ? 'text-destructive'
            : 'text-foreground',
      },
      {
        label: '亏损请求',
        value: formatInteger(analytics?.profit_risk_loss_making_count),
        tone:
          (analytics?.profit_risk_loss_making_count ?? 0) > 0
            ? 'text-destructive'
            : 'text-foreground',
      },
      {
        label: '压缩节省',
        value: `${formatInteger(analytics?.compression_saved_tokens)} tokens`,
        tone: 'text-foreground',
      },
      {
        label: '输出长尾',
        value: `P99 ${formatInteger(analytics?.output_policy_completion_p99_tokens)} tokens`,
        tone:
          (analytics?.output_policy_completion_p99_tokens ?? 0) > 4096
            ? 'text-amber-600'
            : 'text-foreground',
      },
      {
        label: '重试成本',
        value: formatUSD(analytics?.retry_cost_usd),
        tone:
          (analytics?.retry_cost_usd ?? 0) > 0
            ? 'text-amber-600'
            : 'text-foreground',
      },
      {
        label: '缺失成本档案',
        value: formatInteger(analytics?.missing_cost_profile_count),
        tone:
          (analytics?.missing_cost_profile_count ?? 0) > 0
            ? 'text-amber-600'
            : 'text-foreground',
      },
    ],
    [analytics]
  )

  const applyMode = async (mode: ManagedMode) => {
    if (!profitSettings || !compressionSettings) return
    setIsSaving(true)
    try {
      const nextProfit = buildProfitForMode(
        profitSettings,
        selectedGroup,
        mode,
        analytics
      )
      const nextCompression = buildCompressionForMode(
        compressionSettings,
        selectedGroup,
        mode
      )
      const [profitRes, compressionRes] = await Promise.all([
        updateProfitSettings(nextProfit),
        updateCompressionSettings(nextCompression),
      ])
      if (!profitRes.success || !compressionRes.success) {
        throw new Error(profitRes.message || compressionRes.message)
      }
      setProfitSettings(profitRes.data ?? nextProfit)
      setCompressionSettings(compressionRes.data ?? nextCompression)
      toast.success(`已应用 ${MODE_META[mode].label}`)
      void refreshAnalytics()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '应用失败')
    } finally {
      setIsSaving(false)
      setPendingAction(null)
    }
  }

  const applyRecommendation = async (recommendation: Recommendation) => {
    if (recommendation.action === 'observe') {
      await applyMode('observe')
      return
    }
    if (recommendation.action === 'compression') {
      await applyMode('managed')
      return
    }
    if (recommendation.action === 'output_cap') {
      await applyMode('managed')
      return
    }
    setPendingAction(null)
  }

  const confirmPendingAction = () => {
    if (!pendingAction) return
    if (pendingAction.type === 'mode') {
      void applyMode(pendingAction.mode)
      return
    }
    void applyRecommendation(pendingAction.recommendation)
  }

  return (
    <SettingsSection title='收益托管'>
      <SettingsPageTitleStatusPortal>
        <Badge variant='secondary' className='ml-2'>
          root-only
        </Badge>
      </SettingsPageTitleStatusPortal>

      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <h3 className='text-base font-semibold'>收益托管</h3>
          <p className='text-muted-foreground text-sm'>
            按分组查看系统判断，一键应用低风险托管建议。
          </p>
        </div>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => void refreshAnalytics()}
          disabled={isRefreshing}
        >
          <RefreshCcwIcon data-icon='inline-start' />
          <span>{isRefreshing ? '刷新中' : '刷新判断'}</span>
        </Button>
      </div>

      <div className='grid min-w-0 gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'>
        <section className='rounded-lg border bg-card p-4'>
          <div className='flex items-center gap-2'>
            <BarChart3Icon className='size-4 text-muted-foreground' />
            <h4 className='text-sm font-semibold'>分组选择</h4>
          </div>
          <div className='mt-4'>
            <Select value={selectedGroup} onValueChange={setSelectedGroup}>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='选择分组' />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {groups.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <div className='mt-3 text-xs text-muted-foreground'>
            当前只影响所选分组：{selectedGroup}
          </div>
        </section>

        <section className='rounded-lg border bg-card p-4'>
          <div className='flex items-center gap-2'>
            <ZapIcon className='size-4 text-muted-foreground' />
            <h4 className='text-sm font-semibold'>当前状态</h4>
          </div>
          <div className='mt-4 grid min-w-0 gap-2 sm:grid-cols-3'>
            {(Object.keys(MODE_META) as ManagedMode[]).map((mode) => {
              const meta = MODE_META[mode]
              const Icon = meta.icon
              const active = currentMode === mode
              return (
                <button
                  key={mode}
                  type='button'
                  className={cn(
                    'min-w-0 rounded-lg border p-3 text-left transition-colors',
                    active
                      ? 'border-primary bg-primary/5'
                      : 'hover:bg-muted/40'
                  )}
                  disabled={isLoading || isSaving}
                  onClick={() => setPendingAction({ type: 'mode', mode })}
                >
                  <span className='flex items-center gap-2 text-sm font-medium'>
                    <Icon className='size-4' />
                    {meta.label}
                  </span>
                  <span className='mt-1 block text-xs leading-5 text-muted-foreground'>
                    {meta.description}
                  </span>
                </button>
              )
            })}
          </div>
        </section>
      </div>

      <section className='rounded-lg border bg-card p-4'>
        <div className='flex items-center justify-between gap-2'>
          <div className='flex items-center gap-2'>
            <InfoIcon className='size-4 text-muted-foreground' />
            <h4 className='text-sm font-semibold'>系统判断</h4>
          </div>
          <Badge variant='outline'>
            {analytics ? `${formatInteger(analytics.request_count)} 请求` : '加载中'}
          </Badge>
        </div>
        <div className='mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
          {judgmentItems.map((item) => (
            <div key={item.label} className='rounded-lg border px-3 py-2'>
              <div className='text-xs text-muted-foreground'>{item.label}</div>
              <div className={cn('mt-1 text-sm font-semibold', item.tone)}>
                {item.value}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className='rounded-lg border bg-card p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex items-center gap-2'>
            <SparklesIcon className='size-4 text-muted-foreground' />
            <h4 className='text-sm font-semibold'>推荐操作</h4>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              size='sm'
              variant='outline'
              render={<Link to='/system-settings/operations/profit-center' />}
            >
              <Settings2Icon data-icon='inline-start' />
              <span>收益中心</span>
            </Button>
            <Button
              size='sm'
              variant='outline'
              render={<Link to='/system-settings/operations/prompt-compression' />}
            >
              <Settings2Icon data-icon='inline-start' />
              <span>压缩设置</span>
            </Button>
          </div>
        </div>
        <div className='mt-4 grid gap-3'>
          {recommendations.map((item) => (
            <div key={item.id} className='rounded-lg border p-3'>
              <div className='flex flex-wrap items-start justify-between gap-3'>
                <div className='min-w-0 space-y-1'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <h5 className='text-sm font-semibold'>{item.title}</h5>
                    <Badge variant='secondary'>
                      {item.confidence === 'high'
                        ? '高置信'
                        : item.confidence === 'medium'
                          ? '中置信'
                          : '低置信'}
                    </Badge>
                  </div>
                  <p className='text-sm text-muted-foreground'>{item.body}</p>
                  {detailsID === item.id && (
                    <p className='rounded-lg bg-muted/40 px-3 py-2 text-xs text-muted-foreground'>
                      {item.reason}
                    </p>
                  )}
                </div>
                <div className='flex shrink-0 flex-wrap gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() =>
                      setDetailsID(detailsID === item.id ? null : item.id)
                    }
                  >
                    <InfoIcon data-icon='inline-start' />
                    <span>查看原因</span>
                  </Button>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() => setIgnoredIDs((ids) => [...ids, item.id])}
                  >
                    <span>忽略</span>
                  </Button>
                  {item.action === 'advanced_profit' ? (
                    <Button
                      size='sm'
                      render={<Link to='/system-settings/operations/profit-center' />}
                    >
                      <Settings2Icon data-icon='inline-start' />
                      <span>进入高级设置</span>
                    </Button>
                  ) : item.action === 'advanced_compression' ? (
                    <Button
                      size='sm'
                      render={<Link to='/system-settings/operations/prompt-compression' />}
                    >
                      <Settings2Icon data-icon='inline-start' />
                      <span>进入高级设置</span>
                    </Button>
                  ) : item.action !== 'none' ? (
                    <Button
                      type='button'
                      size='sm'
                      onClick={() =>
                        setPendingAction({
                          type: 'recommendation',
                          recommendation: item,
                        })
                      }
                      disabled={isSaving}
                    >
                      <CheckCircle2Icon data-icon='inline-start' />
                      <span>应用</span>
                    </Button>
                  ) : null}
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <ConfirmDialog
        open={Boolean(pendingAction)}
        onOpenChange={(open) => !open && setPendingAction(null)}
        title='确认应用收益托管调整'
        desc={
          <div className='space-y-3 text-sm'>
            <p>影响分组：{selectedGroup}</p>
            <Separator />
            <ul className='space-y-1 text-muted-foreground'>
              <li>会更新该分组的收益观察配置。</li>
              <li>
                {pendingAction?.type === 'mode' &&
                pendingAction.mode === 'managed'
                  ? '会对该分组开启 stacked 压缩观察，可能改写上游请求。'
                  : pendingAction?.type === 'recommendation' &&
                      ['compression', 'output_cap'].includes(
                        pendingAction.recommendation.action
                      )
                    ? '会应用推荐的观察策略，其中压缩类建议会改写上游请求。'
                    : '不会改写请求内容。'}
              </li>
              <li>用户计费仍按平台计费用消耗计算。</li>
              <li>可随时切回“关闭”或“只观察”。</li>
              {selectedGroup === 'default' && (
                <li className='text-amber-600'>
                  default 分组影响范围更大，请确认当前流式链路和日志正常。
                </li>
              )}
            </ul>
          </div>
        }
        confirmText={isSaving ? '应用中' : '确认应用'}
        isLoading={isSaving}
        handleConfirm={confirmPendingAction}
      />
    </SettingsSection>
  )
}
