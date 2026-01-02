'use client'

import { ChevronDownIcon, ChevronUpIcon } from '@heroicons/react/16/solid'
import clsx from 'clsx'
import type React from 'react'
import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { Link } from './link'

// Sorting types and utilities
export type SortDirection = 'asc' | 'desc' | null
export type SortConfig<T = string> = { key: T; direction: SortDirection }

// Hook for managing sort state
export function useTableSort<T extends string>(defaultKey?: T, defaultDirection: SortDirection = null) {
  const [sortConfig, setSortConfig] = useState<SortConfig<T>>({
    key: defaultKey as T,
    direction: defaultDirection,
  })

  const handleSort = useCallback((key: T) => {
    setSortConfig((current) => {
      if (current.key !== key) {
        return { key, direction: 'asc' }
      }
      if (current.direction === 'asc') {
        return { key, direction: 'desc' }
      }
      if (current.direction === 'desc') {
        return { key, direction: null }
      }
      return { key, direction: 'asc' }
    })
  }, [])

  return { sortConfig, handleSort, setSortConfig }
}

// Utility function to sort data
export function sortData<T>(
  data: T[],
  sortConfig: SortConfig<string>,
  getValueFn: (item: T, key: string) => unknown
): T[] {
  if (!sortConfig.key || !sortConfig.direction) {
    return data
  }

  return [...data].sort((a, b) => {
    const aValue = getValueFn(a, sortConfig.key)
    const bValue = getValueFn(b, sortConfig.key)

    // Handle null/undefined
    if (aValue == null && bValue == null) return 0
    if (aValue == null) return sortConfig.direction === 'asc' ? 1 : -1
    if (bValue == null) return sortConfig.direction === 'asc' ? -1 : 1

    // Compare values
    let comparison = 0
    if (typeof aValue === 'string' && typeof bValue === 'string') {
      comparison = aValue.localeCompare(bValue)
    } else if (typeof aValue === 'number' && typeof bValue === 'number') {
      comparison = aValue - bValue
    } else if (aValue instanceof Date && bValue instanceof Date) {
      comparison = aValue.getTime() - bValue.getTime()
    } else {
      comparison = String(aValue).localeCompare(String(bValue))
    }

    return sortConfig.direction === 'asc' ? comparison : -comparison
  })
}

const TableContext = createContext<{ bleed: boolean; dense: boolean; grid: boolean; striped: boolean }>({
  bleed: false,
  dense: false,
  grid: false,
  striped: false,
})

export function Table({
  bleed = false,
  dense = false,
  grid = false,
  striped = false,
  className,
  children,
  ...props
}: { bleed?: boolean; dense?: boolean; grid?: boolean; striped?: boolean } & React.ComponentPropsWithoutRef<'div'>) {
  return (
    <TableContext.Provider value={{ bleed, dense, grid, striped } as React.ContextType<typeof TableContext>}>
      <div className="flow-root">
        <div {...props} className={clsx(className, '-mx-(--gutter) overflow-x-auto whitespace-nowrap')}>
          <div className={clsx('inline-block min-w-full align-middle', !bleed && 'sm:px-(--gutter)')}>
            <table className="min-w-full text-left text-sm/6 text-zinc-950 dark:text-white">{children}</table>
          </div>
        </div>
      </div>
    </TableContext.Provider>
  )
}

export function TableHead({ className, ...props }: React.ComponentPropsWithoutRef<'thead'>) {
  return <thead {...props} className={clsx(className, 'text-zinc-500 dark:text-zinc-400')} />
}

export function TableBody(props: React.ComponentPropsWithoutRef<'tbody'>) {
  return <tbody {...props} />
}

const TableRowContext = createContext<{ href?: string; target?: string; title?: string }>({
  href: undefined,
  target: undefined,
  title: undefined,
})

export function TableRow({
  href,
  target,
  title,
  className,
  ...props
}: { href?: string; target?: string; title?: string } & React.ComponentPropsWithoutRef<'tr'>) {
  let { striped } = useContext(TableContext)

  return (
    <TableRowContext.Provider value={{ href, target, title } as React.ContextType<typeof TableRowContext>}>
      <tr
        {...props}
        className={clsx(
          className,
          href &&
            'has-[[data-row-link][data-focus]]:outline-2 has-[[data-row-link][data-focus]]:-outline-offset-2 has-[[data-row-link][data-focus]]:outline-brand-500 dark:focus-within:bg-white/2.5',
          striped && 'even:bg-zinc-950/2.5 dark:even:bg-white/2.5',
          href && striped && 'hover:bg-zinc-950/5 dark:hover:bg-white/5',
          href && !striped && 'hover:bg-zinc-950/2.5 dark:hover:bg-white/2.5',
          href && 'cursor-pointer'
        )}
      />
    </TableRowContext.Provider>
  )
}

export function TableHeader({ className, ...props }: React.ComponentPropsWithoutRef<'th'>) {
  let { bleed, grid } = useContext(TableContext)

  return (
    <th
      {...props}
      className={clsx(
        className,
        'border-b border-b-zinc-950/10 px-4 py-2 font-medium first:pl-(--gutter,--spacing(2)) last:pr-(--gutter,--spacing(2)) dark:border-b-white/10',
        grid && 'border-l border-l-zinc-950/5 first:border-l-0 dark:border-l-white/5',
        !bleed && 'sm:first:pl-1 sm:last:pr-1'
      )}
    />
  )
}

export function TableCell({ className, children, ...props }: React.ComponentPropsWithoutRef<'td'>) {
  let { bleed, dense, grid, striped } = useContext(TableContext)
  let { href, target, title } = useContext(TableRowContext)
  let [cellRef, setCellRef] = useState<HTMLElement | null>(null)

  return (
    <td
      ref={href ? setCellRef : undefined}
      {...props}
      className={clsx(
        className,
        'relative px-4 first:pl-(--gutter,--spacing(2)) last:pr-(--gutter,--spacing(2))',
        !striped && 'border-b border-zinc-950/5 dark:border-white/5',
        grid && 'border-l border-l-zinc-950/5 first:border-l-0 dark:border-l-white/5',
        dense ? 'py-2.5' : 'py-4',
        !bleed && 'sm:first:pl-1 sm:last:pr-1'
      )}
    >
      {children}
      {href && (
        <Link
          data-row-link
          href={href}
          target={target}
          aria-label={title}
          tabIndex={cellRef?.previousElementSibling === null ? 0 : -1}
          className="absolute inset-0 z-10 focus:outline-hidden"
        />
      )}
    </td>
  )
}

// Sortable table header - shows sort indicator and handles click
export function SortableTableHeader({
  sortKey,
  currentSort,
  onSort,
  className,
  children,
  ...props
}: {
  sortKey: string
  currentSort: SortConfig<string>
  onSort: (key: string) => void
} & React.ComponentPropsWithoutRef<'th'>) {
  let { bleed, grid } = useContext(TableContext)
  const isActive = currentSort.key === sortKey && currentSort.direction !== null
  const direction = isActive ? currentSort.direction : null

  return (
    <th
      {...props}
      onClick={() => onSort(sortKey)}
      className={clsx(
        className,
        'border-b border-b-zinc-950/10 px-4 py-2 font-medium first:pl-(--gutter,--spacing(2)) last:pr-(--gutter,--spacing(2)) dark:border-b-white/10',
        grid && 'border-l border-l-zinc-950/5 first:border-l-0 dark:border-l-white/5',
        !bleed && 'sm:first:pl-1 sm:last:pr-1',
        'cursor-pointer select-none hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors'
      )}
    >
      <span className="inline-flex items-center gap-1">
        {children}
        <span className={clsx('inline-flex flex-col', !isActive && 'opacity-30')}>
          {direction === 'asc' ? (
            <ChevronUpIcon className="h-3 w-3" />
          ) : direction === 'desc' ? (
            <ChevronDownIcon className="h-3 w-3" />
          ) : (
            <span className="h-3 w-3 flex flex-col justify-center">
              <ChevronUpIcon className="h-2 w-2 -mb-0.5" />
              <ChevronDownIcon className="h-2 w-2 -mt-0.5" />
            </span>
          )}
        </span>
      </span>
    </th>
  )
}
