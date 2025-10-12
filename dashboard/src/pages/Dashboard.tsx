import { useQuery } from '@tanstack/react-query'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import { TrendingUp, Package, CheckCircle, XCircle, Clock, DollarSign } from 'lucide-react'
import StatCard from '../components/StatCard'
import { fetchStats, fetchRecentOrders } from '../api/orders'
import OrderCard from '../components/OrderCard'

const COLORS = ['#10b981', '#f59e0b', '#ef4444', '#6366f1']

export default function Dashboard() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: fetchStats,
    refetchInterval: 30000,
  })

  const { data: recentOrders, isLoading: ordersLoading } = useQuery({
    queryKey: ['recentOrders'],
    queryFn: () => fetchRecentOrders(10),
  })

  if (statsLoading || ordersLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading dashboard...</div>
      </div>
    )
  }

  const pieData = [
    { name: 'Success', value: stats?.success_count || 0 },
    { name: 'Skipped', value: stats?.skipped_count || 0 },
    { name: 'Failed', value: stats?.failure_count || 0 },
    { name: 'Processing', value: stats?.processing_count || 0 },
  ]

  const categoryData = Object.entries(stats?.category_breakdown || {}).map(([name, count]) => ({
    name: name.slice(0, 15),
    count,
  }))

  return (
    <div className="space-y-6">
      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Total Orders"
          value={stats?.total_traces || 0}
          icon={Package}
          trend={12}
          color="blue"
        />
        <StatCard
          title="Success Rate"
          value={`${((stats?.success_count || 0) / (stats?.total_traces || 1) * 100).toFixed(1)}%`}
          icon={CheckCircle}
          trend={5}
          color="green"
        />
        <StatCard
          title="Total Amount"
          value={`$${(stats?.total_amount || 0).toFixed(2)}`}
          icon={DollarSign}
          color="purple"
        />
        <StatCard
          title="Avg Processing"
          value={`${((stats?.avg_duration || 0) / 1000000).toFixed(0)}ms`}
          icon={Clock}
          color="yellow"
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Status Distribution */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-4">Order Status Distribution</h3>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie
                data={pieData}
                cx="50%"
                cy="50%"
                labelLine={false}
                label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                outerRadius={80}
                fill="#8884d8"
                dataKey="value"
              >
                {pieData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/* Category Breakdown */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-4">Top Categories</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={categoryData.slice(0, 8)}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="name" angle={-45} textAnchor="end" height={80} />
              <YAxis />
              <Tooltip />
              <Bar dataKey="count" fill="#8b5cf6" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Recent Orders */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h3 className="text-lg font-semibold">Recent Orders</h3>
        </div>
        <div className="divide-y">
          {recentOrders?.map((order: any) => (
            <OrderCard key={order.id} order={order} />
          ))}
        </div>
      </div>
    </div>
  )
}