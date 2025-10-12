import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Package, DollarSign, Calendar, Tag, AlertCircle, CheckCircle, XCircle, Clock } from 'lucide-react'
import { format } from 'date-fns'
import { clsx } from 'clsx'
import { fetchOrderDetail } from '../api/orders'

const statusIcons = {
  success: CheckCircle,
  failed: XCircle,
  skipped: AlertCircle,
  processing: Clock,
  'dry-run': Package,
}

const statusColors = {
  success: 'text-green-600 bg-green-50',
  failed: 'text-red-600 bg-red-50',
  skipped: 'text-yellow-600 bg-yellow-50',
  processing: 'text-blue-600 bg-blue-50',
  'dry-run': 'text-purple-600 bg-purple-50',
}

export default function OrderDetail() {
  const { orderId } = useParams<{ orderId: string }>()
  
  const { data: order, isLoading, error } = useQuery({
    queryKey: ['order', orderId],
    queryFn: () => fetchOrderDetail(orderId!),
    enabled: !!orderId,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading order details...</div>
      </div>
    )
  }

  if (error || !order) {
    return (
      <div className="bg-red-50 text-red-800 p-4 rounded-lg">
        Error loading order: {(error as Error)?.message || 'Order not found'}
      </div>
    )
  }

  const StatusIcon = statusIcons[order.status] || Package

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Link
            to="/orders"
            className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
          >
            <ArrowLeft className="h-5 w-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold">Order {order.order_id}</h1>
            <p className="text-gray-600">
              {order.provider.charAt(0).toUpperCase() + order.provider.slice(1)} • {format(new Date(order.order_date), 'MMMM d, yyyy')}
            </p>
          </div>
        </div>
        <div className={clsx('flex items-center space-x-2 px-4 py-2 rounded-lg', statusColors[order.status])}>
          <StatusIcon className="h-5 w-5" />
          <span className="font-medium">{order.status.charAt(0).toUpperCase() + order.status.slice(1)}</span>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-600">Total Amount</p>
              <p className="text-xl font-bold">${order.order_total?.toFixed(2)}</p>
            </div>
            <DollarSign className="h-8 w-8 text-gray-300" />
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-600">Items</p>
              <p className="text-xl font-bold">{order.item_count}</p>
            </div>
            <Package className="h-8 w-8 text-gray-300" />
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-600">Splits</p>
              <p className="text-xl font-bold">{order.split_count || 0}</p>
            </div>
            <Tag className="h-8 w-8 text-gray-300" />
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-600">Processing Time</p>
              <p className="text-xl font-bold">{order.duration ? `${(order.duration / 1000000).toFixed(0)}ms` : 'N/A'}</p>
            </div>
            <Clock className="h-8 w-8 text-gray-300" />
          </div>
        </div>
      </div>

      {/* Error Message */}
      {order.error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <div className="flex items-start space-x-3">
            <AlertCircle className="h-5 w-5 text-red-600 mt-0.5" />
            <div>
              <p className="font-medium text-red-900">Processing Error</p>
              <p className="text-red-700 mt-1">{order.error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Items */}
      {order.items && order.items.length > 0 && (
        <div className="bg-white rounded-lg shadow">
          <div className="px-6 py-4 border-b">
            <h2 className="text-lg font-semibold">Order Items</h2>
          </div>
          <div className="divide-y">
            {order.items.map((item: any, idx: number) => (
              <div key={idx} className="px-6 py-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <p className="font-medium">{item.name}</p>
                    <div className="flex items-center space-x-4 mt-1 text-sm text-gray-600">
                      <span>Qty: {item.quantity}</span>
                      <span>${item.unit_price?.toFixed(2)} each</span>
                      {item.category && (
                        <span className="px-2 py-1 bg-gray-100 rounded text-xs">
                          {item.category}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="text-right">
                    <p className="font-medium">${item.total_price?.toFixed(2)}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Splits */}
      {order.splits && order.splits.length > 0 && (
        <div className="bg-white rounded-lg shadow">
          <div className="px-6 py-4 border-b">
            <h2 className="text-lg font-semibold">Transaction Splits</h2>
          </div>
          <div className="divide-y">
            {order.splits.map((split: any, idx: number) => (
              <div key={idx} className="px-6 py-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">{split.category}</p>
                    <p className="text-sm text-gray-600 mt-1">{split.merchant_name}</p>
                    {split.notes && (
                      <p className="text-sm text-gray-500 mt-1">{split.notes}</p>
                    )}
                  </div>
                  <div className="text-right">
                    <p className="font-medium text-red-600">-${Math.abs(split.amount).toFixed(2)}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
          <div className="px-6 py-4 bg-gray-50 border-t">
            <div className="flex items-center justify-between font-medium">
              <span>Total</span>
              <span className="text-red-600">-${order.order_total?.toFixed(2)}</span>
            </div>
          </div>
        </div>
      )}

      {/* Metadata */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">Technical Details</h2>
        </div>
        <div className="px-6 py-4 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-gray-600">Trace ID</span>
            <span className="font-mono text-sm">{order.id}</span>
          </div>
          {order.monarch_transaction_id && (
            <div className="flex items-center justify-between">
              <span className="text-gray-600">Monarch Transaction</span>
              <span className="font-mono text-sm">{order.monarch_transaction_id}</span>
            </div>
          )}
          <div className="flex items-center justify-between">
            <span className="text-gray-600">Processed At</span>
            <span className="text-sm">{format(new Date(order.created_at), 'MMM d, yyyy h:mm a')}</span>
          </div>
          {order.dry_run && (
            <div className="flex items-center justify-between">
              <span className="text-gray-600">Mode</span>
              <span className="px-2 py-1 bg-yellow-100 text-yellow-800 rounded text-sm">Dry Run</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}