import { Button } from '@/components/button'
import { Heading } from '@/components/heading'
import { Input, InputGroup } from '@/components/input'
import { Select } from '@/components/select'
import { getTransactions } from '@/lib/api'
import { ChevronLeftIcon, ChevronRightIcon, MagnifyingGlassIcon } from '@heroicons/react/16/solid'
import type { Metadata } from 'next'
import Link from 'next/link'
import { TransactionsTable } from './transactions-table'

export const metadata: Metadata = {
  title: 'Transactions',
}

const PAGE_SIZE = 25

// Supported retail providers for syncing
const SYNCED_MERCHANTS = ['walmart', 'costco', 'amazon']

// Check if a merchant name matches any supported provider
function isSyncedMerchant(merchantName: string | undefined): boolean {
  if (!merchantName) return false
  const lowerName = merchantName.toLowerCase()
  return SYNCED_MERCHANTS.some(provider => lowerName.includes(provider))
}

interface PageProps {
  searchParams: Promise<{
    page?: string
    search?: string
    days?: string
    pending?: string
    merchant?: string
    notes?: string
    splits?: string
  }>
}

export default async function TransactionsPage({ searchParams }: PageProps) {
  const params = await searchParams
  const currentPage = Math.max(1, parseInt(params.page || '1', 10))
  const search = params.search || ''
  const days = params.days || '30'
  const pendingOnly = params.pending === 'true'
  // notes filter: 'with' = has notes, 'without' = no notes, '' = all
  const notesFilter = params.notes || ''
  // splits filter: 'with' = has splits, 'without' = no splits, '' = all
  const splitsFilter = params.splits || ''
  // Default to 'synced' to show only supported merchants
  const merchantFilter = params.merchant || 'synced'
  const daysBack = parseInt(days, 10)
  const offset = (currentPage - 1) * PAGE_SIZE

  let data
  try {
    data = await getTransactions({
      limit: PAGE_SIZE,
      offset,
      search: search || undefined,
      days_back: daysBack,
      pending: pendingOnly || undefined,
    })
  } catch (error) {
    return (
      <>
        <Heading>Transactions</Heading>
        <div className="mt-8 rounded-lg bg-red-50 p-4 dark:bg-red-900/20">
          <p className="text-sm text-red-700 dark:text-red-400">
            Failed to load transactions. Make sure the API server is running and Monarch Money is configured.
          </p>
        </div>
      </>
    )
  }

  const totalPages = Math.ceil(data.total_count / PAGE_SIZE)
  const hasNextPage = currentPage < totalPages
  const hasPrevPage = currentPage > 1

  function buildUrl(page: number) {
    const params = new URLSearchParams()
    params.set('page', page.toString())
    if (search) params.set('search', search)
    if (days) params.set('days', days)
    if (pendingOnly) params.set('pending', 'true')
    if (notesFilter) params.set('notes', notesFilter)
    if (splitsFilter) params.set('splits', splitsFilter)
    if (merchantFilter !== 'synced') params.set('merchant', merchantFilter)
    return `/transactions?${params.toString()}`
  }

  // Filter transactions based on merchant filter
  let filteredTransactions = data.transactions
  let filteredCount = data.total_count

  if (merchantFilter === 'synced') {
    // Filter to only show transactions from supported merchants
    filteredTransactions = data.transactions.filter(txn =>
      isSyncedMerchant(txn.merchant?.name) || isSyncedMerchant(txn.plaid_name)
    )
    // Note: This is an approximation since we filter client-side
    filteredCount = filteredTransactions.length
  } else if (merchantFilter !== 'all') {
    // Filter by specific merchant
    filteredTransactions = data.transactions.filter(txn => {
      const merchantName = txn.merchant?.name?.toLowerCase() || txn.plaid_name?.toLowerCase() || ''
      return merchantName.includes(merchantFilter.toLowerCase())
    })
    filteredCount = filteredTransactions.length
  }

  // Filter by notes
  if (notesFilter === 'with') {
    filteredTransactions = filteredTransactions.filter(txn => txn.notes && txn.notes.trim().length > 0)
    filteredCount = filteredTransactions.length
  } else if (notesFilter === 'without') {
    filteredTransactions = filteredTransactions.filter(txn => !txn.notes || txn.notes.trim().length === 0)
    filteredCount = filteredTransactions.length
  }

  // Filter by splits
  if (splitsFilter === 'with') {
    filteredTransactions = filteredTransactions.filter(txn => txn.has_splits)
    filteredCount = filteredTransactions.length
  } else if (splitsFilter === 'without') {
    filteredTransactions = filteredTransactions.filter(txn => !txn.has_splits)
    filteredCount = filteredTransactions.length
  }

  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Transactions</Heading>
        <div className="flex gap-4">
          <form className="flex flex-wrap items-center gap-4">
            <InputGroup>
              <MagnifyingGlassIcon />
              <Input name="search" placeholder="Search transactions..." defaultValue={search} className="w-48" />
            </InputGroup>
            <Select name="merchant" defaultValue={merchantFilter}>
              <option value="synced">Synced Merchants</option>
              <option value="all">All Merchants</option>
              <option value="walmart">Walmart</option>
              <option value="costco">Costco</option>
              <option value="amazon">Amazon</option>
            </Select>
            <Select name="days" defaultValue={days}>
              <option value="7">Last 7 Days</option>
              <option value="14">Last 14 Days</option>
              <option value="30">Last 30 Days</option>
              <option value="60">Last 60 Days</option>
              <option value="90">Last 90 Days</option>
            </Select>
            <label className="flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
              <input
                type="checkbox"
                name="pending"
                value="true"
                defaultChecked={pendingOnly}
                className="rounded border-zinc-300 text-zinc-900 focus:ring-zinc-500 dark:border-zinc-600 dark:bg-zinc-800"
              />
              Pending only
            </label>
            <Select name="notes" defaultValue={notesFilter}>
              <option value="">All Notes</option>
              <option value="with">Has Notes</option>
              <option value="without">No Notes</option>
            </Select>
            <Select name="splits" defaultValue={splitsFilter}>
              <option value="">All Splits</option>
              <option value="with">Has Splits</option>
              <option value="without">No Splits</option>
            </Select>
            <Button type="submit">Filter</Button>
          </form>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between">
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          {merchantFilter === 'synced' || merchantFilter !== 'all' ? (
            <>Showing {filteredCount} {merchantFilter === 'synced' ? 'synced merchant' : merchantFilter} transactions</>
          ) : (
            <>Showing {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total_count)} of {data.total_count} transactions</>
          )}
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
            Page {currentPage} of {totalPages || 1}
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

      <TransactionsTable transactions={filteredTransactions} />

      {filteredTransactions.length === 0 && (
        <div className="mt-8 text-center text-zinc-500 dark:text-zinc-400">
          <p>No transactions found.</p>
          <p className="mt-2 text-sm">
            {notesFilter === 'with' ? (
              'No transactions with notes found. Try changing the Notes filter.'
            ) : notesFilter === 'without' ? (
              'All transactions have notes. Try changing the Notes filter.'
            ) : splitsFilter === 'with' ? (
              'No split transactions found. Try changing the Splits filter.'
            ) : splitsFilter === 'without' ? (
              'All transactions have splits. Try changing the Splits filter.'
            ) : merchantFilter === 'synced' ? (
              'No transactions from synced merchants (Walmart, Costco, Amazon). Try selecting "All Merchants".'
            ) : search || pendingOnly ? (
              'Try adjusting your filters or search query.'
            ) : (
              'No transactions in the selected time period.'
            )}
          </p>
        </div>
      )}

      {filteredTransactions.length > 10 && (
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
            Page {currentPage} of {totalPages || 1}
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
