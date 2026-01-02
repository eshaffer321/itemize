import { Badge } from '@/components/badge'
import { Button } from '@/components/button'
import { DescriptionDetails, DescriptionList, DescriptionTerm } from '@/components/description-list'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { getTransaction, getOrdersByTransactionId } from '@/lib/api'
import type { TransactionDetail, TransactionSplit, Order } from '@/lib/api'
import { ChevronLeftIcon, ShoppingCartIcon } from '@heroicons/react/16/solid'
import type { Metadata } from 'next'
import Link from 'next/link'
import { notFound } from 'next/navigation'

export const metadata: Metadata = {
  title: 'Transaction Details',
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  })
}

function formatDateTime(dateString: string): string {
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
  }).format(Math.abs(amount))
}

function AmountDisplay({ amount, large }: { amount: number; large?: boolean }) {
  const isExpense = amount < 0
  const className = large
    ? `text-2xl font-bold ${isExpense ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`
    : `font-medium ${isExpense ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`
  return (
    <span className={className}>
      {isExpense ? '-' : '+'}{formatCurrency(amount)}
    </span>
  )
}

function SplitRow({ split }: { split: TransactionSplit }) {
  return (
    <div className="flex items-center justify-between py-3 border-b border-zinc-100 dark:border-zinc-800 last:border-0">
      <div>
        <p className="font-medium text-zinc-900 dark:text-zinc-100">
          {split.merchant?.name || 'No merchant'}
        </p>
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          {split.category?.icon} {split.category?.name || 'Uncategorized'}
        </p>
        {split.notes && (
          <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">{split.notes}</p>
        )}
      </div>
      <AmountDisplay amount={split.amount} />
    </div>
  )
}

interface PageProps {
  params: Promise<{ id: string }>
}

export default async function TransactionDetailPage({ params }: PageProps) {
  const { id } = await params

  let transaction: TransactionDetail
  let linkedOrders: Order[] = []
  try {
    transaction = await getTransaction(id)
    // Fetch orders that are linked to this transaction
    const ordersResult = await getOrdersByTransactionId(id)
    // Filter to only orders that have this exact transaction_id
    linkedOrders = ordersResult.orders.filter(order => order.transaction_id === id)
  } catch (error) {
    if (error instanceof Error && error.message.includes('not found')) {
      notFound()
    }
    return (
      <>
        <div className="mb-6">
          <Link href="/transactions" className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-300">
            <ChevronLeftIcon className="size-4" />
            Back to Transactions
          </Link>
        </div>
        <Heading>Transaction Details</Heading>
        <div className="mt-8 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm text-red-700 dark:text-red-400">
            Failed to load transaction details.
          </p>
        </div>
      </>
    )
  }

  return (
    <>
      <div className="mb-6">
        <Link href="/transactions" className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-300">
          <ChevronLeftIcon className="size-4" />
          Back to Transactions
        </Link>
      </div>

      <div className="flex flex-wrap items-start justify-between gap-4 mb-6">
        <div>
          <Heading>{transaction.merchant?.name || transaction.plaid_name || 'Transaction'}</Heading>
          <p className="mt-1 text-zinc-500 dark:text-zinc-400">{formatDate(transaction.date)}</p>
        </div>
        <div className="text-right">
          <AmountDisplay amount={transaction.amount} large />
          <div className="mt-2 flex items-center justify-end gap-2">
            {transaction.pending && <Badge color="amber">Pending</Badge>}
            {transaction.needs_review && <Badge color="blue">Needs Review</Badge>}
            {transaction.has_splits && <Badge color="purple">Split Transaction</Badge>}
            {transaction.is_recurring && <Badge color="green">Recurring</Badge>}
          </div>
        </div>
      </div>

      <Divider className="my-6" />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div>
          <Subheading>Transaction Details</Subheading>
          <DescriptionList className="mt-4">
            <DescriptionTerm>Transaction ID</DescriptionTerm>
            <DescriptionDetails className="font-mono text-sm">{transaction.id}</DescriptionDetails>

            <DescriptionTerm>Date</DescriptionTerm>
            <DescriptionDetails>{formatDate(transaction.date)}</DescriptionDetails>

            <DescriptionTerm>Merchant</DescriptionTerm>
            <DescriptionDetails>{transaction.merchant?.name || 'Unknown'}</DescriptionDetails>

            {transaction.original_merchant && (
              <>
                <DescriptionTerm>Original Merchant</DescriptionTerm>
                <DescriptionDetails>{transaction.original_merchant}</DescriptionDetails>
              </>
            )}

            <DescriptionTerm>Category</DescriptionTerm>
            <DescriptionDetails>
              {transaction.category ? (
                <span className="inline-flex items-center gap-1">
                  {transaction.category.icon} {transaction.category.name}
                  {transaction.category.group && (
                    <span className="text-zinc-400"> ({transaction.category.group.name})</span>
                  )}
                </span>
              ) : (
                <span className="text-zinc-400">Uncategorized</span>
              )}
            </DescriptionDetails>

            {transaction.original_category && (
              <>
                <DescriptionTerm>Original Category</DescriptionTerm>
                <DescriptionDetails>
                  {transaction.original_category.icon} {transaction.original_category.name}
                </DescriptionDetails>
              </>
            )}

            <DescriptionTerm>Account</DescriptionTerm>
            <DescriptionDetails>
              {transaction.account?.display_name || 'Unknown'}
              {transaction.account?.mask && (
                <span className="text-zinc-400"> (...{transaction.account.mask})</span>
              )}
            </DescriptionDetails>

            {transaction.notes && (
              <>
                <DescriptionTerm>Notes</DescriptionTerm>
                <DescriptionDetails>{transaction.notes}</DescriptionDetails>
              </>
            )}
          </DescriptionList>
        </div>

        <div>
          <Subheading>Status & Metadata</Subheading>
          <DescriptionList className="mt-4">
            <DescriptionTerm>Status</DescriptionTerm>
            <DescriptionDetails>
              {transaction.pending ? (
                <Badge color="amber">Pending</Badge>
              ) : (
                <Badge color="green">Posted</Badge>
              )}
            </DescriptionDetails>

            <DescriptionTerm>Needs Review</DescriptionTerm>
            <DescriptionDetails>
              {transaction.needs_review ? (
                <Badge color="blue">Yes</Badge>
              ) : (
                <span className="text-zinc-400">No</span>
              )}
            </DescriptionDetails>

            {transaction.reviewed_at && (
              <>
                <DescriptionTerm>Reviewed At</DescriptionTerm>
                <DescriptionDetails>{formatDateTime(transaction.reviewed_at)}</DescriptionDetails>
              </>
            )}

            <DescriptionTerm>Hide from Reports</DescriptionTerm>
            <DescriptionDetails>
              {transaction.hide_from_reports ? 'Yes' : 'No'}
            </DescriptionDetails>

            <DescriptionTerm>Recurring</DescriptionTerm>
            <DescriptionDetails>
              {transaction.is_recurring ? <Badge color="green">Yes</Badge> : 'No'}
            </DescriptionDetails>

            <DescriptionTerm>Created</DescriptionTerm>
            <DescriptionDetails>{formatDateTime(transaction.created_at)}</DescriptionDetails>

            <DescriptionTerm>Last Updated</DescriptionTerm>
            <DescriptionDetails>{formatDateTime(transaction.updated_at)}</DescriptionDetails>
          </DescriptionList>
        </div>
      </div>

      {transaction.tags && transaction.tags.length > 0 && (
        <>
          <Divider className="my-6" />
          <Subheading>Tags</Subheading>
          <div className="mt-4 flex flex-wrap gap-2">
            {transaction.tags.map((tag) => (
              <Badge key={tag.id} color="zinc" style={{ backgroundColor: tag.color || undefined }}>
                {tag.name}
              </Badge>
            ))}
          </div>
        </>
      )}

      {transaction.splits && transaction.splits.length > 0 && (
        <>
          <Divider className="my-6" />
          <Subheading>Splits ({transaction.splits.length})</Subheading>
          <div className="mt-4 rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
            {transaction.splits.map((split) => (
              <SplitRow key={split.id} split={split} />
            ))}
          </div>
        </>
      )}

      {linkedOrders.length > 0 && (
        <>
          <Divider className="my-6" />
          <Subheading className="flex items-center gap-2">
            <ShoppingCartIcon className="size-5" />
            Linked Orders ({linkedOrders.length})
          </Subheading>
          <div className="mt-4 space-y-3">
            {linkedOrders.map((order) => (
              <Link
                key={order.order_id}
                href={`/orders/${order.order_id}`}
                className="block rounded-lg border border-zinc-200 dark:border-zinc-800 p-4 hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium text-zinc-900 dark:text-zinc-100">
                      Order {order.order_id}
                    </p>
                    <p className="text-sm text-zinc-500 dark:text-zinc-400">
                      {order.provider.charAt(0).toUpperCase() + order.provider.slice(1)} • {new Date(order.order_date).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="font-medium text-zinc-900 dark:text-zinc-100">
                      {formatCurrency(order.order_total)}
                    </p>
                    <Badge color={order.status === 'success' ? 'green' : order.status === 'failed' ? 'red' : 'amber'}>
                      {order.status}
                    </Badge>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </>
      )}

      <Divider className="my-6" />

      <div className="flex gap-4">
        <Button href="/transactions" outline>
          <ChevronLeftIcon />
          Back to Transactions
        </Button>
      </div>
    </>
  )
}
