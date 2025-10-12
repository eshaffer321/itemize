import { Link } from 'react-router-dom'
import { format } from 'date-fns'
import { clsx } from 'clsx'
import { Package, DollarSign, Tag, AlertCircle } from 'lucide-react'

interface OrderCardProps {
  order: {
    id: string
    order_id: string
    provider: string
    status: string
    order_date: string
    order_total: number
    item_count: number
    split_count: number
    categories?: string[]
    error?: string
  }
}

const statusColors = {
  success: 'bg-green-100 text-green-800',
  failed: 'bg-red-100 text-red-800',
  skipped: 'bg-yellow-100 text-yellow-800',
  processing: 'bg-blue-100 text-blue-800',
  'dry-run': 'bg-purple-100 text-purple-800',
}

export default function OrderCard({ order }: OrderCardProps) {
  return (
    <Link
      to={`/orders/${order.order_id}`}
      className="block p-4 hover:bg-gray-50 transition-colors"
    >
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <div className="flex items-center space-x-3">
            <Package className="h-5 w-5 text-gray-400" />
            <span className="font-medium">{order.order_id}</span>
            <span className={clsx('px-2 py-1 rounded-full text-xs font-medium', statusColors[order.status] || 'bg-gray-100')}>
              {order.status}
            </span>
          </div>
          
          <div className="mt-2 flex items-center space-x-4 text-sm text-gray-600">
            <span className="flex items-center">
              <DollarSign className="h-4 w-4 mr-1" />
              ${order.order_total?.toFixed(2)}
            </span>
            <span>{order.item_count} items</span>
            {order.split_count > 0 && (
              <span>{order.split_count} splits</span>
            )}
            <span>{format(new Date(order.order_date), 'MMM d, yyyy')}</span>
          </div>
          
          {order.categories && order.categories.length > 0 && (
            <div className="mt-2 flex items-center space-x-2">
              <Tag className="h-4 w-4 text-gray-400" />
              <div className="flex flex-wrap gap-1">
                {order.categories.slice(0, 3).map((cat, idx) => (
                  <span key={idx} className="px-2 py-1 bg-gray-100 rounded text-xs">
                    {cat}
                  </span>
                ))}
                {order.categories.length > 3 && (
                  <span className="px-2 py-1 text-xs text-gray-500">
                    +{order.categories.length - 3} more
                  </span>
                )}
              </div>
            </div>
          )}
          
          {order.error && (
            <div className="mt-2 flex items-start space-x-2 text-sm text-red-600">
              <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
              <span className="line-clamp-1">{order.error}</span>
            </div>
          )}
        </div>
        
        <div className="ml-4">
          <span className="text-gray-400">→</span>
        </div>
      </div>
    </Link>
  )
}