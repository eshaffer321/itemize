'use client'

import type { ProviderStats } from '@/lib/api'

const PROVIDER_COLORS: Record<string, { bg: string; bar: string }> = {
  walmart: { bg: 'bg-blue-100 dark:bg-blue-900/30', bar: 'bg-blue-500' },
  costco: { bg: 'bg-red-100 dark:bg-red-900/30', bar: 'bg-red-500' },
  amazon: { bg: 'bg-amber-100 dark:bg-amber-900/30', bar: 'bg-amber-500' },
}

function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount)
}

function getProviderColor(provider: string) {
  const key = provider.toLowerCase()
  return PROVIDER_COLORS[key] || { bg: 'bg-zinc-100 dark:bg-zinc-800', bar: 'bg-zinc-500' }
}

export function ProviderSpendingChart({ stats }: { stats: ProviderStats[] }) {
  if (!stats || stats.length === 0) {
    return (
      <div className="text-center text-sm text-zinc-500 dark:text-zinc-400">
        No provider data available
      </div>
    )
  }

  const maxAmount = Math.max(...stats.map((s) => s.total_amount))
  const totalSpending = stats.reduce((sum, s) => sum + s.total_amount, 0)

  // Sort by total amount descending
  const sortedStats = [...stats].sort((a, b) => b.total_amount - a.total_amount)

  return (
    <div className="space-y-4">
      {sortedStats.map((provider) => {
        const percentage = maxAmount > 0 ? (provider.total_amount / maxAmount) * 100 : 0
        const sharePercentage = totalSpending > 0 ? (provider.total_amount / totalSpending) * 100 : 0
        const colors = getProviderColor(provider.provider)

        return (
          <div key={provider.provider} className="space-y-1">
            <div className="flex items-center justify-between text-sm">
              <div className="flex items-center gap-2">
                <span className="font-medium capitalize">{provider.provider}</span>
                <span className="text-xs text-zinc-500 dark:text-zinc-400">
                  {provider.count} orders ({sharePercentage.toFixed(0)}%)
                </span>
              </div>
              <span className="font-semibold">{formatCurrency(provider.total_amount)}</span>
            </div>
            <div className={`h-3 w-full overflow-hidden rounded-full ${colors.bg}`}>
              <div
                className={`h-full rounded-full transition-all duration-500 ${colors.bar}`}
                style={{ width: `${percentage}%` }}
              />
            </div>
          </div>
        )
      })}

      <div className="mt-4 flex items-center justify-between border-t border-zinc-200 pt-4 dark:border-zinc-700">
        <span className="text-sm font-medium text-zinc-600 dark:text-zinc-400">Total</span>
        <span className="text-lg font-bold">{formatCurrency(totalSpending)}</span>
      </div>
    </div>
  )
}
