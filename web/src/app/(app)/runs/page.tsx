import { Heading } from '@/components/heading'
import { getSyncRuns } from '@/lib/api'
import type { Metadata } from 'next'
import { SyncRunsTable } from './sync-runs-table'

export const metadata: Metadata = {
  title: 'Sync Runs',
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

      <SyncRunsTable runs={data.runs} />

      {data.runs.length === 0 && (
        <div className="mt-8 text-center text-zinc-500 dark:text-zinc-400">
          <p>No sync runs found.</p>
          <p className="mt-2 text-sm">Run a sync from the CLI to see results here.</p>
        </div>
      )}
    </>
  )
}
