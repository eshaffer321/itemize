import { Badge } from '@/components/badge'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { ProviderSpendingChart } from '@/components/provider-spending-chart'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { getOrders, getOrderStats, getSyncRuns, getStats } from '@/lib/api'
import type { Order, SyncRun, StatsResponse } from '@/lib/api'

function Stat({
  title,
  value,
  color,
}: {
  title: string
  value: string | number
  color?: 'green' | 'red' | 'blue' | 'brand' | 'default'
}) {
  const colorClasses = {
    green: 'text-green-600 dark:text-green-400',
    red: 'text-red-600 dark:text-red-400',
    blue: 'text-blue-600 dark:text-blue-400',
    brand: 'text-brand-600 dark:text-brand-400',
    default: '',
  }
  const valueColor = color ? colorClasses[color] : ''

  return (
    <div>
      <Divider />
      <div className="mt-6 text-lg/6 font-medium sm:text-sm/6">{title}</div>
      <div className={`mt-3 text-3xl/8 font-semibold sm:text-2xl/8 ${valueColor}`}>{value}</div>
    </div>
  )
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
  }).format(amount)
}

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, 'green' | 'red' | 'amber' | 'zinc' | 'blue'> = {
    success: 'green',
    failed: 'red',
    'dry-run': 'blue',
    pending: 'amber',
    completed: 'green',
    completed_with_errors: 'amber',
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

export default async function Dashboard() {
  let ordersData
  let runsData
  let stats
  let fullStats: StatsResponse | null = null

  try {
    stats = await getOrderStats()
  } catch {
    stats = { total: 0, success: 0, failed: 0, dryRun: 0, totalAmount: 0 }
  }

  try {
    fullStats = await getStats()
  } catch {
    fullStats = null
  }

  try {
    ordersData = await getOrders({ limit: 10 })
  } catch {
    ordersData = { orders: [], total_count: 0, limit: 10, offset: 0 }
  }

  try {
    runsData = await getSyncRuns()
  } catch {
    runsData = { runs: [], count: 0 }
  }

  const apiDown = stats.total === 0 && runsData.count === 0

  return (
    <>
      <Heading>Dashboard</Heading>

      {apiDown && (
        <div className="mt-4 rounded-lg bg-amber-50 p-4 dark:bg-amber-900/20">
          <p className="text-sm text-amber-700 dark:text-amber-400">
            No data loaded. Make sure the API server is running on port 8085.
          </p>
        </div>
      )}

      <div className="mt-8 grid gap-8 sm:grid-cols-2 xl:grid-cols-5">
        <Stat title="Total Orders" value={stats.total} />
        <Stat title="Successful" value={stats.success} color="green" />
        <Stat title="Failed" value={stats.failed} color="red" />
        <Stat title="Dry Runs" value={stats.dryRun} color="blue" />
        <Stat title="Total Synced" value={formatCurrency(stats.totalAmount)} />
      </div>

      {fullStats && fullStats.provider_stats && fullStats.provider_stats.length > 0 && (
        <div className="mt-10">
          <Subheading>Spending by Provider</Subheading>
          <div className="mt-4 rounded-lg border border-zinc-200 p-6 dark:border-zinc-700">
            <ProviderSpendingChart stats={fullStats.provider_stats} />
          </div>
        </div>
      )}

      <Subheading className="mt-14">Recent Orders</Subheading>
      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader>Order ID</TableHeader>
            <TableHeader>Provider</TableHeader>
            <TableHeader>Date</TableHeader>
            <TableHeader>Status</TableHeader>
            <TableHeader className="text-right">Total</TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {ordersData.orders.slice(0, 5).map((order: Order) => (
            <TableRow key={order.order_id} href={`/orders/${order.order_id}`} title={`Order ${order.order_id}`}>
              <TableCell className="font-medium">{order.order_id}</TableCell>
              <TableCell>
                <ProviderBadge provider={order.provider} />
              </TableCell>
              <TableCell className="text-zinc-500">{formatDate(order.order_date)}</TableCell>
              <TableCell>
                <StatusBadge status={order.status} />
              </TableCell>
              <TableCell className="text-right">{formatCurrency(order.order_total)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {ordersData.orders.length === 0 && (
        <div className="mt-4 text-center text-zinc-500 dark:text-zinc-400">
          <p className="text-sm">No orders found. Run a sync to get started.</p>
        </div>
      )}

      <Subheading className="mt-14">Recent Sync Runs</Subheading>
      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader>Run</TableHeader>
            <TableHeader>Provider</TableHeader>
            <TableHeader>Started</TableHeader>
            <TableHeader>Status</TableHeader>
            <TableHeader className="text-right">Processed</TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {runsData.runs.slice(0, 5).map((run: SyncRun) => (
            <TableRow key={run.id}>
              <TableCell className="font-medium">
                #{run.id}
                {run.dry_run && (
                  <Badge color="purple" className="ml-2">
                    Dry
                  </Badge>
                )}
              </TableCell>
              <TableCell>
                <ProviderBadge provider={run.provider} />
              </TableCell>
              <TableCell className="text-zinc-500">{formatDate(run.started_at)}</TableCell>
              <TableCell>
                <StatusBadge status={run.status} />
              </TableCell>
              <TableCell className="text-right">{run.orders_processed}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {runsData.runs.length === 0 && (
        <div className="mt-4 text-center text-zinc-500 dark:text-zinc-400">
          <p className="text-sm">No sync runs found. Run your first sync from the CLI.</p>
        </div>
      )}
    </>
  )
}
