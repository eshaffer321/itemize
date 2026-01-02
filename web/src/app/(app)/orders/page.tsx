import { Badge } from '@/components/badge'
import { Button } from '@/components/button'
import { ConfidenceBadge } from '@/components/confidence-badge'
import { Heading } from '@/components/heading'
import { Input, InputGroup } from '@/components/input'
import { Select } from '@/components/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { getOrders } from '@/lib/api'
import type { Order } from '@/lib/api'
import { ChevronLeftIcon, ChevronRightIcon, MagnifyingGlassIcon } from '@heroicons/react/16/solid'
import type { Metadata } from 'next'
import Link from 'next/link'

export const metadata: Metadata = {
  title: 'Orders',
}

const PAGE_SIZE = 25

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
  }
  const color = colorMap[status] || 'zinc'
  return <Badge color={color}>{status}</Badge>
}

function ProviderBadge({ provider }: { provider: string }) {
  const colorMap: Record<string, 'blue' | 'zinc' | 'amber'> = {
    walmart: 'blue',
    costco: 'zinc',
    amazon: 'amber',
  }
  const color = colorMap[provider] || 'zinc'
  return <Badge color={color}>{provider}</Badge>
}

interface PageProps {
  searchParams: Promise<{
    page?: string
    provider?: string
    status?: string
    search?: string
    days?: string
  }>
}

export default async function OrdersPage({ searchParams }: PageProps) {
  const params = await searchParams
  const currentPage = Math.max(1, parseInt(params.page || '1', 10))
  const provider = params.provider || ''
  const status = params.status || ''
  const search = params.search || ''
  const days = params.days || ''
  const daysBack = days ? parseInt(days, 10) : undefined
  const offset = (currentPage - 1) * PAGE_SIZE

  let data
  try {
    data = await getOrders({
      limit: PAGE_SIZE,
      offset,
      provider: provider || undefined,
      status: status || undefined,
      search: search || undefined,
      days_back: daysBack,
    })
  } catch (error) {
    return (
      <>
        <Heading>Orders</Heading>
        <div className="mt-8 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm text-red-700 dark:text-red-400">
            Failed to load orders. Make sure the API server is running on port 8085.
          </p>
        </div>
      </>
    )
  }

  const totalPages = Math.ceil(data.total_count / PAGE_SIZE)
  const hasNextPage = currentPage < totalPages
  const hasPrevPage = currentPage > 1

  // Build query string for pagination links
  function buildUrl(page: number) {
    const params = new URLSearchParams()
    params.set('page', page.toString())
    if (provider) params.set('provider', provider)
    if (status) params.set('status', status)
    if (search) params.set('search', search)
    if (days) params.set('days', days)
    return `/orders?${params.toString()}`
  }

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Orders</Heading>
        <div className="flex gap-4">
          <form className="flex flex-wrap items-center gap-4">
            <InputGroup>
              <MagnifyingGlassIcon />
              <Input name="search" placeholder="Search order ID..." defaultValue={search} className="w-48" />
            </InputGroup>
            <Select name="days" defaultValue={days}>
              <option value="">All Time</option>
              <option value="7">Last 7 Days</option>
              <option value="30">Last 30 Days</option>
              <option value="90">Last 90 Days</option>
              <option value="365">Last Year</option>
            </Select>
            <Select name="provider" defaultValue={provider}>
              <option value="">All Providers</option>
              <option value="walmart">Walmart</option>
              <option value="costco">Costco</option>
              <option value="amazon">Amazon</option>
            </Select>
            <Select name="status" defaultValue={status}>
              <option value="">All Statuses</option>
              <option value="success">Success</option>
              <option value="failed">Failed</option>
              <option value="pending">Pending</option>
              <option value="dry-run">Dry Run</option>
            </Select>
            <Button type="submit">Filter</Button>
          </form>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between">
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          Showing {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total_count)} of {data.total_count} orders
        </p>
        <div className="flex items-center gap-2">
          {hasPrevPage ? (
            <Link
              href={buildUrl(currentPage - 1)}
              className="inline-flex items-center gap-1 rounded-md bg-zinc-100 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700"
            >
              <ChevronLeftIcon className="size-4" />
              Previous
            </Link>
          ) : (
            <span className="inline-flex items-center gap-1 rounded-md bg-zinc-50 px-3 py-1.5 text-sm font-medium text-zinc-400 dark:bg-zinc-900 dark:text-zinc-600">
              <ChevronLeftIcon className="size-4" />
              Previous
            </span>
          )}
          <span className="px-2 text-sm text-zinc-600 dark:text-zinc-400">
            Page {currentPage} of {totalPages}
          </span>
          {hasNextPage ? (
            <Link
              href={buildUrl(currentPage + 1)}
              className="inline-flex items-center gap-1 rounded-md bg-zinc-100 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700"
            >
              Next
              <ChevronRightIcon className="size-4" />
            </Link>
          ) : (
            <span className="inline-flex items-center gap-1 rounded-md bg-zinc-50 px-3 py-1.5 text-sm font-medium text-zinc-400 dark:bg-zinc-900 dark:text-zinc-600">
              Next
              <ChevronRightIcon className="size-4" />
            </span>
          )}
        </div>
      </div>

      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader>Order ID</TableHeader>
            <TableHeader>Provider</TableHeader>
            <TableHeader>Date</TableHeader>
            <TableHeader>Status</TableHeader>
            <TableHeader>Confidence</TableHeader>
            <TableHeader className="text-right">Total</TableHeader>
            <TableHeader className="text-right">Items</TableHeader>
            <TableHeader className="text-right">Splits</TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {data.orders.map((order: Order) => (
            <TableRow key={order.order_id} href={`/orders/${order.order_id}`} title={`View order ${order.order_id}`}>
              <TableCell className="font-medium">{order.order_id}</TableCell>
              <TableCell>
                <ProviderBadge provider={order.provider} />
              </TableCell>
              <TableCell className="text-zinc-500">{formatDate(order.order_date)}</TableCell>
              <TableCell>
                <StatusBadge status={order.status} />
              </TableCell>
              <TableCell>
                <ConfidenceBadge confidence={order.match_confidence} />
              </TableCell>
              <TableCell className="text-right">{formatCurrency(order.order_total)}</TableCell>
              <TableCell className="text-right">{order.item_count}</TableCell>
              <TableCell className="text-right">{order.split_count}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {data.orders.length === 0 && (
        <div className="mt-8 text-center text-zinc-500 dark:text-zinc-400">
          <p>No orders found.</p>
          <p className="mt-2 text-sm">
            {provider || status || search || days
              ? 'Try adjusting your filters or search query.'
              : 'Run a sync to import orders from your providers.'}
          </p>
        </div>
      )}

      {/* Bottom pagination for long lists */}
      {data.orders.length > 10 && (
        <div className="mt-6 flex items-center justify-end gap-2">
          {hasPrevPage ? (
            <Link
              href={buildUrl(currentPage - 1)}
              className="inline-flex items-center gap-1 rounded-md bg-zinc-100 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700"
            >
              <ChevronLeftIcon className="size-4" />
              Previous
            </Link>
          ) : null}
          <span className="px-2 text-sm text-zinc-600 dark:text-zinc-400">
            Page {currentPage} of {totalPages}
          </span>
          {hasNextPage ? (
            <Link
              href={buildUrl(currentPage + 1)}
              className="inline-flex items-center gap-1 rounded-md bg-zinc-100 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700"
            >
              Next
              <ChevronRightIcon className="size-4" />
            </Link>
          ) : null}
        </div>
      )}
    </>
  )
}
