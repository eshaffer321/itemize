import { Badge } from '@/components/badge'
import { ConfidenceBadge, ConfidenceBar } from '@/components/confidence-badge'
import { DescriptionDetails, DescriptionList, DescriptionTerm } from '@/components/description-list'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { Link } from '@/components/link'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { getOrder } from '@/lib/api'
import type { OrderItem, OrderSplit } from '@/lib/api'
import { BanknotesIcon, CalendarIcon, ChevronLeftIcon, TagIcon } from '@heroicons/react/16/solid'
import type { Metadata } from 'next'
import { notFound } from 'next/navigation'

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  let { id } = await params
  return {
    title: `Order ${id}`,
  }
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
  const colorMap: Record<string, 'blue' | 'red' | 'amber' | 'zinc'> = {
    walmart: 'blue',
    costco: 'red',
    amazon: 'amber',
  }
  const color = colorMap[provider] || 'zinc'
  return <Badge color={color}>{provider}</Badge>
}

export default async function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  let { id } = await params
  let order

  try {
    order = await getOrder(id)
  } catch (error) {
    notFound()
  }

  if (!order) {
    notFound()
  }

  return (
    <>
      <div className="max-lg:hidden">
        <Link href="/orders" className="inline-flex items-center gap-2 text-sm/6 text-zinc-500 dark:text-zinc-400">
          <ChevronLeftIcon className="size-4 fill-zinc-400 dark:fill-zinc-500" />
          Orders
        </Link>
      </div>
      <div className="mt-4 lg:mt-8">
        <div className="flex items-center gap-4">
          <Heading>Order {order.order_id}</Heading>
          <StatusBadge status={order.status} />
          <ProviderBadge provider={order.provider} />
          {order.dry_run && <Badge color="purple">Dry Run</Badge>}
        </div>
        <div className="isolate mt-2.5 flex flex-wrap justify-between gap-x-6 gap-y-4">
          <div className="flex flex-wrap gap-x-10 gap-y-4 py-1.5">
            <span className="flex items-center gap-3 text-base/6 text-zinc-950 sm:text-sm/6 dark:text-white">
              <BanknotesIcon className="size-4 shrink-0 fill-zinc-400 dark:fill-zinc-500" />
              <span>{formatCurrency(order.order_total)}</span>
            </span>
            <span className="flex items-center gap-3 text-base/6 text-zinc-950 sm:text-sm/6 dark:text-white">
              <CalendarIcon className="size-4 shrink-0 fill-zinc-400 dark:fill-zinc-500" />
              <span>{formatDate(order.order_date)}</span>
            </span>
            <span className="flex items-center gap-3 text-base/6 text-zinc-950 sm:text-sm/6 dark:text-white">
              <TagIcon className="size-4 shrink-0 fill-zinc-400 dark:fill-zinc-500" />
              <span>
                {order.item_count} items, {order.split_count} splits
              </span>
            </span>
          </div>
        </div>
      </div>

      {order.error_message && (
        <div className="mt-8 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm font-medium text-red-700 dark:text-red-400">Error</p>
          <p className="mt-1 text-sm text-red-600 dark:text-red-300">{order.error_message}</p>
        </div>
      )}

      <div className="mt-12">
        <Subheading>Order Summary</Subheading>
        <Divider className="mt-4" />
        <DescriptionList>
          <DescriptionTerm>Subtotal</DescriptionTerm>
          <DescriptionDetails>{formatCurrency(order.order_subtotal)}</DescriptionDetails>
          <DescriptionTerm>Tax</DescriptionTerm>
          <DescriptionDetails>{formatCurrency(order.order_tax)}</DescriptionDetails>
          {order.order_tip !== undefined && order.order_tip > 0 && (
            <>
              <DescriptionTerm>Tip</DescriptionTerm>
              <DescriptionDetails>{formatCurrency(order.order_tip)}</DescriptionDetails>
            </>
          )}
          <DescriptionTerm>Total</DescriptionTerm>
          <DescriptionDetails className="font-semibold">{formatCurrency(order.order_total)}</DescriptionDetails>
          {order.transaction_id && (
            <>
              <DescriptionTerm>Transaction ID</DescriptionTerm>
              <DescriptionDetails>{order.transaction_id}</DescriptionDetails>
            </>
          )}
          <DescriptionTerm>Transaction Amount</DescriptionTerm>
          <DescriptionDetails>{formatCurrency(order.transaction_amount)}</DescriptionDetails>
          <DescriptionTerm>Match Confidence</DescriptionTerm>
          <DescriptionDetails>
            <div className="flex items-center gap-3">
              <ConfidenceBadge confidence={order.match_confidence} size="md" />
              <ConfidenceBar confidence={order.match_confidence} />
            </div>
          </DescriptionDetails>
          <DescriptionTerm>Processed At</DescriptionTerm>
          <DescriptionDetails>{formatDate(order.processed_at)}</DescriptionDetails>
        </DescriptionList>
      </div>

      {order.items && order.items.length > 0 && (
        <div className="mt-12">
          <Subheading>Items ({order.items.length})</Subheading>
          <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Item</TableHeader>
                <TableHeader>Category</TableHeader>
                <TableHeader className="text-right">Qty</TableHeader>
                <TableHeader className="text-right">Unit Price</TableHeader>
                <TableHeader className="text-right">Total</TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {order.items.map((item: OrderItem, index: number) => (
                <TableRow key={index}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell>{item.category || '-'}</TableCell>
                  <TableCell className="text-right">{item.quantity}</TableCell>
                  <TableCell className="text-right">{formatCurrency(item.unit_price)}</TableCell>
                  <TableCell className="text-right">{formatCurrency(item.total_price)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {order.splits && order.splits.length > 0 && (
        <div className="mt-12">
          <Subheading>Transaction Splits ({order.splits.length})</Subheading>
          <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Category</TableHeader>
                <TableHeader className="text-right">Amount</TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {order.splits.map((split: OrderSplit, index: number) => (
                <TableRow key={index}>
                  <TableCell className="font-medium">{split.category_name}</TableCell>
                  <TableCell className="text-right">{formatCurrency(split.amount)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}
