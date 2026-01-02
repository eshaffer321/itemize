'use client'

import { Badge } from '@/components/badge'
import { Button } from '@/components/button'
import { DescriptionDetails, DescriptionList, DescriptionTerm } from '@/components/description-list'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { Link } from '@/components/link'
import { Text } from '@/components/text'
import { getSyncJob, cancelSyncJob, type SyncJob } from '@/lib/api'
import {
  ArrowLeftIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  ClockIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  XCircleIcon,
} from '@heroicons/react/16/solid'
import { useParams, useRouter } from 'next/navigation'
import { useCallback, useEffect, useState } from 'react'

function formatDate(dateString: string): string {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return 'Invalid date'
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short',
  })
}

function formatDuration(startedAt: string, completedAt?: string): string {
  const start = new Date(startedAt)
  const end = completedAt ? new Date(completedAt) : new Date()
  const diffMs = end.getTime() - start.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const mins = Math.floor(diffSecs / 60)
  const secs = diffSecs % 60

  if (mins > 0) {
    return `${mins}m ${secs}s`
  }
  return `${secs}s`
}

// Convert machine phase names to human-friendly display text
function formatPhase(phase: string, provider?: string): string {
  const providerName = provider
    ? provider.charAt(0).toUpperCase() + provider.slice(1)
    : 'provider'

  const phaseMap: Record<string, string> = {
    pending: 'Waiting to start...',
    initializing: 'Initializing...',
    fetching_orders: `Fetching orders from ${providerName}...`,
    processing_orders: 'Processing orders...',
    completed: 'Completed',
    failed: 'Failed',
  }

  return phaseMap[phase] || phase
}

function StatusIcon({ status }: { status: string }) {
  switch (status) {
    case 'completed':
      return <CheckCircleIcon className="h-6 w-6 text-green-500" />
    case 'failed':
      return <XCircleIcon className="h-6 w-6 text-red-500" />
    case 'running':
      return <ArrowPathIcon className="h-6 w-6 animate-spin text-cyan-500" />
    case 'pending':
      return <ClockIcon className="h-6 w-6 text-amber-500" />
    case 'cancelled':
      return <ExclamationCircleIcon className="h-6 w-6 text-zinc-500" />
    default:
      return null
  }
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
  return <Badge color={color}>{status}</Badge>
}

function ProviderBadge({ provider }: { provider: string }) {
  const colorMap: Record<string, 'blue' | 'rose' | 'orange' | 'zinc'> = {
    walmart: 'blue',
    costco: 'rose',
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
        className="rounded-full bg-cyan-600 py-2 shadow-sm transition-all duration-500"
        style={{ width: `${Math.max(Math.min(percentage, 100), percentage > 0 ? 5 : 0)}%` }}
      />
    </div>
  )
}

export default function SyncJobDetailPage() {
  const params = useParams()
  const router = useRouter()
  const jobId = Array.isArray(params.jobId) ? params.jobId[0] : (params.jobId as string)

  const [job, setJob] = useState<SyncJob | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadJob = useCallback(async () => {
    if (!jobId) return
    try {
      const data = await getSyncJob(jobId)
      setJob(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load job')
    } finally {
      setLoading(false)
    }
  }, [jobId])

  // Initial load
  useEffect(() => {
    if (!jobId) {
      setLoading(false)
      return
    }
    loadJob()
  }, [jobId, loadJob])

  // Auto-refresh for running jobs
  useEffect(() => {
    if (job?.status === 'running' || job?.status === 'pending') {
      const interval = setInterval(loadJob, 2000)
      return () => clearInterval(interval)
    }
  }, [job?.status, loadJob])

  // Validate jobId - must come after all hooks
  if (!jobId) {
    return (
      <>
        <div className="flex items-center gap-4">
          <Link href="/sync" className="text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
            <ArrowLeftIcon className="h-5 w-5" />
          </Link>
          <Heading>Invalid Job ID</Heading>
        </div>
        <Text className="mt-4">No job ID was provided.</Text>
      </>
    )
  }

  async function handleCancel() {
    const confirmed = window.confirm(
      'Are you sure you want to cancel this sync job? This action cannot be undone.'
    )
    if (!confirmed) return

    try {
      await cancelSyncJob(jobId)
      await loadJob()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel job')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <ArrowPathIcon className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    )
  }

  if (error && !job) {
    return (
      <>
        <div className="flex items-center gap-4">
          <Link href="/sync" className="text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
            <ArrowLeftIcon className="h-5 w-5" />
          </Link>
          <Heading>Sync Job Details</Heading>
        </div>
        <div className="mt-8 flex items-center gap-3 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <ExclamationTriangleIcon className="h-5 w-5 flex-shrink-0 text-red-600 dark:text-red-400" />
          <p className="text-sm text-red-700 dark:text-red-400">{error}</p>
        </div>
      </>
    )
  }

  if (!job) {
    return (
      <>
        <div className="flex items-center gap-4">
          <Link href="/sync" className="text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
            <ArrowLeftIcon className="h-5 w-5" />
          </Link>
          <Heading>Job Not Found</Heading>
        </div>
        <Text className="mt-4">The requested sync job could not be found.</Text>
      </>
    )
  }

  return (
    <>
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <Link href="/sync" className="text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
            <ArrowLeftIcon className="h-5 w-5" />
          </Link>
          <div>
            <Heading className="flex items-center gap-3">
              <StatusIcon status={job.status} />
              Sync Job
            </Heading>
            <Text className="font-mono text-sm text-zinc-500">{job.job_id}</Text>
          </div>
        </div>
        <div className="flex gap-2">
          <Button outline onClick={loadJob}>
            <ArrowPathIcon />
            Refresh
          </Button>
          {job.status === 'running' && (
            <Button color="red" onClick={handleCancel}>
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Status badges */}
      <div className="mt-6 flex flex-wrap items-center gap-3">
        <div className="flex gap-2">
          <StatusBadge status={job.status} />
          <ProviderBadge provider={job.provider} />
        </div>
        {job.dry_run && (
          <div className="rounded-lg border border-purple-200 bg-purple-50 px-3 py-1 dark:border-purple-800 dark:bg-purple-900/30">
            <Text className="text-sm font-semibold text-purple-700 dark:text-purple-300">
              Preview Mode - No changes will be saved
            </Text>
          </div>
        )}
      </div>

      {/* Running job progress */}
      {job.status === 'running' && (
        <div className="mt-6 rounded-lg border border-cyan-200 bg-cyan-50 p-4 dark:border-cyan-800 dark:bg-cyan-900/30">
          <Subheading>Progress</Subheading>
          <div className="mt-4 space-y-4">
            {/* Show different UI based on whether we know the total orders yet */}
            {job.progress.total_orders > 0 ? (
              <>
                <ProgressBar
                  current={job.progress.processed_orders}
                  total={job.progress.total_orders}
                />
                <div className="flex justify-between text-sm">
                  <Text>{formatPhase(job.progress.current_phase, job.provider)}</Text>
                  <Text>
                    {job.progress.processed_orders} of {job.progress.total_orders} orders
                  </Text>
                </div>
              </>
            ) : (
              <div className="flex items-center gap-3">
                <ArrowPathIcon className="h-5 w-5 animate-spin text-cyan-600" />
                <div>
                  <Text className="font-medium">{formatPhase(job.progress.current_phase, job.provider)}</Text>
                  <Text className="text-sm text-zinc-500">
                    Running for {formatDuration(job.started_at)}
                  </Text>
                </div>
              </div>
            )}
            {job.progress.skipped_orders > 0 && (
              <Text className="text-sm text-amber-600 dark:text-amber-400">
                {job.progress.skipped_orders} orders skipped (already processed)
              </Text>
            )}
            {job.progress.errored_orders > 0 && (
              <Text className="text-sm text-red-600 dark:text-red-400">
                {job.progress.errored_orders} orders with errors
              </Text>
            )}
          </div>
        </div>
      )}

      {/* Error message */}
      {job.error && (
        <div className="mt-6 flex items-start gap-3 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <ExclamationTriangleIcon className="h-5 w-5 flex-shrink-0 text-red-600 dark:text-red-400" />
          <div>
            <Text className="font-medium text-red-700 dark:text-red-400">Error</Text>
            <Text className="mt-1 text-sm text-red-600 dark:text-red-300">{job.error}</Text>
          </div>
        </div>
      )}

      <Divider className="my-8" />

      {/* Job details */}
      <Subheading>Job Details</Subheading>
      <DescriptionList className="mt-4">
        <DescriptionTerm>Job ID</DescriptionTerm>
        <DescriptionDetails className="font-mono">{job.job_id}</DescriptionDetails>

        <DescriptionTerm>Provider</DescriptionTerm>
        <DescriptionDetails className="capitalize">{job.provider}</DescriptionDetails>

        <DescriptionTerm>Mode</DescriptionTerm>
        <DescriptionDetails>
          {job.dry_run ? 'Dry Run (Preview Only)' : 'Live Sync'}
        </DescriptionDetails>

        <DescriptionTerm>Started</DescriptionTerm>
        <DescriptionDetails>{formatDate(job.started_at)}</DescriptionDetails>

        {job.completed_at && (
          <>
            <DescriptionTerm>Completed</DescriptionTerm>
            <DescriptionDetails>{formatDate(job.completed_at)}</DescriptionDetails>
          </>
        )}

        <DescriptionTerm>Duration</DescriptionTerm>
        <DescriptionDetails>{formatDuration(job.started_at, job.completed_at)}</DescriptionDetails>
      </DescriptionList>

      {/* Request configuration */}
      {job.request && (
        <>
          <Divider className="my-8" />
          <Subheading>Configuration</Subheading>
          <DescriptionList className="mt-4">
            <DescriptionTerm>Lookback Days</DescriptionTerm>
            <DescriptionDetails>{job.request.lookback_days || 14}</DescriptionDetails>

            {job.request.max_orders && (
              <>
                <DescriptionTerm>Max Orders</DescriptionTerm>
                <DescriptionDetails>{job.request.max_orders}</DescriptionDetails>
              </>
            )}

            {job.request.order_id && (
              <>
                <DescriptionTerm>Specific Order</DescriptionTerm>
                <DescriptionDetails className="font-mono">{job.request.order_id}</DescriptionDetails>
              </>
            )}

            {job.request.force && (
              <>
                <DescriptionTerm>Force Re-sync</DescriptionTerm>
                <DescriptionDetails>Yes</DescriptionDetails>
              </>
            )}

            {job.request.verbose && (
              <>
                <DescriptionTerm>Verbose Logging</DescriptionTerm>
                <DescriptionDetails>Enabled</DescriptionDetails>
              </>
            )}
          </DescriptionList>
        </>
      )}

      {/* Results */}
      {job.result && (
        <>
          <Divider className="my-8" />
          <Subheading>Results</Subheading>
          <DescriptionList className="mt-4">
            <DescriptionTerm>Orders Found</DescriptionTerm>
            <DescriptionDetails>{job.progress.total_orders}</DescriptionDetails>

            <DescriptionTerm>Orders Processed</DescriptionTerm>
            <DescriptionDetails>{job.result.processed_count}</DescriptionDetails>

            <DescriptionTerm>Orders Skipped</DescriptionTerm>
            <DescriptionDetails>
              {job.result.skipped_count}
              {job.result.skipped_count > 0 && (
                <span className="ml-2 text-sm text-zinc-500">(already processed)</span>
              )}
            </DescriptionDetails>

            <DescriptionTerm>Orders with Errors</DescriptionTerm>
            <DescriptionDetails>
              {job.result.error_count > 0 ? (
                <span className="text-red-600 dark:text-red-400">{job.result.error_count}</span>
              ) : (
                <span className="text-green-600 dark:text-green-400">0</span>
              )}
            </DescriptionDetails>
          </DescriptionList>
        </>
      )}

      {/* Back button */}
      <div className="mt-8">
        <Button outline onClick={() => router.push('/sync')}>
          <ArrowLeftIcon />
          Back to Sync
        </Button>
      </div>
    </>
  )
}
