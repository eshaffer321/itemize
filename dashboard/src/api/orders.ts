const API_BASE = import.meta.env.VITE_API_URL || '/api'

export interface Order {
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
  duration?: number
  created_at: string
  monarch_transaction_id?: string
  dry_run?: boolean
  items?: OrderItem[]
  splits?: TransactionSplit[]
}

export interface OrderItem {
  name: string
  quantity: number
  unit_price: number
  total_price: number
  category?: string
}

export interface TransactionSplit {
  category: string
  merchant_name: string
  amount: number
  notes?: string
}

export interface Stats {
  total_traces: number
  success_count: number
  failure_count: number
  skipped_count: number
  processing_count: number
  total_amount: number
  avg_duration: number
  category_breakdown: Record<string, number>
}

export interface OrderFilters {
  search?: string
  status?: string
  provider?: string
  startDate?: string
  endDate?: string
  limit?: number
}

async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!response.ok) {
    throw new Error(`API request failed: ${response.statusText}`)
  }

  return response.json()
}

export async function fetchStats(): Promise<Stats> {
  return fetchJson<Stats>(`${API_BASE}/stats`)
}

export async function fetchRecentOrders(limit = 10): Promise<Order[]> {
  return fetchJson<Order[]>(`${API_BASE}/orders/recent?limit=${limit}`)
}

export async function fetchOrders(filters: OrderFilters = {}): Promise<Order[]> {
  const params = new URLSearchParams()
  
  if (filters.search) params.append('search', filters.search)
  if (filters.status) params.append('status', filters.status)
  if (filters.provider) params.append('provider', filters.provider)
  if (filters.startDate) params.append('start_date', filters.startDate)
  if (filters.endDate) params.append('end_date', filters.endDate)
  if (filters.limit) params.append('limit', filters.limit.toString())

  return fetchJson<Order[]>(`${API_BASE}/orders?${params}`)
}

export async function fetchOrderDetail(orderId: string): Promise<Order> {
  return fetchJson<Order>(`${API_BASE}/orders/${orderId}`)
}

export async function syncOrders(provider?: string, dryRun = false): Promise<{ message: string; count: number }> {
  const params = new URLSearchParams()
  if (provider) params.append('provider', provider)
  if (dryRun) params.append('dry_run', 'true')

  return fetchJson(`${API_BASE}/sync?${params}`, {
    method: 'POST',
  })
}

export async function retryOrder(orderId: string): Promise<Order> {
  return fetchJson<Order>(`${API_BASE}/orders/${orderId}/retry`, {
    method: 'POST',
  })
}

export async function deleteOrder(orderId: string): Promise<{ message: string }> {
  return fetchJson(`${API_BASE}/orders/${orderId}`, {
    method: 'DELETE',
  })
}