import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Calendar, Search, Filter, ChevronDown } from 'lucide-react'
import OrderCard from '../components/OrderCard'
import { fetchOrders } from '../api/orders'
import { clsx } from 'clsx'

const statusOptions = ['all', 'success', 'failed', 'skipped', 'processing', 'dry-run']
const providerOptions = ['all', 'walmart', 'costco']

export default function Orders() {
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('all')
  const [provider, setProvider] = useState('all')
  const [dateRange, setDateRange] = useState({ start: '', end: '' })
  const [showFilters, setShowFilters] = useState(false)

  const { data: orders, isLoading, error } = useQuery({
    queryKey: ['orders', { search, status, provider, dateRange }],
    queryFn: () => fetchOrders({
      search,
      status: status === 'all' ? undefined : status,
      provider: provider === 'all' ? undefined : provider,
      startDate: dateRange.start || undefined,
      endDate: dateRange.end || undefined,
    }),
  })

  if (error) {
    return (
      <div className="bg-red-50 text-red-800 p-4 rounded-lg">
        Error loading orders: {(error as Error).message}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Search and Filters */}
      <div className="bg-white rounded-lg shadow p-4">
        <div className="flex items-center space-x-4">
          {/* Search */}
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
            <input
              type="text"
              placeholder="Search by order ID..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>

          {/* Filter Toggle */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={clsx(
              'flex items-center space-x-2 px-4 py-2 rounded-lg transition-colors',
              showFilters ? 'bg-primary-50 text-primary-600' : 'bg-gray-100 hover:bg-gray-200'
            )}
          >
            <Filter className="h-5 w-5" />
            <span>Filters</span>
            <ChevronDown className={clsx('h-4 w-4 transition-transform', showFilters && 'rotate-180')} />
          </button>
        </div>

        {/* Expanded Filters */}
        {showFilters && (
          <div className="mt-4 pt-4 border-t grid grid-cols-1 md:grid-cols-4 gap-4">
            {/* Status Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
              >
                {statusOptions.map(opt => (
                  <option key={opt} value={opt}>
                    {opt === 'all' ? 'All Statuses' : opt.charAt(0).toUpperCase() + opt.slice(1)}
                  </option>
                ))}
              </select>
            </div>

            {/* Provider Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Provider</label>
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
              >
                {providerOptions.map(opt => (
                  <option key={opt} value={opt}>
                    {opt === 'all' ? 'All Providers' : opt.charAt(0).toUpperCase() + opt.slice(1)}
                  </option>
                ))}
              </select>
            </div>

            {/* Date Range */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                  type="date"
                  value={dateRange.start}
                  onChange={(e) => setDateRange(prev => ({ ...prev, start: e.target.value }))}
                  className="w-full pl-10 pr-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                  type="date"
                  value={dateRange.end}
                  onChange={(e) => setDateRange(prev => ({ ...prev, end: e.target.value }))}
                  className="w-full pl-10 pr-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
                />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Results Summary */}
      <div className="flex items-center justify-between">
        <p className="text-gray-600">
          {isLoading ? 'Loading...' : `Found ${orders?.length || 0} orders`}
        </p>
      </div>

      {/* Orders List */}
      <div className="bg-white rounded-lg shadow divide-y">
        {isLoading ? (
          <div className="p-8 text-center text-gray-500">Loading orders...</div>
        ) : orders?.length === 0 ? (
          <div className="p-8 text-center text-gray-500">No orders found</div>
        ) : (
          orders?.map((order: any) => (
            <OrderCard key={order.id} order={order} />
          ))
        )}
      </div>
    </div>
  )
}