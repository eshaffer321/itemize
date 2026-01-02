import { Badge } from '@/components/badge'
import { CollapsibleSection } from '@/components/collapsible-section'
import { DescriptionDetails, DescriptionList, DescriptionTerm } from '@/components/description-list'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { Link } from '@/components/link'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/table'
import { getOrder, getOrderLedger } from '@/lib/api'
import type { OrderItem, OrderSplit, Ledger, LedgerCharge } from '@/lib/api'
import { BanknotesIcon, CalendarIcon, ChevronLeftIcon, TagIcon, CreditCardIcon, ExclamationTriangleIcon } from '@heroicons/react/16/solid'
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

function LedgerStateBadge({ state }: { state: string }) {
  const stateConfig: Record<string, { color: 'green' | 'amber' | 'red' | 'blue' | 'zinc'; label: string }> = {
    charged: { color: 'green', label: 'Charged' },
    payment_pending: { color: 'amber', label: 'Pending' },
    partial_refund: { color: 'blue', label: 'Partial Refund' },
    refunded: { color: 'red', label: 'Refunded' },
  }
  const config = stateConfig[state] || { color: 'zinc', label: state }
  return <Badge color={config.color}>{config.label}</Badge>
}

function ChargeTypeBadge({ type }: { type: string }) {
  if (type === 'refund') {
    return <Badge color="red">Refund</Badge>
  }
  return <Badge color="green">Payment</Badge>
}

export default async function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  let { id } = await params
  let order
  let ledger: Ledger | null = null

  try {
    order = await getOrder(id)
    ledger = await getOrderLedger(id)
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
              <DescriptionTerm>Monarch Transaction</DescriptionTerm>
              <DescriptionDetails>
                <Link href={`/transactions/${order.transaction_id}`} className="text-blue-600 hover:underline dark:text-blue-400">
                  View Transaction →
                </Link>
              </DescriptionDetails>
            </>
          )}
          <DescriptionTerm>Transaction Amount</DescriptionTerm>
          <DescriptionDetails>{formatCurrency(order.transaction_amount)}</DescriptionDetails>
          <DescriptionTerm>Processed At</DescriptionTerm>
          <DescriptionDetails>{formatDate(order.processed_at)}</DescriptionDetails>
        </DescriptionList>
      </div>

      {order.items && order.items.length > 0 && (
        <div className="mt-12">
          <CollapsibleSection
            title={<Subheading className="mb-0">Items</Subheading>}
            itemCount={order.items.length}
            defaultOpen={order.items.length <= 15}
          >
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
          </CollapsibleSection>
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

      {ledger && (
        <div className="mt-12">
          <div className="flex items-center gap-4">
            <Subheading>Payment Ledger</Subheading>
            <LedgerStateBadge state={ledger.ledger_state} />
            {ledger.has_refunds && <Badge color="amber">Has Refunds</Badge>}
            {!ledger.is_valid && <Badge color="red">Invalid</Badge>}
          </div>

          {/* Prominent validation warning when ledger is invalid */}
          {!ledger.is_valid && (
            <div className="mt-4 rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-800 dark:bg-red-900/20">
              <div className="flex items-start gap-3">
                <ExclamationTriangleIcon className="size-5 shrink-0 fill-red-500" aria-hidden="true" />
                <div>
                  <p className="text-sm font-medium text-red-800 dark:text-red-200">Ledger Validation Failed</p>
                  {ledger.validation_notes && (
                    <p className="mt-1 text-sm text-red-700 dark:text-red-300">{ledger.validation_notes}</p>
                  )}
                </div>
              </div>
            </div>
          )}

          <Divider className="mt-4" />
          <DescriptionList>
            <DescriptionTerm>Total Charged</DescriptionTerm>
            <DescriptionDetails className="font-semibold">{formatCurrency(ledger.total_charged)}</DescriptionDetails>
            <DescriptionTerm>Payment Methods</DescriptionTerm>
            <DescriptionDetails>{ledger.payment_method_types || 'N/A'}</DescriptionDetails>
            <DescriptionTerm>Charge Count</DescriptionTerm>
            <DescriptionDetails>{ledger.charge_count}</DescriptionDetails>
            <DescriptionTerm>Ledger Version</DescriptionTerm>
            <DescriptionDetails>v{ledger.ledger_version}</DescriptionDetails>
            <DescriptionTerm>Last Updated</DescriptionTerm>
            <DescriptionDetails>{formatDate(ledger.fetched_at)}</DescriptionDetails>
          </DescriptionList>

          <div className="mt-8">
            <Subheading level={3}>Charges ({ledger.charges?.length ?? 0})</Subheading>
            {ledger.charges && ledger.charges.length > 0 ? (
              <>
                <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
                  <TableHead>
                    <TableRow>
                      <TableHeader title="Charge sequence number">Seq</TableHeader>
                      <TableHeader>Type</TableHeader>
                      <TableHeader>Date</TableHeader>
                      <TableHeader>Payment Method</TableHeader>
                      <TableHeader>Card</TableHeader>
                      <TableHeader className="text-right">Amount</TableHeader>
                      <TableHeader>Status</TableHeader>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {ledger.charges.map((charge: LedgerCharge) => (
                      <TableRow key={charge.id}>
                        <TableCell className="text-zinc-500">{charge.charge_sequence}</TableCell>
                        <TableCell>
                          <ChargeTypeBadge type={charge.charge_type} />
                        </TableCell>
                        <TableCell>{charge.charged_at ? formatDate(charge.charged_at) : '-'}</TableCell>
                        <TableCell className="font-medium">{charge.payment_method}</TableCell>
                        <TableCell>
                          {charge.card_type && charge.card_last_four ? (
                            <span className="flex items-center gap-2">
                              <CreditCardIcon
                                className="size-4 fill-zinc-400"
                                aria-label={`${charge.card_type} card`}
                              />
                              <span>
                                {charge.card_type} ****{charge.card_last_four}
                              </span>
                            </span>
                          ) : (
                            '-'
                          )}
                        </TableCell>
                        <TableCell className="text-right font-medium">
                          <span className={charge.charge_type === 'refund' ? 'text-red-600 dark:text-red-400' : ''}>
                            {charge.charge_type === 'refund' ? '-' : ''}
                            {formatCurrency(Math.abs(charge.charge_amount))}
                          </span>
                        </TableCell>
                        <TableCell>
                          {charge.is_matched ? (
                            <Badge color="green">Matched</Badge>
                          ) : (
                            <Badge color="zinc">Unmatched</Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                    {/* Total row */}
                    <TableRow className="border-t-2 border-zinc-200 dark:border-zinc-700">
                      <TableCell colSpan={5} className="text-right font-semibold text-zinc-700 dark:text-zinc-300">
                        Net Total
                      </TableCell>
                      <TableCell className="text-right font-semibold">
                        {formatCurrency(ledger.total_charged)}
                      </TableCell>
                      <TableCell />
                    </TableRow>
                  </TableBody>
                </Table>
              </>
            ) : (
              <div className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 p-6 text-center dark:border-zinc-700 dark:bg-zinc-800/50">
                <p className="text-sm text-zinc-500 dark:text-zinc-400">No charge records available for this ledger.</p>
              </div>
            )}
          </div>
        </div>
      )}
    </>
  )
}
