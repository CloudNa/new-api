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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  AlertTriangle,
  Coins,
  Gauge,
  Sparkles,
  TrendingUp,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  getProfitAnalytics,
  type ProfitAnalytics,
} from '@/features/system-settings/api'
import { PanelWrapper } from '../ui/panel-wrapper'

function formatUSD(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  const absValue = Math.abs(value)
  const formatted = `$${absValue.toFixed(absValue >= 1 ? 2 : 4)}`
  return value < 0 ? `-${formatted}` : formatted
}

function formatPercent(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return `${value.toFixed(1)}%`
}

function formatNumber(value?: number | null) {
  return new Intl.NumberFormat('zh-CN').format(value ?? 0)
}

function ratioPercent(value?: number | null, total?: number | null) {
  const safeValue = Number(value ?? 0)
  const safeTotal = Math.max(Number(total ?? 0), safeValue, 1)
  return Math.min(100, Math.max(0, (safeValue / safeTotal) * 100))
}

function marginTone(value?: number | null) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return 'text-muted-foreground'
  }
  if (value < 0) return 'text-destructive'
  if (value < 20) return 'text-warning'
  return 'text-success'
}

function MiniStat(props: {
  label: string
  value: string
  description: string
  icon: LucideIcon
  tone?: string
}) {
  const Icon = props.icon

  return (
    <div className='bg-background/60 min-w-0 rounded-xl border p-3'>
      <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
        <Icon className='size-3.5 shrink-0' aria-hidden='true' />
        <span className='truncate'>{props.label}</span>
      </div>
      <div
        className={cn(
          'mt-2 truncate font-mono text-lg font-semibold tabular-nums',
          props.tone
        )}
        title={props.value}
      >
        {props.value}
      </div>
      <div className='text-muted-foreground mt-1 line-clamp-2 text-xs'>
        {props.description}
      </div>
    </div>
  )
}

function BarRow(props: {
  label: string
  value: string
  percent: number
  tone?: 'default' | 'success' | 'warning' | 'destructive'
}) {
  const color =
    props.tone === 'success'
      ? 'bg-success'
      : props.tone === 'warning'
        ? 'bg-warning'
        : props.tone === 'destructive'
          ? 'bg-destructive'
          : 'bg-primary'

  return (
    <div className='grid gap-1.5'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground truncate'>{props.label}</span>
        <span className='shrink-0 font-mono font-medium tabular-nums'>
          {props.value}
        </span>
      </div>
      <div
        className='bg-muted h-2 overflow-hidden rounded-full'
        role='img'
        aria-label={`${props.label}: ${props.value}`}
      >
        <div
          className={cn('h-full rounded-full', color)}
          style={{ width: `${props.percent}%` }}
        />
      </div>
    </div>
  )
}

function buildRecommendation(analytics?: ProfitAnalytics | null) {
  if (!analytics || analytics.request_count === 0) {
    return '暂无足够样本，建议先保持只观察。'
  }
  if (analytics.missing_cost_profile_count > 0) {
    return '存在缺失成本档案，建议先补齐成本估算。'
  }
  if ((analytics.profit_risk_loss_making_count ?? 0) > 0) {
    return '存在亏损请求，建议进入收益托管查看建议。'
  }
  if ((analytics.output_policy_completion_p99_tokens ?? 0) > 4096) {
    return '输出长尾偏高，建议观察输出上限策略。'
  }
  if ((analytics.compression_saved_tokens ?? 0) > 0) {
    return '压缩已经产生节省，可继续观察收益表现。'
  }
  return '当前没有高风险信号，保持托管观察即可。'
}

export function ProfitManagedPanel() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['dashboard', 'profit-managed', 'analytics'],
    queryFn: async () => {
      const result = await getProfitAnalytics()
      if (!result.success) {
        throw new Error(result.message || 'Failed to load profit analytics')
      }
      return result.data ?? null
    },
    staleTime: 60 * 1000,
  })

  const analytics = query.data
  const totalRevenueAndCost = Math.max(
    analytics?.estimated_revenue_usd ?? 0,
    analytics?.estimated_upstream_cost_usd ?? 0,
    Math.abs(analytics?.gross_margin_usd ?? 0),
    1
  )
  const recommendation = useMemo(
    () => buildRecommendation(analytics),
    [analytics]
  )

  return (
    <PanelWrapper
      title={t('收益托管概览')}
      description={t('仅 root 可见，展示平台收益、节省和风险信号。')}
      loading={query.isLoading}
      empty={!query.isLoading && !analytics}
      emptyMessage={t('暂无收益托管数据')}
      headerActions={
        <Button
          size='sm'
          variant='outline'
          render={<Link to='/profit-managed' />}
        >
          <Sparkles data-icon='inline-start' />
          <span>{t('进入收益托管')}</span>
        </Button>
      }
    >
      <div className='grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'>
        <div className='grid min-w-0 gap-3 sm:grid-cols-2'>
          <MiniStat
            icon={TrendingUp}
            label={t('毛利率')}
            value={formatPercent(analytics?.gross_margin_pct)}
            description={t('收入与上游成本的估算差值')}
            tone={marginTone(analytics?.gross_margin_pct)}
          />
          <MiniStat
            icon={Coins}
            label={t('估算毛利')}
            value={formatUSD(analytics?.gross_margin_usd)}
            description={t('平台内部收益口径，不对用户展示')}
            tone={marginTone(analytics?.gross_margin_pct)}
          />
          <MiniStat
            icon={Sparkles}
            label={t('压缩节省')}
            value={`${formatNumber(analytics?.compression_saved_tokens)} tokens`}
            description={t('请求前压缩带来的上游 token 节省')}
          />
          <MiniStat
            icon={AlertTriangle}
            label={t('风险请求')}
            value={formatNumber(analytics?.profit_risk_loss_making_count)}
            description={t('亏损或低毛利风险需要关注')}
            tone={
              (analytics?.profit_risk_loss_making_count ?? 0) > 0
                ? 'text-destructive'
                : 'text-foreground'
            }
          />
        </div>

        <div className='bg-background/60 grid min-w-0 content-start gap-4 rounded-xl border p-3'>
          <div className='grid gap-3'>
            <BarRow
              label={t('估算收入')}
              value={formatUSD(analytics?.estimated_revenue_usd)}
              percent={ratioPercent(
                analytics?.estimated_revenue_usd,
                totalRevenueAndCost
              )}
              tone='success'
            />
            <BarRow
              label={t('上游成本')}
              value={formatUSD(analytics?.estimated_upstream_cost_usd)}
              percent={ratioPercent(
                analytics?.estimated_upstream_cost_usd,
                totalRevenueAndCost
              )}
              tone='warning'
            />
            <BarRow
              label={t('重试成本')}
              value={formatUSD(analytics?.retry_cost_usd)}
              percent={ratioPercent(
                analytics?.retry_cost_usd,
                totalRevenueAndCost
              )}
              tone={
                (analytics?.retry_cost_usd ?? 0) > 0 ? 'destructive' : 'default'
              }
            />
            <BarRow
              label={t('成本档案覆盖')}
              value={`${formatNumber(analytics?.cost_known_count)} / ${formatNumber(
                analytics?.request_count
              )}`}
              percent={ratioPercent(
                analytics?.cost_known_count,
                analytics?.request_count
              )}
              tone={
                (analytics?.missing_cost_profile_count ?? 0) > 0
                  ? 'warning'
                  : 'success'
              }
            />
          </div>

          <div className='bg-muted/40 flex min-w-0 items-start gap-2 rounded-lg p-3'>
            <Gauge className='text-muted-foreground mt-0.5 size-4 shrink-0' />
            <div className='min-w-0'>
              <div className='text-sm font-medium'>{t('系统建议')}</div>
              <div className='text-muted-foreground mt-1 text-sm leading-6'>
                {recommendation}
              </div>
            </div>
          </div>

          <div className='grid grid-cols-2 gap-2 text-xs sm:grid-cols-4'>
            <div className='rounded-lg border px-2.5 py-2'>
              <div className='text-muted-foreground'>{t('请求样本')}</div>
              <div className='mt-1 font-mono font-semibold'>
                {formatNumber(analytics?.request_count)}
              </div>
            </div>
            <div className='rounded-lg border px-2.5 py-2'>
              <div className='text-muted-foreground'>{t('缺失成本')}</div>
              <div className='mt-1 font-mono font-semibold'>
                {formatNumber(analytics?.missing_cost_profile_count)}
              </div>
            </div>
            <div className='rounded-lg border px-2.5 py-2'>
              <div className='text-muted-foreground'>{t('输出 P99')}</div>
              <div className='mt-1 font-mono font-semibold'>
                {formatNumber(analytics?.output_policy_completion_p99_tokens)}
              </div>
            </div>
            <div className='rounded-lg border px-2.5 py-2'>
              <div className='text-muted-foreground'>{t('缓存节省')}</div>
              <div className='mt-1 font-mono font-semibold'>
                {formatUSD(analytics?.response_cache_saved_usd)}
              </div>
            </div>
          </div>
        </div>
      </div>
    </PanelWrapper>
  )
}
