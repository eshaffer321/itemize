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
import { ArrowPathIcon, PlayIcon, XMarkIcon } from '@heroicons/react/16/solid'
import { useEffect, useState } from 'react'

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

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, 'green' | 'red' | 'amber' | 'zinc' | 'blue'> = {
    completed: 'green',
    failed: 'red',
    running: 'blue',
    pending: 'amber',
    cancelled: 'zinc',
  }
  const color = colorMap[status] || 'zinc'
  return <Badge color={color}>{status}</Badge>
}

function ProviderBadge({ provider }: { provider: string }) {
  const colorMap: Record<string, 'blue' | 'red' | 'amber' | 'zinc'> = {
    walmart: 'blue',
    costco: 'red',
    amazon: 'amber',
  }
  const color = colorMap[provider] || 'zinc'
  return <Badge color={color}>{provider}</Badge>
}

function ProgressBar({ current, total }: { current: number; total: number }) {
  const percentage = total > 0 ? (current / total) * 100 : 0
  return (
    <div className="w-full rounded-full bg-zinc-200 dark:bg-zinc-700">
      <div
        className="rounded-full bg-blue-600 py-1 text-center text-xs font-medium leading-none text-white transition-all duration-500"
        style={{ width: `${Math.min(percentage, 100)}%` }}
      >
        {total > 0 && `${current}/${total}`}
      </div>
    </div>
  )
}

export default function SyncPage() {
  const [jobs, setJobs] = useState<SyncJob[]>([])
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Form state
  const [provider, setProvider] = useState<'walmart' | 'costco' | 'amazon'>('walmart')
  const [dryRun, setDryRun] = useState(true)
  const [lookbackDays, setLookbackDays] = useState(14)
  const [maxOrders, setMaxOrders] = useState<number | undefined>(undefined)
  const [verbose, setVerbose] = useState(false)
  const [force, setForce] = useState(false)
  const [orderId, setOrderId] = useState('')

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
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load sync jobs')
    } finally {
      setLoading(false)
    }
  }

  async function loadActiveJobs() {
    try {
      const data = await getActiveSyncJobs()
      // Update only active jobs to avoid flickering
      if (data.jobs.length > 0) {
        setJobs((prevJobs) => {
          const activeJobIds = new Set(data.jobs.map((j) => j.job_id))
          const inactiveJobs = prevJobs.filter((j) => !activeJobIds.has(j.job_id))
          return [...data.jobs, ...inactiveJobs]
        })
      }
    } catch (err) {
      // Silently fail on polling errors to avoid noise
    }
  }

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
      // Reset form to defaults
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
        <div className="mt-4 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm text-red-700 dark:text-red-400">{error}</p>
        </div>
      )}

      {success && (
        <div className="mt-4 rounded-lg bg-green-50 p-4 dark:bg-green-900/20">
          <p className="text-sm text-green-700 dark:text-green-400">{success}</p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="mt-8">
        <Fieldset>
          <Legend>Sync Configuration</Legend>

          <div className="grid grid-cols-1 gap-8 sm:grid-cols-2">
            <div>
              <Label>Provider</Label>
              <Select value={provider} onChange={(e) => setProvider(e.target.value as any)}>
                <option value="walmart">Walmart</option>
                <option value="costco">Costco</option>
                <option value="amazon">Amazon</option>
              </Select>
            </div>

            <div>
              <Label>Lookback Days</Label>
              <Input
                type="number"
                value={lookbackDays}
                onChange={(e) => setLookbackDays(parseInt(e.target.value))}
                min={1}
                max={365}
              />
            </div>

            <div>
              <Label>Max Orders (optional)</Label>
              <Input
                type="number"
                value={maxOrders || ''}
                onChange={(e) => setMaxOrders(e.target.value ? parseInt(e.target.value) : undefined)}
                min={1}
                placeholder="No limit"
              />
            </div>

            <div>
              <Label>Specific Order ID (optional)</Label>
              <Input
                type="text"
                value={orderId}
                onChange={(e) => setOrderId(e.target.value)}
                placeholder="Leave empty for all orders"
              />
            </div>
          </div>

          <Divider className="my-6" />

          <div className="space-y-4">
            <CheckboxField>
              <Checkbox checked={dryRun} onChange={(checked) => setDryRun(checked)} />
              <Label>Dry Run (preview changes without applying)</Label>
            </CheckboxField>

            <CheckboxField>
              <Checkbox checked={force} onChange={(checked) => setForce(checked)} />
              <Label>Force (reprocess already processed orders)</Label>
            </CheckboxField>

            <CheckboxField>
              <Checkbox checked={verbose} onChange={(checked) => setVerbose(checked)} />
              <Label>Verbose Logging</Label>
            </CheckboxField>
          </div>

          <Divider className="my-6" />

          <div className="flex justify-end gap-4">
            <Button type="submit" disabled={submitting}>
              <PlayIcon />
              {submitting ? 'Starting...' : 'Start Sync'}
            </Button>
          </div>
        </Fieldset>
      </form>

      <Divider className="my-10" />

      <div className="flex items-center justify-between">
        <Subheading>Sync Jobs</Subheading>
        <Button outline onClick={() => loadJobs()} disabled={loading}>
          <ArrowPathIcon className={loading ? 'animate-spin' : ''} />
          Refresh
        </Button>
      </div>

      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
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
                <StatusBadge status={job.status} />
                {job.dry_run && (
                  <Badge color="purple" className="ml-2">
                    Dry
                  </Badge>
                )}
              </TableCell>
              <TableCell>
                {job.status === 'running' ? (
                  <div className="space-y-1">
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
                  <Button plain onClick={() => handleCancel(job.job_id)}>
                    <XMarkIcon />
                    Cancel
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {jobs.length === 0 && !loading && (
        <div className="mt-8 text-center text-zinc-500 dark:text-zinc-400">
          <p className="text-sm">No sync jobs found. Start your first sync above.</p>
        </div>
      )}
    </>
  )
}
