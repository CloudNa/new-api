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
import { useEffect, useState } from 'react'
import {
  CheckCircle2Icon,
  ClipboardCheckIcon,
  RefreshCcwIcon,
  RocketIcon,
  SendIcon,
  XCircleIcon,
} from 'lucide-react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  getSystemUpdateBackups,
  getSystemUpdateStatus,
  precheckSystemUpdate,
  rollbackSystemUpdate,
  smokeSystemUpdate,
  startSystemUpdate,
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
  const canStartUpdate =
    updaterEnabled && !updaterRunning && !loading && updateChecksPassed
  const canStartRollback = updaterEnabled && !updaterRunning && !loading

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

  useEffect(() => {
    void refreshStatus()
    void refreshBackups()
  }, [])

  useEffect(() => {
    if (!updaterRunning) return
    const timer = window.setInterval(() => {
      void refreshStatus()
    }, 5000)
    return () => window.clearInterval(timer)
  }, [updaterRunning])

  useEffect(() => {
    if (updaterRunning || status?.last_exit !== 0) return
    void refreshBackups()
  }, [updaterRunning, status?.last_exit])

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
      const res =
        operation.action === 'update'
          ? await startSystemUpdate(operation.component)
          : await rollbackSystemUpdate({
              component: operation.component,
              backup_id: operation.backupId,
            })
      if (res.data) {
        setStatus(res.data)
      }
      if (res.success) {
        toast.success(
          operation.action === 'update'
            ? t('Update task started.')
            : t('Rollback task started.')
        )
      } else {
        toast.error(
          res.message ||
            (operation.action === 'update'
              ? t('Failed to start update task')
              : t('Failed to start rollback task'))
        )
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : operation.action === 'update'
            ? t('Failed to start update task')
            : t('Failed to start rollback task')
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
              component === 'all' ? null : (component as SystemRollbackComponent)
            const latestBackup = rollbackComponent
              ? backups[rollbackComponent]?.[0]
              : null
            return (
              <div
                key={component}
                className='space-y-3 rounded-lg border p-4'
              >
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

        <ConfirmDialog
          open={confirmOpen}
          onOpenChange={(open) => {
            setConfirmOpen(open)
            if (!open) setPendingOperation(null)
          }}
          title={
            pendingIsRollback
              ? t('Rollback component?')
              : pendingOperation?.component === 'all'
                ? t('Start combined one-click update?')
                : t('Start component update?')
          }
          desc={
            pendingIsRollback
              ? t(
                  'This will roll back only the selected component image. Runtime data is not restored by default.'
                )
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
              : `${t('Update')} ${pendingComponentLabel}`
          }
          destructive={pendingIsRollback}
          isLoading={loading}
          disabled={
            !pendingOperation ||
            !updaterEnabled ||
            updaterRunning ||
            (pendingOperation.action === 'update' && !updateChecksPassed)
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
              <div className='font-medium'>{status?.message || t('Unknown')}</div>
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
            {status?.current_backup_id && status.current_backup_id !== 'latest' && (
              <div>
                <div className='text-muted-foreground'>{t('Backup ID')}</div>
                <div className='font-medium'>{status.current_backup_id}</div>
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
