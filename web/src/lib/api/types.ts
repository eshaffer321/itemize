export interface OrderItem {
  name: string
  quantity: number
  unit_price: number
  total_price: number
  category?: string
}

export interface OrderSplit {
  category_id: string
  category_name: string
  amount: number
  items?: OrderItem[]
  notes?: string
}

export interface Order {
  order_id: string
  provider: string
  transaction_id?: string
  order_date: string
  processed_at: string
  order_total: number
  order_subtotal: number
  order_tax: number
  order_tip?: number
  transaction_amount: number
  status: string
  error_message?: string
  item_count: number
  split_count: number
  match_confidence: number
  dry_run: boolean
  items?: OrderItem[]
  splits?: OrderSplit[]
}

export interface OrderListResponse {
  orders: Order[]
  total_count: number
  limit: number
  offset: number
}

export interface SyncRun {
  id: number
  provider: string
  started_at: string
  completed_at?: string
  lookback_days: number
  dry_run: boolean
  orders_found: number
  orders_processed: number
  orders_skipped: number
  orders_errored: number
  status: string
}

export interface SyncRunListResponse {
  runs: SyncRun[]
  count: number
}

export interface HealthResponse {
  status: string
  timestamp: string
}

export interface OrderFilters {
  provider?: string
  status?: string
  search?: string
  days_back?: number
  limit?: number
  offset?: number
}

export interface ProviderStats {
  provider: string
  count: number
  success_count: number
  total_amount: number
}

export interface StatsResponse {
  total_processed: number
  success_count: number
  failed_count: number
  skipped_count: number
  dry_run_count: number
  total_amount: number
  average_order_amount: number
  total_splits: number
  provider_stats: ProviderStats[]
}

// Sync Job Types
export interface StartSyncRequest {
  provider: 'walmart' | 'costco' | 'amazon'
  dry_run?: boolean
  lookback_days?: number
  max_orders?: number
  verbose?: boolean
  order_id?: string
  force?: boolean
}

export interface StartSyncResponse {
  job_id: string
  message: string
}

export interface SyncJobProgress {
  current_phase: string
  total_orders: number
  processed_orders: number
  skipped_orders: number
  errored_orders: number
}

export interface SyncJobResult {
  processed_count: number
  skipped_count: number
  error_count: number
}

export interface SyncJob {
  job_id: string
  provider: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  dry_run: boolean
  progress: SyncJobProgress
  request?: StartSyncRequest
  started_at: string
  completed_at?: string
  result?: SyncJobResult
  error?: string
}

export interface SyncJobListResponse {
  jobs: SyncJob[]
  count: number
}

// Ledger Types
export interface LedgerCharge {
  id: number
  order_ledger_id: number
  order_id: string
  sync_run_id?: number
  charge_sequence: number
  charge_amount: number
  charge_type: string
  payment_method: string
  card_type?: string
  card_last_four?: string
  charged_at?: string
  monarch_transaction_id?: string
  is_matched: boolean
  match_confidence?: number
  matched_at?: string
  split_count?: number
}

export interface Ledger {
  id: number
  order_id: string
  sync_run_id?: number
  provider: string
  fetched_at: string
  ledger_state: string
  ledger_version: number
  total_charged: number
  charge_count: number
  payment_method_types: string
  has_refunds: boolean
  is_valid: boolean
  validation_notes?: string
  charges?: LedgerCharge[]
}

export interface LedgerListResponse {
  ledgers: Ledger[]
  total_count: number
  limit: number
  offset: number
}

// Transaction Types (from Monarch Money)
export interface TransactionMerchant {
  id: string
  name: string
}

export interface TransactionAccount {
  id: string
  display_name: string
  mask?: string
  logo_url?: string
}

export interface CategoryGroup {
  id: string
  name: string
  type: string
}

export interface TransactionCategory {
  id: string
  name: string
  icon?: string
  is_system_category: boolean
  group?: CategoryGroup
}

export interface TransactionTag {
  id: string
  name: string
  color?: string
}

export interface Transaction {
  id: string
  date: string
  amount: number
  pending: boolean
  hide_from_reports: boolean
  plaid_name?: string
  merchant?: TransactionMerchant
  notes?: string
  has_splits: boolean
  is_recurring: boolean
  needs_review: boolean
  reviewed_at?: string
  created_at: string
  updated_at: string
  account?: TransactionAccount
  category?: TransactionCategory
  tags?: TransactionTag[]
}

export interface TransactionSplit {
  id: string
  amount: number
  merchant?: TransactionMerchant
  notes?: string
  category?: TransactionCategory
}

export interface TransactionDetail extends Transaction {
  original_merchant?: string
  original_category?: TransactionCategory
  splits?: TransactionSplit[]
}

export interface TransactionListResponse {
  transactions: Transaction[]
  total_count: number
  limit: number
  offset: number
  has_more: boolean
}

export interface TransactionFilters {
  search?: string
  days_back?: number
  pending?: boolean
  limit?: number
  offset?: number
}
