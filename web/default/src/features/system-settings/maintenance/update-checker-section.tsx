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
import { useEffect, useEffectEvent, useState } from 'react'
import type { TFunction } from 'i18next'
import {
  CheckCircle2Icon,
  ClipboardCheckIcon,
  GitBranchIcon,
  HardDriveIcon,
  RefreshCcwIcon,
  RocketIcon,
  SendIcon,
  Trash2Icon,
  XCircleIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  getSystemCleanupPreview,
  getSystemUpdateBackups,
  getSystemUpdateStatus,
  precheckSystemUpdate,
  prepareSystemUpstreamMerge,
  rollbackSystemUpdate,
  smokeSystemUpdate,
  startSystemCleanup,
  startSystemUpdate,
  type SystemCleanupPreview,
  type SystemCleanupRequest,
  type SystemRollbackComponent,
  type SystemUpdateBackup,
  type SystemUpdatePrecheck,
  type SystemUpdateSmoke,
  type SystemUpdateStatus,
  type SystemUpdateComponent,
} from '../api'
import { SettingsSection } from '../components/settings-section'

const UPDATE_COMPONENTS: SystemUpdateComponent[] = [
  'all',
  'new-api',
  'gpt-load',
  'cliproxyapi',
  'cpa-manager-plus',
]
const ROLLBACK_COMPONENTS: SystemRollbackComponent[] = [
  'new-api',
  'gpt-load',
  'cliproxyapi',
  'cpa-manager-plus',
]

const DEFAULT_CLEANUP_POLICY: SystemCleanupRequest = {
  keep_rollback_images: 5,
  keep_backups: 5,
  keep_legacy_images: 2,
  build_cache_max_age_hours: 168,
  prune_dangling_images: true,
  prune_build_cache: true,
  prune_build_cache_all: false,
  prune_legacy_images: false,
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1
  )
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 2)} ${units[index]}`
}

type PendingOperation =
  | {
      action: 'update'
      component: SystemUpdateComponent
    }
  | {
      action: 'rollback'
      component: SystemRollbackComponent
      backupId: string
    }
  | {
      action: 'prepare-upstream-merge'
      component: 'new-api'
    }

async function runPendingOperation(operation: PendingOperation) {
  if (operation.action === 'update') {
    return startSystemUpdate(operation.component)
  }
  if (operation.action === 'rollback') {
    return rollbackSystemUpdate({
      component: operation.component,
      backup_id: operation.backupId,
    })
  }
  return prepareSystemUpstreamMerge()
}

type UpdateCheckerSectionProps = {
  currentVersion?: string | null
  startTime?: number | null
}

export function UpdateCheckerSection({
  currentVersion,
  startTime,
}: UpdateCheckerSectionProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [prechecking, setPrechecking] = useState(false)
  const [smoking, setSmoking] = useState(false)
  const [cleanupLoading, setCleanupLoading] = useState(false)
  const [cleanupConfirmOpen, setCleanupConfirmOpen] = useState(false)
  const [cleanupPolicy, setCleanupPolicy] = useState<SystemCleanupRequest>(
    DEFAULT_CLEANUP_POLICY
  )
  const [cleanupPreview, setCleanupPreview] =
    useState<SystemCleanupPreview | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingOperation, setPendingOperation] =
    useState<PendingOperation | null>(null)
  const [backupsLoading, setBackupsLoading] = useState(false)
  const [backups, setBackups] = useState<
    Record<SystemRollbackComponent, SystemUpdateBackup[]>
  >({
    'new-api': [],
    'gpt-load': [],
    cliproxyapi: [],
    'cpa-manager-plus': [],
  })
  const [precheck, setPrecheck] = useState<SystemUpdatePrecheck | null>(null)
  const [smoke, setSmoke] = useState<SystemUpdateSmoke | null>(null)
  const [status, setStatus] = useState<SystemUpdateStatus | null>(null)

  const uptime = startTime ? formatTimestamp(startTime) : t('Unknown')
  const version = currentVersion || t('Unknown')
  const updaterEnabled = status?.enabled ?? false
  const updaterRunning = status?.running ?? false
  const precheckPassed = precheck?.ok === true
  const smokePassed = smoke?.ok === true
  const updateChecksPassed = precheckPassed && smokePassed
  const logLines = status?.log_tail ?? []
  const upstream = status?.upstream
  const upstreamNeedsUpdate = upstream?.needs_update === true
  const canStartUpdate =
    updaterEnabled && !updaterRunning && !loading && updateChecksPassed
  const canStartRollback = updaterEnabled && !updaterRunning && !loading
  const canPrepareUpstreamMerge =
    updaterEnabled && !updaterRunning && !loading && upstreamNeedsUpdate
  const canStartCleanup =
    updaterEnabled && !updaterRunning && !cleanupLoading && !!cleanupPreview

  const refreshBackups = async () => {
    if (!updaterEnabled && status) return
    setBackupsLoading(true)
    try {
      const results = await Promise.all(
        ROLLBACK_COMPONENTS.map(async (component) => ({
          component,
          response: await getSystemUpdateBackups(component),
        }))
      )
      setBackups((current) => {
        const next = { ...current }
        for (const { component, response } of results) {
          if (response.data?.backups) {
            next[component] = response.data.backups
          }
        }
        return next
      })
    } catch {
      toast.error(t('Failed to load component backups'))
    } finally {
      setBackupsLoading(false)
    }
  }

  const refreshStatus = async () => {
    try {
      const res = await getSystemUpdateStatus()
      if (res.data) {
        setStatus(res.data)
      }
    } catch {
      setStatus({
        enabled: false,
        running: false,
        message: t('Failed to load updater status'),
      })
    }
  }

  const refreshCleanupPreview = async () => {
    setCleanupLoading(true)
    try {
      const res = await getSystemCleanupPreview(cleanupPolicy)
      if (!res.success || !res.data) {
        throw new Error(res.message || '无法获取服务器空间清理预览')
      }
      setCleanupPreview(res.data)
      return res.data
    } catch (error) {
      const message =
        error instanceof Error ? error.message : '无法获取服务器空间清理预览'
      toast.error(message)
      return null
    } finally {
      setCleanupLoading(false)
    }
  }

  const refreshStatusEvent = useEffectEvent(refreshStatus)
  const refreshBackupsEvent = useEffectEvent(refreshBackups)
  const refreshCleanupPreviewEvent = useEffectEvent(refreshCleanupPreview)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void refreshStatusEvent()
      void refreshBackupsEvent()
      void refreshCleanupPreviewEvent()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [])

  useEffect(() => {
    if (!updaterRunning) return
    const timer = window.setInterval(() => {
      void refreshStatusEvent()
    }, 5000)
    return () => window.clearInterval(timer)
  }, [updaterRunning])

  useEffect(() => {
    if (updaterRunning || status?.last_exit !== 0) return
    const timer = window.setTimeout(() => {
      void refreshBackupsEvent()
      void refreshCleanupPreviewEvent()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [updaterRunning, status?.last_exit])

  const updateCleanupNumber = (
    key:
      | 'keep_rollback_images'
      | 'keep_backups'
      | 'keep_legacy_images'
      | 'build_cache_max_age_hours',
    value: string,
    minimum: number,
    maximum: number
  ) => {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) return
    setCleanupPolicy((current) => ({
      ...current,
      [key]: Math.min(maximum, Math.max(minimum, Math.round(parsed))),
    }))
  }

  const handleStartCleanup = async () => {
    setCleanupLoading(true)
    try {
      const res = await startSystemCleanup(cleanupPolicy)
      if (res.data) setStatus(res.data)
      if (!res.success) {
        throw new Error(res.message || '无法启动服务器空间清理任务')
      }
      setCleanupConfirmOpen(false)
      toast.success('服务器空间清理任务已启动。')
    } catch (error) {
      const message =
        error instanceof Error ? error.message : '无法启动服务器空间清理任务'
      toast.error(message)
    } finally {
      setCleanupLoading(false)
    }
  }

  const handleOpenCleanupConfirm = async () => {
    const preview = await refreshCleanupPreview()
    if (preview) setCleanupConfirmOpen(true)
  }

  const handlePrecheck = async () => {
    setPrechecking(true)
    try {
      const res = await precheckSystemUpdate()
      if (res.data) {
        setPrecheck(res.data)
      }
      if (res.success) {
        toast.success(t('Precheck passed.'))
      } else {
        toast.error(res.message || t('Precheck failed'))
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Precheck failed')
      toast.error(message)
    } finally {
      setPrechecking(false)
    }
  }

  const handleSmoke = async () => {
    setSmoking(true)
    try {
      const res = await smokeSystemUpdate()
      if (res.data) {
        setSmoke(res.data)
      }
      if (res.success) {
        toast.success(t('Smoke test passed.'))
      } else {
        toast.error(res.message || t('Smoke test failed'))
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Smoke test failed')
      toast.error(message)
    } finally {
      setSmoking(false)
    }
  }

  const openUpdateConfirm = (component: SystemUpdateComponent) => {
    setPendingOperation({ action: 'update', component })
    setConfirmOpen(true)
  }

  const openPrepareUpstreamMergeConfirm = () => {
    setPendingOperation({
      action: 'prepare-upstream-merge',
      component: 'new-api',
    })
    setConfirmOpen(true)
  }

  const openRollbackConfirm = (
    component: SystemRollbackComponent,
    backupId: string
  ) => {
    setPendingOperation({ action: 'rollback', component, backupId })
    setConfirmOpen(true)
  }

  const handleConfirmOperation = async () => {
    const operation = pendingOperation
    if (!operation) return
    setConfirmOpen(false)
    setLoading(true)
    try {
      const res = await runPendingOperation(operation)
      if (res.data) {
        setStatus(res.data)
      }
      if (res.success) {
        toast.success(
          operation.action === 'update'
            ? t('Update task started.')
            : operation.action === 'rollback'
              ? t('Rollback task started.')
              : '上游合并准备任务已启动。'
        )
      } else {
        toast.error(
          res.message ||
            (operation.action === 'update'
              ? t('Failed to start update task')
              : operation.action === 'rollback'
                ? t('Failed to start rollback task')
                : '无法启动上游合并准备任务')
        )
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : operation.action === 'update'
            ? t('Failed to start update task')
            : operation.action === 'rollback'
              ? t('Failed to start rollback task')
              : '无法启动上游合并准备任务'
      toast.error(message)
    } finally {
      setLoading(false)
      setPendingOperation(null)
    }
  }

  const pendingComponentLabel = pendingOperation
    ? getSystemUpdateComponentLabel(t, pendingOperation.component)
    : ''
  const pendingIsRollback = pendingOperation?.action === 'rollback'
  const pendingIsPrepareMerge =
    pendingOperation?.action === 'prepare-upstream-merge'
  const cleanupDescription = [
    `每个组件保留 ${cleanupPolicy.keep_rollback_images ?? 5} 个回滚镜像和 ${cleanupPolicy.keep_backups ?? 5} 个备份`,
    cleanupPolicy.prune_legacy_images
      ? `保留 ${cleanupPolicy.keep_legacy_images ?? 2} 个旧版应用镜像`
      : '不清理旧版应用镜像',
    cleanupPolicy.prune_dangling_images ? '清理悬空镜像' : '保留悬空镜像',
    cleanupPolicy.prune_build_cache
      ? `${cleanupPolicy.prune_build_cache_all ? '深度清理' : '清理'}超过 ${cleanupPolicy.build_cache_max_age_hours ?? 168} 小时的未使用构建缓存`
      : '保留构建缓存',
  ].join('，')

  return (
    <SettingsSection title={t('System maintenance')}>
      <div className='space-y-6'>
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='rounded-lg border p-4'>
            <div className='text-muted-foreground text-sm'>
              {t('Current version')}
            </div>
            <div className='text-lg font-semibold'>{version}</div>
          </div>
          <div className='rounded-lg border p-4'>
            <div className='text-muted-foreground text-sm'>
              {t('Uptime since')}
            </div>
            <div className='text-lg font-semibold'>{uptime}</div>
          </div>
        </div>

        <div className='rounded-lg border p-4'>
          <div className='mb-3 flex flex-wrap items-center gap-2'>
            <div className='font-medium'>官方上游基线</div>
            <Badge
              variant={
                upstream?.error
                  ? 'destructive'
                  : upstreamNeedsUpdate
                    ? 'secondary'
                    : 'default'
              }
            >
              {upstream?.error
                ? '检测失败'
                : upstreamNeedsUpdate
                  ? '需要合并上游'
                  : '已跟上官方上游'}
            </Badge>
            {upstream?.checked_at && (
              <div className='text-muted-foreground text-sm'>
                检查时间：{upstream.checked_at}
              </div>
            )}
          </div>
          <div className='grid gap-3 text-sm md:grid-cols-3'>
            <CommitInfoBlock
              label='当前定制版'
              commit={upstream?.current}
              fallback={version}
            />
            <CommitInfoBlock label='当前上游基线' commit={upstream?.baseline} />
            <CommitInfoBlock label='官方最新上游' commit={upstream?.latest} />
          </div>
          <div className='text-muted-foreground mt-3 text-sm'>
            {upstream?.error
              ? `无法检测官方上游：${upstream.error}`
              : upstreamNeedsUpdate
                ? `官方上游已有 ${upstream.upstream_commits_since_baseline} 个新提交；当前定制分支相对基线有 ${upstream.custom_commits_since_baseline} 个自定义提交。建议先把 QuantumNous/new-api 的更新合并到定制分支，通过测试后再执行 new-api 单独更新。`
                : '当前定制分支的上游基线已经等于官方最新提交，暂时不需要合并官方上游。'}
          </div>
          {upstream?.source_url && (
            <div className='text-muted-foreground mt-2 text-xs break-all'>
              官方来源：{upstream.source_url}，分支：{upstream.branch || 'main'}
            </div>
          )}
          <div className='mt-4 flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='secondary'
              onClick={openPrepareUpstreamMergeConfirm}
              disabled={!canPrepareUpstreamMerge}
            >
              <GitBranchIcon className='me-2 h-4 w-4' />
              准备上游合并
            </Button>
            <div className='text-muted-foreground text-xs'>
              在服务器 staging worktree
              合并官方上游，测试通过后推送定制分支；生产目录不会被直接合并。
            </div>
          </div>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='secondary'
            onClick={handlePrecheck}
            disabled={!updaterEnabled || updaterRunning || prechecking}
          >
            <ClipboardCheckIcon className='me-2 h-4 w-4' />
            {prechecking ? t('Checking...') : t('Run precheck')}
          </Button>
          <Button
            type='button'
            variant='secondary'
            onClick={handleSmoke}
            disabled={!updaterEnabled || updaterRunning || smoking}
          >
            <SendIcon className='me-2 h-4 w-4' />
            {smoking ? t('Running smoke...') : t('Run smoke')}
          </Button>
          <Button type='button' variant='secondary' onClick={refreshStatus}>
            <RefreshCcwIcon className='me-2 h-4 w-4' />
            {t('Refresh status')}
          </Button>
        </div>

        <div className='flex flex-wrap items-center gap-2 text-sm'>
          <Badge variant={precheckPassed ? 'default' : 'secondary'}>
            {precheckPassed ? t('Precheck passed') : t('Precheck required')}
          </Badge>
          <Badge variant={smokePassed ? 'default' : 'secondary'}>
            {smokePassed ? t('Smoke passed') : t('Smoke required')}
          </Badge>
          <div className='text-muted-foreground'>
            {updateChecksPassed
              ? t('Combined update is ready.')
              : t('Run precheck and smoke before combined update.')}
          </div>
        </div>

        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
          {UPDATE_COMPONENTS.map((component) => {
            const rollbackComponent =
              component === 'all'
                ? null
                : (component as SystemRollbackComponent)
            const latestBackup = rollbackComponent
              ? backups[rollbackComponent]?.[0]
              : null
            return (
              <div key={component} className='space-y-3 rounded-lg border p-4'>
                <div>
                  <div className='font-medium'>
                    {getSystemUpdateComponentLabel(t, component)}
                  </div>
                  <div className='text-muted-foreground text-sm'>
                    {component === 'all'
                      ? t('Runs each component update with separate backups.')
                      : latestBackup
                        ? `${t('Latest backup')}: ${latestBackup.id}`
                        : backupsLoading
                          ? t('Loading backups...')
                          : t('No backups yet')}
                  </div>
                </div>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    onClick={() => openUpdateConfirm(component)}
                    disabled={!canStartUpdate}
                  >
                    <RocketIcon className='me-2 h-4 w-4' />
                    {component === 'all'
                      ? t('Combined update')
                      : t('Update component')}
                  </Button>
                  {rollbackComponent && (
                    <Button
                      type='button'
                      size='sm'
                      variant='secondary'
                      onClick={() =>
                        latestBackup &&
                        openRollbackConfirm(rollbackComponent, latestBackup.id)
                      }
                      disabled={!canStartRollback || !latestBackup}
                    >
                      <RefreshCcwIcon className='me-2 h-4 w-4' />
                      {t('Rollback latest')}
                    </Button>
                  )}
                </div>
              </div>
            )
          })}
        </div>

        <div className='rounded-lg border p-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div>
              <div className='flex items-center gap-2 font-medium'>
                <HardDriveIcon className='h-4 w-4' />
                服务器空间清理
              </div>
              <div className='text-muted-foreground mt-1 text-sm'>
                仅清理超出保留数量的回滚镜像和备份、悬空镜像及过期构建缓存，不触碰运行中容器和业务数据。
              </div>
            </div>
            {cleanupPreview && (
              <Badge
                variant={
                  cleanupPreview.disk.used_percent >= 90
                    ? 'destructive'
                    : cleanupPreview.disk.used_percent >= 80
                      ? 'secondary'
                      : 'default'
                }
              >
                磁盘已用 {cleanupPreview.disk.used_percent}%
              </Badge>
            )}
          </div>

          <div className='mt-4 grid gap-4 md:grid-cols-4'>
            <label className='space-y-2 text-sm'>
              <span className='font-medium'>每个组件保留回滚镜像</span>
              <Input
                type='number'
                min={1}
                max={20}
                value={cleanupPolicy.keep_rollback_images ?? 5}
                onChange={(event) =>
                  updateCleanupNumber(
                    'keep_rollback_images',
                    event.target.value,
                    1,
                    20
                  )
                }
                disabled={updaterRunning || cleanupLoading}
              />
            </label>
            <label className='space-y-2 text-sm'>
              <span className='font-medium'>每个组件保留备份</span>
              <Input
                type='number'
                min={1}
                max={20}
                value={cleanupPolicy.keep_backups ?? 5}
                onChange={(event) =>
                  updateCleanupNumber('keep_backups', event.target.value, 1, 20)
                }
                disabled={updaterRunning || cleanupLoading}
              />
            </label>
            <label className='space-y-2 text-sm'>
              <span className='font-medium'>旧版应用镜像保留数量</span>
              <Input
                type='number'
                min={1}
                max={10}
                value={cleanupPolicy.keep_legacy_images ?? 2}
                onChange={(event) =>
                  updateCleanupNumber(
                    'keep_legacy_images',
                    event.target.value,
                    1,
                    10
                  )
                }
                disabled={updaterRunning || cleanupLoading}
              />
            </label>
            <label className='space-y-2 text-sm'>
              <span className='font-medium'>构建缓存最长保留（小时）</span>
              <Input
                type='number'
                min={24}
                max={2160}
                step={24}
                value={cleanupPolicy.build_cache_max_age_hours ?? 168}
                onChange={(event) =>
                  updateCleanupNumber(
                    'build_cache_max_age_hours',
                    event.target.value,
                    24,
                    2160
                  )
                }
                disabled={updaterRunning || cleanupLoading}
              />
            </label>
          </div>

          <div className='mt-4 flex flex-wrap gap-6 text-sm'>
            <label className='flex items-center gap-2'>
              <Switch
                checked={cleanupPolicy.prune_dangling_images ?? true}
                onCheckedChange={(checked) =>
                  setCleanupPolicy((current) => ({
                    ...current,
                    prune_dangling_images: checked,
                  }))
                }
                disabled={updaterRunning || cleanupLoading}
              />
              清理悬空镜像
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={cleanupPolicy.prune_build_cache ?? true}
                onCheckedChange={(checked) =>
                  setCleanupPolicy((current) => ({
                    ...current,
                    prune_build_cache: checked,
                  }))
                }
                disabled={updaterRunning || cleanupLoading}
              />
              清理过期构建缓存
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={cleanupPolicy.prune_build_cache_all ?? false}
                onCheckedChange={(checked) =>
                  setCleanupPolicy((current) => ({
                    ...current,
                    prune_build_cache_all: checked,
                  }))
                }
                disabled={
                  updaterRunning ||
                  cleanupLoading ||
                  !cleanupPolicy.prune_build_cache
                }
              />
              深度清理全部未使用构建缓存
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={cleanupPolicy.prune_legacy_images ?? false}
                onCheckedChange={(checked) =>
                  setCleanupPolicy((current) => ({
                    ...current,
                    prune_legacy_images: checked,
                  }))
                }
                disabled={updaterRunning || cleanupLoading}
              />
              清理旧版应用镜像
            </label>
          </div>

          {cleanupPreview && (
            <div className='mt-4 grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4'>
              <div>
                <div className='text-muted-foreground'>磁盘可用</div>
                <div className='font-medium'>
                  {formatBytes(cleanupPreview.disk.free_bytes)} /{' '}
                  {formatBytes(cleanupPreview.disk.total_bytes)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>Docker 镜像</div>
                <div className='font-medium'>
                  {cleanupPreview.docker?.images?.size ?? '未知'}，可回收{' '}
                  {cleanupPreview.docker?.images?.reclaimable ?? '未知'}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>构建缓存</div>
                <div className='font-medium'>
                  {cleanupPreview.docker?.build_cache?.size ?? '未知'}，可回收{' '}
                  {cleanupPreview.docker?.build_cache?.reclaimable ?? '未知'}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground'>本次候选</div>
                <div className='font-medium'>
                  {cleanupPreview.rollback_images?.candidate_count ?? 0}{' '}
                  个回滚镜像，{cleanupPreview.backups?.candidate_count ?? 0}{' '}
                  个备份，{cleanupPreview.legacy_images?.candidate_count ?? 0}{' '}
                  个旧版镜像
                </div>
              </div>
            </div>
          )}

          {status?.last_cleanup && (
            <div className='text-muted-foreground mt-3 text-sm'>
              上次清理释放 {formatBytes(status.last_cleanup.freed_bytes)}，删除{' '}
              {status.last_cleanup.removed_rollback_images} 个回滚镜像和{' '}
              {status.last_cleanup.removed_backups} 个备份、{' '}
              {status.last_cleanup.removed_legacy_images} 个旧版镜像。
            </div>
          )}

          <div className='mt-4 flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='secondary'
              onClick={() => void refreshCleanupPreview()}
              disabled={!updaterEnabled || updaterRunning || cleanupLoading}
            >
              <RefreshCcwIcon className='me-2 h-4 w-4' />
              {cleanupLoading ? '正在读取...' : '刷新清理预览'}
            </Button>
            <Button
              type='button'
              variant='destructive'
              onClick={() => void handleOpenCleanupConfirm()}
              disabled={!updaterEnabled || updaterRunning || cleanupLoading}
            >
              <Trash2Icon className='me-2 h-4 w-4' />
              执行安全清理
            </Button>
          </div>
        </div>

        <ConfirmDialog
          open={cleanupConfirmOpen}
          onOpenChange={setCleanupConfirmOpen}
          title='执行服务器空间清理？'
          desc={`${cleanupDescription}。当前预览包含 ${cleanupPreview?.rollback_images?.candidate_count ?? 0} 个回滚镜像和 ${cleanupPreview?.backups?.candidate_count ?? 0} 个备份。`}
          confirmText='确认清理'
          destructive
          isLoading={cleanupLoading}
          disabled={!canStartCleanup}
          handleConfirm={handleStartCleanup}
        />

        <ConfirmDialog
          open={confirmOpen}
          onOpenChange={(open) => {
            setConfirmOpen(open)
            if (!open) setPendingOperation(null)
          }}
          title={
            pendingIsRollback
              ? t('Rollback component?')
              : pendingIsPrepareMerge
                ? '准备上游合并？'
                : pendingOperation?.component === 'all'
                  ? t('Start combined one-click update?')
                  : t('Start component update?')
          }
          desc={
            pendingIsRollback
              ? t(
                  'This will roll back only the selected component image. Runtime data is not restored by default.'
                )
              : pendingIsPrepareMerge
                ? '这会在服务器独立 staging worktree 中合并 QuantumNous/new-api 官方上游，运行检查和构建验证，成功后推送到当前定制分支。它不会在生产目录直接执行 merge；完成后你还需要执行 new-api 单独更新来部署已验证提交。'
                : pendingOperation?.component === 'all'
                  ? t(
                      'This will update new-api, GPT-Load, CLIProxyAPI, and CPA Manager Plus together while preserving separate backups for each component.'
                    )
                  : t(
                      'This will update only the selected component and create a component-specific rollback point.'
                    )
          }
          confirmText={
            pendingIsRollback
              ? `${t('Rollback')} ${pendingComponentLabel}`
              : pendingIsPrepareMerge
                ? '开始准备合并'
                : `${t('Update')} ${pendingComponentLabel}`
          }
          destructive={pendingIsRollback}
          isLoading={loading}
          disabled={
            !pendingOperation ||
            !updaterEnabled ||
            updaterRunning ||
            (pendingOperation.action === 'update' && !updateChecksPassed) ||
            (pendingOperation.action === 'prepare-upstream-merge' &&
              !upstreamNeedsUpdate)
          }
          handleConfirm={handleConfirmOperation}
        />

        <div className='rounded-lg border p-4'>
          <div className='grid gap-3 text-sm md:grid-cols-2'>
            <div>
              <div className='text-muted-foreground'>{t('Updater status')}</div>
              <div className='font-medium'>
                {!updaterEnabled
                  ? t('Not configured')
                  : updaterRunning
                    ? t('Running')
                    : status?.last_exit === 0
                      ? t('Last update succeeded')
                      : status?.last_exit
                        ? t('Last update failed')
                        : t('Idle')}
              </div>
            </div>
            <div>
              <div className='text-muted-foreground'>{t('Last message')}</div>
              <div className='font-medium'>
                {status?.message || t('Unknown')}
              </div>
            </div>
            {status?.started_at && (
              <div>
                <div className='text-muted-foreground'>{t('Started at')}</div>
                <div className='font-medium'>{status.started_at}</div>
              </div>
            )}
            {status?.current_action && status?.current_component && (
              <div>
                <div className='text-muted-foreground'>
                  {t('Current operation')}
                </div>
                <div className='font-medium'>
                  {status.current_action}{' '}
                  {getSystemUpdateComponentLabel(t, status.current_component)}
                </div>
              </div>
            )}
            {status?.current_backup_id &&
              status.current_backup_id !== 'latest' && (
                <div>
                  <div className='text-muted-foreground'>{t('Backup ID')}</div>
                  <div className='font-medium'>{status.current_backup_id}</div>
                </div>
              )}
            {status?.current_staging_dir && (
              <div>
                <div className='text-muted-foreground'>Staging 目录</div>
                <div className='font-medium break-all'>
                  {status.current_staging_dir}
                </div>
              </div>
            )}
            {status?.finished_at && (
              <div>
                <div className='text-muted-foreground'>{t('Finished at')}</div>
                <div className='font-medium'>{status.finished_at}</div>
              </div>
            )}
          </div>

          {logLines.length > 0 && (
            <div className='mt-4'>
              <div className='text-muted-foreground mb-2 text-sm'>
                {t('Current task log')}
              </div>
              <pre className='bg-muted max-h-80 overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap'>
                {logLines.join('\n')}
              </pre>
            </div>
          )}
        </div>

        {smoke && (
          <div className='rounded-lg border p-4'>
            <div className='mb-3 flex flex-wrap items-center gap-2'>
              <div className='font-medium'>{t('Smoke test results')}</div>
              <Badge variant={smoke.ok ? 'default' : 'destructive'}>
                {smoke.ok ? t('Passed') : t('Failed')}
              </Badge>
              {smoke.checked_at && (
                <div className='text-muted-foreground text-sm'>
                  {smoke.checked_at}
                </div>
              )}
            </div>
            <div className='grid gap-3 text-sm md:grid-cols-2'>
              <HealthRow
                label={t('new-api health')}
                ok={smoke.new_api_healthy}
              />
              <HealthRow
                label={t('GPT-Load health')}
                ok={smoke.gpt_load_healthy}
              />
              <HealthRow
                label={t('CLIProxyAPI health')}
                ok={smoke.cliproxyapi_ready}
              />
              <HealthRow
                label={t('CPA Manager Plus health')}
                ok={smoke.cpa_manager_plus_ready ?? false}
              />
              <HealthRow
                label={t('Sidecar bridge sources')}
                ok={smoke.sidecar_bridge_sources_ok ?? false}
              />
              <HealthRow
                label={t('响应缓存自定义层')}
                ok={smoke.response_cache_sources_ok ?? false}
              />
              <HealthRow
                label={t('输出限额自定义层')}
                ok={smoke.output_policy_sources_ok ?? false}
              />
              <HealthRow
                label={t('收益风控自定义层')}
                ok={smoke.profit_risk_sources_ok ?? false}
              />
              <HealthRow
                label={t('OmniRoute 压缩一致性保护')}
                ok={smoke.omniroute_parity_sources_ok ?? false}
              />
              <HealthRow
                label={t('V2 online smoke script')}
                ok={smoke.v2_smoke_script_ok ?? false}
              />
              <HealthRow
                label={t('proxy-test chat smoke')}
                ok={
                  smoke.proxy_test_chat_ok === true ||
                  smoke.proxy_test_chat_skipped === true
                }
                statusText={
                  smoke.proxy_test_chat_skipped
                    ? t('Skipped')
                    : smoke.proxy_test_chat_checked
                      ? smoke.proxy_test_chat_ok
                        ? t('OK')
                        : t('Failed')
                      : t('Not configured')
                }
              />
              <div>
                <div className='text-muted-foreground'>{t('HTTP status')}</div>
                <div className='font-medium'>
                  {smoke.status ?? t('Unknown')}
                </div>
              </div>
            </div>
            {smoke.error && (
              <div className='text-destructive mt-3 text-sm'>{smoke.error}</div>
            )}
          </div>
        )}

        {precheck && (
          <div className='rounded-lg border p-4'>
            <div className='mb-3 flex flex-wrap items-center gap-2'>
              <div className='font-medium'>{t('Precheck results')}</div>
              <Badge variant={precheck.ok ? 'default' : 'destructive'}>
                {precheck.ok ? t('Passed') : t('Failed')}
              </Badge>
              {precheck.checked_at && (
                <div className='text-muted-foreground text-sm'>
                  {precheck.checked_at}
                </div>
              )}
            </div>
            <div className='divide-y rounded-md border'>
              {(precheck.checks ?? []).map((item) => (
                <div
                  key={item.name}
                  className='grid gap-2 p-3 text-sm md:grid-cols-[220px_1fr]'
                >
                  <div className='flex items-center gap-2 font-medium'>
                    {item.ok ? (
                      <CheckCircle2Icon className='text-primary h-4 w-4' />
                    ) : (
                      <XCircleIcon className='text-destructive h-4 w-4' />
                    )}
                    {item.name}
                  </div>
                  <div className='text-muted-foreground break-words'>
                    {item.message || (item.ok ? t('OK') : t('Failed'))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </SettingsSection>
  )
}

function HealthRow({
  label,
  ok,
  statusText,
}: {
  label: string
  ok: boolean
  statusText?: string
}) {
  const { t } = useTranslation()
  return (
    <div>
      <div className='text-muted-foreground'>{label}</div>
      <div className='font-medium'>
        {statusText ?? (ok ? t('OK') : t('Failed'))}
      </div>
    </div>
  )
}

function CommitInfoBlock({
  label,
  commit,
  fallback,
}: {
  label: string
  commit?: {
    version?: string
    short_commit?: string
    date?: string
    subject?: string
  } | null
  fallback?: string
}) {
  const displayVersion = commit?.version || fallback || '未知'
  const shortCommit = commit?.short_commit
  return (
    <div className='rounded-md border p-3'>
      <div className='text-muted-foreground'>{label}</div>
      <div className='mt-1 font-medium break-words'>{displayVersion}</div>
      {shortCommit && (
        <div className='text-muted-foreground mt-1 font-mono text-xs'>
          {shortCommit}
        </div>
      )}
      {commit?.date && (
        <div className='text-muted-foreground mt-1 text-xs'>{commit.date}</div>
      )}
      {commit?.subject && (
        <div className='text-muted-foreground mt-1 line-clamp-2 text-xs'>
          {commit.subject}
        </div>
      )}
    </div>
  )
}

function getSystemUpdateComponentLabel(
  t: TFunction,
  component: SystemUpdateComponent
) {
  switch (component) {
    case 'all':
      return t('Combined stack')
    case 'new-api':
      return 'new-api'
    case 'gpt-load':
      return 'GPT-Load'
    case 'cliproxyapi':
      return 'CLIProxyAPI'
    case 'cpa-manager-plus':
      return 'CPA Manager Plus'
  }
}
