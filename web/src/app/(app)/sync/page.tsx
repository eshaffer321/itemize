'use client'

import { Badge } from '@/components/badge'
import { Button } from '@/components/button'
import { Checkbox, CheckboxField } from '@/components/checkbox'
import { Divider } from '@/components/divider'
import { Fieldset, Label, Legend } from '@/components/fieldset'
import { Heading, Subheading } from '@/components/heading'
import { Input } from '@/components/input'
import { Select } from '@/components/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { Text } from '@/components/text'
import {
  cancelSyncJob,
  getActiveSyncJobs,
  getSyncJobs,
  startSync,
  type SyncJob,
  type StartSyncRequest,
} from '@/lib/api'
import {
  ArrowPathIcon,
  ExclamationTriangleIcon,
  InformationCircleIcon,
  PlayIcon,
  ShieldCheckIcon,
  XMarkIcon,
} from '@heroicons/react/16/solid'
import { useCallback, useEffect, useState } from 'react'

// Helper text component for form fields
function HelpText({ children }: { children: React.ReactNode }) {
  return <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">{children}</p>
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  return `${diffDays}d ago`
}

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, 'green' | 'red' | 'amber' | 'zinc' | 'cyan'> = {
    completed: 'green',
    failed: 'red',
    running: 'cyan',
    pending: 'amber',
    cancelled: 'zinc',
  }
  const color = colorMap[status] || 'zinc'
  return (
    <Badge color={color} aria-label={`Status: ${status}`}>
      {status}
    </Badge>
  )
}

function ProviderBadge({ provider }: { provider: string }) {
  const colorMap: Record<string, 'blue' | 'red' | 'orange' | 'zinc'> = {
    walmart: 'blue',
    costco: 'red',
    amazon: 'orange',
  }
  const color = colorMap[provider] || 'zinc'
  return <Badge color={color}>{provider}</Badge>
}

function ProgressBar({ current, total }: { current: number; total: number }) {
  const percentage = total > 0 ? (current / total) * 100 : 0
  return (
    <div className="w-full rounded-full bg-zinc-100 shadow-inner dark:bg-zinc-800">
      <div
        className="rounded-full bg-cyan-600 py-1 text-center text-xs font-medium leading-none text-white shadow-sm transition-all duration-500"
        style={{ width: `${Math.max(Math.min(percentage, 100), percentage > 0 ? 10 : 0)}%` }}
      >
        {total > 0 && `${current}/${total}`}
      </div>
    </div>
  )
}

// Mobile-friendly job card component
function JobCard({ job, onCancel }: { job: SyncJob; onCancel: (jobId: string) => void }) {
  return (
    <div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2">
          <ProviderBadge provider={job.provider} />
          <StatusBadge status={job.status} />
          {job.dry_run && (
            <Badge color="purple">Dry</Badge>
          )}
        </div>
        {job.status === 'running' && (
          <Button plain onClick={() => onCancel(job.job_id)} className="text-red-600">
            <XMarkIcon className="h-4 w-4" />
          </Button>
        )}
      </div>
      <div className="mt-3">
        <Text className="font-mono text-xs text-zinc-500">{job.job_id.substring(0, 8)}</Text>
        <Text className="text-xs text-zinc-500">{formatRelativeTime(job.started_at)}</Text>
      </div>
      {job.status === 'running' && (
        <div className="mt-3 space-y-1">
          <ProgressBar current={job.progress.processed_orders} total={job.progress.total_orders} />
          <Text className="text-xs text-zinc-500">
            {job.progress.current_phase}
            {job.progress.errored_orders > 0 && ` (${job.progress.errored_orders} errors)`}
          </Text>
        </div>
      )}
      {job.status !== 'running' && job.result && (
        <div className="mt-3">
          <Text className="text-sm">
            {job.result.orders_processed} / {job.result.orders_found} orders
            {job.result.orders_errored > 0 && ` (${job.result.orders_errored} errors)`}
          </Text>
        </div>
      )}
    </div>
  )
}

export default function SyncPage() {
  const [jobs, setJobs] = useState<SyncJob[]>([])
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)

  // Form state - load provider from localStorage
  const [provider, setProvider] = useState<'walmart' | 'costco' | 'amazon'>(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('sync_provider')
      if (saved === 'walmart' || saved === 'costco' || saved === 'amazon') {
        return saved
      }
    }
    return 'walmart'
  })
  const [dryRun, setDryRun] = useState(true)
  const [lookbackDays, setLookbackDays] = useState(14)
  const [maxOrders, setMaxOrders] = useState<number | undefined>(undefined)
  const [verbose, setVerbose] = useState(false)
  const [force, setForce] = useState(false)
  const [orderId, setOrderId] = useState('')

  // Save provider preference to localStorage
  useEffect(() => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('sync_provider', provider)
    }
  }, [provider])

  // Auto-dismiss success/error messages after 5 seconds
  useEffect(() => {
    if (success || error) {
      const timer = setTimeout(() => {
        setSuccess(null)
        setError(null)
      }, 5000)
      return () => clearTimeout(timer)
    }
  }, [success, error])

  // Auto-refresh active jobs
  useEffect(() => {
    loadJobs()
    const interval = setInterval(() => {
      loadActiveJobs()
    }, 3000) // Poll every 3 seconds
    return () => clearInterval(interval)
  }, [])

  async function loadJobs() {
    try {
      setLoading(true)
      const data = await getSyncJobs()
      setJobs(data.jobs)
      setLastUpdated(new Date())
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load sync jobs')
    } finally {
      setLoading(false)
    }
  }

  const loadActiveJobs = useCallback(async () => {
    try {
      const data = await getActiveSyncJobs()
      setLastUpdated(new Date())
      // Update only active jobs to avoid flickering
      if (data.jobs.length > 0) {
        setJobs((prevJobs) => {
          const activeJobIds = new Set(data.jobs.map((j) => j.job_id))
          const inactiveJobs = prevJobs.filter((j) => !activeJobIds.has(j.job_id))
          return [...data.jobs, ...inactiveJobs]
        })
      }
    } catch {
      // Silently fail on polling errors to avoid noise
    }
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    setSuccess(null)

    const request: StartSyncRequest = {
      provider,
      dry_run: dryRun,
      lookback_days: lookbackDays,
      max_orders: maxOrders,
      verbose,
      force,
      order_id: orderId || undefined,
    }

    try {
      const response = await startSync(request)
      setSuccess(response.message)
      // Reload jobs to show the new one
      await loadJobs()
      // Reset form to defaults (keep provider)
      setDryRun(true)
      setForce(false)
      setOrderId('')
      setMaxOrders(undefined)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start sync')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleCancel(jobId: string) {
    const confirmed = window.confirm(
      'Are you sure you want to cancel this sync job? This action cannot be undone.'
    )
    if (!confirmed) {
      return
    }

    try {
      await cancelSyncJob(jobId)
      setSuccess('Job cancelled successfully')
      await loadJobs()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel job')
    }
  }

  return (
    <>
      <Heading>Sync</Heading>
      <Text>Start a new sync job to import orders from your providers.</Text>

      {error && (
        <div className="mt-4 flex items-start gap-3 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <ExclamationTriangleIcon className="h-5 w-5 flex-shrink-0 text-red-600 dark:text-red-400" />
          <p className="text-sm text-red-700 dark:text-red-400">{error}</p>
        </div>
      )}

      {success && (
        <div className="mt-4 flex items-start gap-3 rounded-lg bg-green-50 p-4 dark:bg-green-900/20">
          <ShieldCheckIcon className="h-5 w-5 flex-shrink-0 text-green-600 dark:text-green-400" />
          <p className="text-sm text-green-700 dark:text-green-400">{success}</p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="mt-8">
        <Fieldset>
          <Legend>Sync Configuration</Legend>

          {/* Prominent Dry Run Toggle */}
          <div
            className={`mb-6 rounded-lg border-2 p-4 ${
              dryRun
                ? 'border-cyan-200 bg-cyan-50 dark:border-cyan-800 dark:bg-cyan-900/20'
                : 'border-amber-300 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20'
            }`}
          >
            <CheckboxField>
              <Checkbox checked={dryRun} onChange={(checked) => setDryRun(checked)} />
              <Label className="font-semibold">
                {dryRun ? (
                  <span className="flex items-center gap-2">
                    <ShieldCheckIcon className="h-4 w-4 text-cyan-600" />
                    Dry Run Mode (Safe Preview)
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    <ExclamationTriangleIcon className="h-4 w-4 text-amber-600" />
                    Live Sync Mode
                  </span>
                )}
              </Label>
            </CheckboxField>
            <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
              {dryRun
                ? 'Changes will NOT be saved to Monarch Money. Use this to preview what would happen.'
                : 'Changes WILL be saved to Monarch Money. Transaction splits will be created.'}
            </p>
          </div>

          {/* Main form grid - 4 columns on large screens */}
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <Label>Provider</Label>
              <Select value={provider} onChange={(e) => setProvider(e.target.value as 'walmart' | 'costco' | 'amazon')}>
                <option value="walmart">Walmart</option>
                <option value="costco">Costco</option>
                <option value="amazon">Amazon</option>
              </Select>
              <HelpText>Your order history will be fetched from this provider</HelpText>
            </div>

            <div>
              <Label>Lookback Days</Label>
              <Input
                type="number"
                value={lookbackDays}
                onChange={(e) => setLookbackDays(parseInt(e.target.value) || 14)}
                min={1}
                max={365}
              />
              <HelpText>Import orders from the past X days (1-365)</HelpText>
            </div>

            <div>
              <Label>
                Max Orders
                <span className="ml-1 text-zinc-400">(optional)</span>
              </Label>
              <Input
                type="number"
                value={maxOrders || ''}
                onChange={(e) => setMaxOrders(e.target.value ? parseInt(e.target.value) : undefined)}
                min={1}
                placeholder="No limit"
              />
              <HelpText>Limit number of orders to process</HelpText>
            </div>

            <div>
              <Label>
                Order ID
                <span className="ml-1 text-zinc-400">(optional)</span>
              </Label>
              <Input
                type="text"
                value={orderId}
                onChange={(e) => setOrderId(e.target.value)}
                placeholder="Specific order"
              />
              <HelpText>Process only this order ID</HelpText>
            </div>
          </div>

          {/* Advanced Options Toggle */}
          <div className="mt-6">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-200"
            >
              <InformationCircleIcon className="h-4 w-4" />
              {showAdvanced ? 'Hide' : 'Show'} Advanced Options
            </button>

            {showAdvanced && (
              <div className="mt-4 space-y-4 rounded-lg bg-zinc-50 p-4 dark:bg-zinc-800/50">
                <CheckboxField>
                  <Checkbox checked={force} onChange={(checked) => setForce(checked)} />
                  <Label>
                    Force Re-sync
                    <span className="ml-2 text-xs font-normal text-zinc-500">
                      Reprocess orders that were already synced
                    </span>
                  </Label>
                </CheckboxField>

                <CheckboxField>
                  <Checkbox checked={verbose} onChange={(checked) => setVerbose(checked)} />
                  <Label>
                    Debug Logging
                    <span className="ml-2 text-xs font-normal text-zinc-500">
                      Capture detailed logs on the server
                    </span>
                  </Label>
                </CheckboxField>
              </div>
            )}
          </div>

          <Divider className="my-6" />

          <div className="flex flex-col gap-4 sm:flex-row sm:justify-end">
            <Button type="submit" disabled={submitting} className="w-full sm:w-auto">
              {submitting ? (
                <>
                  <ArrowPathIcon className="animate-spin" />
                  Starting...
                </>
              ) : (
                <>
                  <PlayIcon />
                  Start Sync
                </>
              )}
            </Button>
          </div>
        </Fieldset>
      </form>

      <Divider className="my-10" />

      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <Subheading>Sync Jobs</Subheading>
          {lastUpdated && (
            <Text className="text-xs text-zinc-500">
              Updated {formatRelativeTime(lastUpdated.toISOString())}
            </Text>
          )}
        </div>
        <Button outline onClick={() => loadJobs()} disabled={loading}>
          <ArrowPathIcon className={loading ? 'animate-spin' : ''} />
          Refresh
        </Button>
      </div>

      {/* Desktop Table View */}
      <div className="hidden lg:block">
        <Table className="mt-4 [--gutter:--spacing(4)] lg:[--gutter:--spacing(6)]">
          <TableHead>
            <TableRow>
              <TableHeader>Job ID</TableHeader>
              <TableHeader>Provider</TableHeader>
              <TableHeader>Status</TableHeader>
              <TableHeader>Progress</TableHeader>
              <TableHeader>Started</TableHeader>
              <TableHeader>Actions</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {jobs.map((job) => (
              <TableRow key={job.job_id}>
                <TableCell className="font-mono text-xs">{job.job_id.substring(0, 8)}</TableCell>
                <TableCell>
                  <ProviderBadge provider={job.provider} />
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={job.status} />
                    {job.dry_run && (
                      <Badge color="purple">Dry</Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  {job.status === 'running' ? (
                    <div className="w-40 space-y-1">
                      <ProgressBar current={job.progress.processed_orders} total={job.progress.total_orders} />
                      <Text className="text-xs text-zinc-500">
                        {job.progress.current_phase}
                        {job.progress.errored_orders > 0 && ` (${job.progress.errored_orders} errors)`}
                      </Text>
                    </div>
                  ) : job.result ? (
                    <Text className="text-sm">
                      {job.result.orders_processed} / {job.result.orders_found}
                      {job.result.orders_errored > 0 && ` (${job.result.orders_errored} errors)`}
                    </Text>
                  ) : (
                    <Text className="text-sm text-zinc-500">-</Text>
                  )}
                </TableCell>
                <TableCell className="text-zinc-500">{formatDate(job.started_at)}</TableCell>
                <TableCell>
                  {job.status === 'running' && (
                    <Button color="red" plain onClick={() => handleCancel(job.job_id)}>
                      <XMarkIcon />
                      Cancel
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Mobile Card View */}
      <div className="mt-4 space-y-4 lg:hidden">
        {jobs.map((job) => (
          <JobCard key={job.job_id} job={job} onCancel={handleCancel} />
        ))}
      </div>

      {jobs.length === 0 && !loading && (
        <div className="mt-8 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-zinc-100 dark:bg-zinc-800">
            <ArrowPathIcon className="h-6 w-6 text-zinc-400" />
          </div>
          <Text className="text-zinc-500 dark:text-zinc-400">No sync jobs found yet.</Text>
          <Text className="mt-1 text-sm text-zinc-400 dark:text-zinc-500">
            Configure your sync settings above and click Start Sync.
          </Text>
        </div>
      )}
    </>
  )
}
