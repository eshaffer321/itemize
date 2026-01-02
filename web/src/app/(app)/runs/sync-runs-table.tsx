'use client'

import { Badge } from '@/components/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  SortableTableHeader,
  useTableSort,
  sortData,
  type SortConfig,
} from '@/components/table'
import type { SyncRun } from '@/lib/api'
import { useMemo } from 'react'

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

type SyncRunSortKey = 'id' | 'provider' | 'started_at' | 'status' | 'orders_found' | 'orders_processed' | 'orders_skipped' | 'orders_errored'

function getSyncRunValue(run: SyncRun, key: string): unknown {
  switch (key) {
    case 'id':
      return run.id
    case 'provider':
      return run.provider
    case 'started_at':
      return new Date(run.started_at)
    case 'status':
      return run.status
    case 'orders_found':
      return run.orders_found
    case 'orders_processed':
      return run.orders_processed
    case 'orders_skipped':
      return run.orders_skipped
    case 'orders_errored':
      return run.orders_errored
    default:
      return null
  }
}

interface SyncRunsTableProps {
  runs: SyncRun[]
}

export function SyncRunsTable({ runs }: SyncRunsTableProps) {
  const { sortConfig, handleSort } = useTableSort<SyncRunSortKey>('started_at', 'desc')

  const sortedRuns = useMemo(() => {
    return sortData(runs, sortConfig as SortConfig<string>, getSyncRunValue)
  }, [runs, sortConfig])

  const onSort = handleSort as (key: string) => void

  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <SortableTableHeader sortKey="id" currentSort={sortConfig} onSort={onSort}>
            Run ID
          </SortableTableHeader>
          <SortableTableHeader sortKey="provider" currentSort={sortConfig} onSort={onSort}>
            Provider
          </SortableTableHeader>
          <SortableTableHeader sortKey="started_at" currentSort={sortConfig} onSort={onSort}>
            Started
          </SortableTableHeader>
          <SortableTableHeader sortKey="status" currentSort={sortConfig} onSort={onSort}>
            Status
          </SortableTableHeader>
          <SortableTableHeader sortKey="orders_found" currentSort={sortConfig} onSort={onSort} className="text-right">
            Found
          </SortableTableHeader>
          <SortableTableHeader sortKey="orders_processed" currentSort={sortConfig} onSort={onSort} className="text-right">
            Processed
          </SortableTableHeader>
          <SortableTableHeader sortKey="orders_skipped" currentSort={sortConfig} onSort={onSort} className="text-right">
            Skipped
          </SortableTableHeader>
          <SortableTableHeader sortKey="orders_errored" currentSort={sortConfig} onSort={onSort} className="text-right">
            Errors
          </SortableTableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {sortedRuns.map((run: SyncRun) => (
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
            <TableCell className="text-zinc-500">
              <div>{formatDate(run.started_at)}</div>
              <div className="text-xs text-zinc-400">{formatDuration(run.started_at, run.completed_at)}</div>
            </TableCell>
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
  )
}
