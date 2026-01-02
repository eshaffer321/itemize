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
import type { Transaction } from '@/lib/api'
import { useMemo } from 'react'

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
  }).format(Math.abs(amount))
}

function PendingBadge({ pending }: { pending: boolean }) {
  if (!pending) return null
  return <Badge color="amber">Pending</Badge>
}

function AmountDisplay({ amount }: { amount: number }) {
  const isExpense = amount < 0
  return (
    <span className={isExpense ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}>
      {isExpense ? '-' : '+'}{formatCurrency(amount)}
    </span>
  )
}

function CategoryBadge({ category }: { category?: Transaction['category'] }) {
  if (!category) return <span className="text-zinc-400">Uncategorized</span>
  return (
    <span className="inline-flex items-center gap-1">
      {category.icon && <span>{category.icon}</span>}
      {category.name}
    </span>
  )
}

type TransactionSortKey = 'date' | 'merchant' | 'category' | 'account' | 'amount'

function getTransactionValue(txn: Transaction, key: string): unknown {
  switch (key) {
    case 'date':
      return new Date(txn.date)
    case 'merchant':
      return txn.merchant?.name || txn.plaid_name || ''
    case 'category':
      return txn.category?.name || ''
    case 'account':
      return txn.account?.display_name || ''
    case 'amount':
      return txn.amount
    default:
      return null
  }
}

interface TransactionsTableProps {
  transactions: Transaction[]
}

export function TransactionsTable({ transactions }: TransactionsTableProps) {
  const { sortConfig, handleSort } = useTableSort<TransactionSortKey>('date', 'desc')

  const sortedTransactions = useMemo(() => {
    return sortData(transactions, sortConfig as SortConfig<string>, getTransactionValue)
  }, [transactions, sortConfig])

  const onSort = handleSort as (key: string) => void

  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <SortableTableHeader sortKey="date" currentSort={sortConfig} onSort={onSort}>
            Date
          </SortableTableHeader>
          <SortableTableHeader sortKey="merchant" currentSort={sortConfig} onSort={onSort}>
            Merchant
          </SortableTableHeader>
          <SortableTableHeader sortKey="category" currentSort={sortConfig} onSort={onSort}>
            Category
          </SortableTableHeader>
          <SortableTableHeader sortKey="account" currentSort={sortConfig} onSort={onSort}>
            Account
          </SortableTableHeader>
          <SortableTableHeader sortKey="amount" currentSort={sortConfig} onSort={onSort} className="text-right">
            Amount
          </SortableTableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {sortedTransactions.map((txn: Transaction) => (
          <TableRow key={txn.id} href={`/transactions/${txn.id}`} title={`View transaction details`}>
            <TableCell className="text-zinc-500">{formatDate(txn.date)}</TableCell>
            <TableCell className="font-medium">
              {txn.merchant?.name || txn.plaid_name || 'Unknown'}
            </TableCell>
            <TableCell>
              <CategoryBadge category={txn.category} />
            </TableCell>
            <TableCell className="text-zinc-500">
              {txn.account?.display_name || 'Unknown'}
            </TableCell>
            <TableCell>
              <div className="flex items-center justify-end gap-2">
                <PendingBadge pending={txn.pending} />
                {txn.needs_review && <Badge color="blue">Review</Badge>}
                {txn.has_splits && <Badge color="purple">Split</Badge>}
                <span className="font-medium">
                  <AmountDisplay amount={txn.amount} />
                </span>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
