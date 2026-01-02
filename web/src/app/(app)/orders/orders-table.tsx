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
import type { Order } from '@/lib/api'
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

type OrderSortKey = 'order_id' | 'provider' | 'order_date' | 'status' | 'order_total' | 'item_count' | 'split_count'

function getOrderValue(order: Order, key: string): unknown {
  switch (key) {
    case 'order_id':
      return order.order_id
    case 'provider':
      return order.provider
    case 'order_date':
      return new Date(order.order_date)
    case 'status':
      return order.status
    case 'order_total':
      return order.order_total
    case 'item_count':
      return order.item_count
    case 'split_count':
      return order.split_count
    default:
      return null
  }
}

interface OrdersTableProps {
  orders: Order[]
}

export function OrdersTable({ orders }: OrdersTableProps) {
  const { sortConfig, handleSort } = useTableSort<OrderSortKey>('order_date', 'desc')

  const sortedOrders = useMemo(() => {
    return sortData(orders, sortConfig as SortConfig<string>, getOrderValue)
  }, [orders, sortConfig])

  // Cast handler to string type for SortableTableHeader compatibility
  const onSort = handleSort as (key: string) => void

  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <SortableTableHeader sortKey="order_id" currentSort={sortConfig} onSort={onSort}>
            Order ID
          </SortableTableHeader>
          <SortableTableHeader sortKey="provider" currentSort={sortConfig} onSort={onSort}>
            Provider
          </SortableTableHeader>
          <SortableTableHeader sortKey="order_date" currentSort={sortConfig} onSort={onSort}>
            Date
          </SortableTableHeader>
          <SortableTableHeader sortKey="status" currentSort={sortConfig} onSort={onSort}>
            Status
          </SortableTableHeader>
          <SortableTableHeader sortKey="order_total" currentSort={sortConfig} onSort={onSort} className="text-right">
            Total
          </SortableTableHeader>
          <SortableTableHeader sortKey="item_count" currentSort={sortConfig} onSort={onSort} className="text-right">
            Items
          </SortableTableHeader>
          <SortableTableHeader sortKey="split_count" currentSort={sortConfig} onSort={onSort} className="text-right">
            Splits
          </SortableTableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {sortedOrders.map((order: Order) => (
          <TableRow key={order.order_id} href={`/orders/${order.order_id}`} title={`View order ${order.order_id}`}>
            <TableCell className="font-medium">{order.order_id}</TableCell>
            <TableCell>
              <ProviderBadge provider={order.provider} />
            </TableCell>
            <TableCell className="text-zinc-500">{formatDate(order.order_date)}</TableCell>
            <TableCell>
              <StatusBadge status={order.status} />
            </TableCell>
            <TableCell className="text-right">{formatCurrency(order.order_total)}</TableCell>
            <TableCell className="text-right">{order.item_count}</TableCell>
            <TableCell className="text-right">{order.split_count}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
