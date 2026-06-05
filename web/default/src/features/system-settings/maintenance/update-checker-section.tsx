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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  getSystemUpdateStatus,
  precheckSystemUpdate,
  smokeSystemUpdate,
  startSystemUpdate,
  type SystemUpdatePrecheck,
  type SystemUpdateSmoke,
  type SystemUpdateStatus,
} from '../api'
import { SettingsSection } from '../components/settings-section'

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
  }, [])

  useEffect(() => {
    if (!updaterRunning) return
    const timer = window.setInterval(() => {
      void refreshStatus()
    }, 5000)
    return () => window.clearInterval(timer)
  }, [updaterRunning])

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

  const handleStartUpdate = async () => {
    setConfirmOpen(false)
    setLoading(true)
    try {
      const res = await startSystemUpdate()
      if (res.data) {
        setStatus(res.data)
      }
      if (res.success) {
        toast.success(t('Update task started.'))
      } else {
        toast.error(res.message || t('Failed to start update task'))
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Failed to start update task')
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

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
            onClick={() => setConfirmOpen(true)}
            disabled={
              !updaterEnabled || updaterRunning || loading || !updateChecksPassed
            }
          >
            {updaterRunning || loading ? (
              t('Updating...')
            ) : (
              <>
                <RocketIcon className='me-2 h-4 w-4' />
                {t('Combined update')}
              </>
            )}
          </Button>
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

        <ConfirmDialog
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          title={t('Start combined one-click update?')}
          desc={t(
            'This will update new-api, GPT-Load, and CLIProxyAPI together while preserving the Glart bridge and private sidecar ports.'
          )}
          confirmText={t('Start combined update')}
          isLoading={loading}
          disabled={!updaterEnabled || updaterRunning || !updateChecksPassed}
          handleConfirm={handleStartUpdate}
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

function HealthRow({ label, ok }: { label: string; ok: boolean }) {
  const { t } = useTranslation()
  return (
    <div>
      <div className='text-muted-foreground'>{label}</div>
      <div className='font-medium'>{ok ? t('OK') : t('Failed')}</div>
    </div>
  )
}
