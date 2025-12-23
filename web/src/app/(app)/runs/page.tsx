import { Badge } from '@/components/badge'
import { Heading } from '@/components/heading'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { getSyncRuns } from '@/lib/api'
import type { SyncRun } from '@/lib/api'
import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Sync Runs',
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function formatDuration(startedAt: string, completedAt?: string): string {
  if (!completedAt) return 'Running...'
  const start = new Date(startedAt).getTime()
  const end = new Date(completedAt).getTime()
  const durationMs = end - start
  if (durationMs < 1000) return `${durationMs}ms`
  if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`
  return `${Math.floor(durationMs / 60000)}m ${Math.floor((durationMs % 60000) / 1000)}s`
}

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, 'green' | 'red' | 'amber' | 'zinc' | 'blue' | 'yellow'> = {
    completed: 'green',
    completed_with_errors: 'amber',
    failed: 'red',
    running: 'blue',
  }
  const color = colorMap[status] || 'zinc'
  const displayStatus = status.replace(/_/g, ' ')
  return <Badge color={color}>{displayStatus}</Badge>
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

export default async function SyncRunsPage() {
  let data
  try {
    data = await getSyncRuns()
  } catch (error) {
    return (
      <>
        <Heading>Sync Runs</Heading>
        <div className="mt-8 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm text-red-700 dark:text-red-400">
            Failed to load sync runs. Make sure the API server is running on port 8080.
          </p>
        </div>
      </>
    )
  }

  return (
    <>
      <Heading>Sync Runs</Heading>

      <p className="mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        {data.count} total sync runs
      </p>

      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader>Run ID</TableHeader>
            <TableHeader>Provider</TableHeader>
            <TableHeader>Started</TableHeader>
            <TableHeader>Duration</TableHeader>
            <TableHeader>Status</TableHeader>
            <TableHeader className="text-right">Found</TableHeader>
            <TableHeader className="text-right">Processed</TableHeader>
            <TableHeader className="text-right">Skipped</TableHeader>
            <TableHeader className="text-right">Errors</TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {data.runs.map((run: SyncRun) => (
            <TableRow key={run.id}>
              <TableCell className="font-medium">
                #{run.id}
                {run.dry_run && (
                  <Badge color="purple" className="ml-2">
                    Dry Run
                  </Badge>
                )}
              </TableCell>
              <TableCell>
                <ProviderBadge provider={run.provider} />
              </TableCell>
              <TableCell className="text-zinc-500">{formatDate(run.started_at)}</TableCell>
              <TableCell className="text-zinc-500">{formatDuration(run.started_at, run.completed_at)}</TableCell>
              <TableCell>
                <StatusBadge status={run.status} />
              </TableCell>
              <TableCell className="text-right">{run.orders_found}</TableCell>
              <TableCell className="text-right">{run.orders_processed}</TableCell>
              <TableCell className="text-right">{run.orders_skipped}</TableCell>
              <TableCell className="text-right">
                {run.orders_errored > 0 ? (
                  <span className="text-red-600 dark:text-red-400">{run.orders_errored}</span>
                ) : (
                  run.orders_errored
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {data.runs.length === 0 && (
        <div className="mt-8 text-center text-zinc-500 dark:text-zinc-400">
          <p>No sync runs found.</p>
          <p className="mt-2 text-sm">Run a sync from the CLI to see results here.</p>
        </div>
      )}
    </>
  )
}
