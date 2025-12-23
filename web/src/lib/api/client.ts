import { Order, OrderFilters, OrderListResponse, SyncRun, SyncRunListResponse, HealthResponse, StatsResponse } from './types'

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8085'

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  })

  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`)
  }

  return response.json()
}

export async function getOrders(filters?: OrderFilters): Promise<OrderListResponse> {
  const params = new URLSearchParams()
  if (filters?.provider) params.append('provider', filters.provider)
  if (filters?.status) params.append('status', filters.status)
  if (filters?.search) params.append('search', filters.search)
  if (filters?.days_back) params.append('days_back', filters.days_back.toString())
  if (filters?.limit) params.append('limit', filters.limit.toString())
  if (filters?.offset) params.append('offset', filters.offset.toString())

  const queryString = params.toString()
  const url = `${API_BASE_URL}/api/orders${queryString ? `?${queryString}` : ''}`

  return fetchJSON<OrderListResponse>(url)
}

export async function getOrder(orderId: string): Promise<Order> {
  return fetchJSON<Order>(`${API_BASE_URL}/api/orders/${encodeURIComponent(orderId)}`)
}

export async function getSyncRuns(): Promise<SyncRunListResponse> {
  return fetchJSON<SyncRunListResponse>(`${API_BASE_URL}/api/runs`)
}

export async function getSyncRun(runId: number): Promise<SyncRun> {
  return fetchJSON<SyncRun>(`${API_BASE_URL}/api/runs/${runId}`)
}

export async function getHealth(): Promise<HealthResponse> {
  return fetchJSON<HealthResponse>(`${API_BASE_URL}/health`)
}

export async function getStats(): Promise<StatsResponse> {
  return fetchJSON<StatsResponse>(`${API_BASE_URL}/api/stats`)
}

export interface OrderStats {
  total: number
  success: number
  failed: number
  dryRun: number
  totalAmount: number
}

export async function getOrderStats(): Promise<OrderStats> {
  // Fetch counts for each status in parallel
  const [allOrders, successOrders, failedOrders, dryRunOrders] = await Promise.all([
    getOrders({ limit: 100 }), // Get more orders to calculate total amount
    getOrders({ status: 'success', limit: 1 }),
    getOrders({ status: 'failed', limit: 1 }),
    getOrders({ status: 'dry-run', limit: 1 }),
  ])

  const totalAmount = allOrders.orders.reduce((sum, order) => {
    // Only count successful syncs for total amount
    if (order.status === 'success') {
      return sum + order.order_total
    }
    return sum
  }, 0)

  return {
    total: allOrders.total_count,
    success: successOrders.total_count,
    failed: failedOrders.total_count,
    dryRun: dryRunOrders.total_count,
    totalAmount,
  }
}
