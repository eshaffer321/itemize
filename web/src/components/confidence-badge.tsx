import { CheckCircleIcon, ExclamationTriangleIcon, QuestionMarkCircleIcon } from '@heroicons/react/16/solid'
import clsx from 'clsx'

type ConfidenceLevel = 'high' | 'medium' | 'low'

interface ConfidenceBadgeProps {
  confidence: number // 0-1 scale
  showPercentage?: boolean
  showIcon?: boolean
  size?: 'sm' | 'md'
  className?: string
}

function getConfidenceLevel(confidence: number): ConfidenceLevel {
  if (confidence >= 0.9) return 'high'
  if (confidence >= 0.7) return 'medium'
  return 'low'
}

function getConfidenceLabel(level: ConfidenceLevel): string {
  switch (level) {
    case 'high':
      return 'High'
    case 'medium':
      return 'Review'
    case 'low':
      return 'Low'
  }
}

const levelStyles: Record<ConfidenceLevel, string> = {
  high: 'bg-green-500/15 text-green-700 dark:bg-green-500/10 dark:text-green-400',
  medium: 'bg-amber-400/20 text-amber-700 dark:bg-amber-400/10 dark:text-amber-400',
  low: 'bg-red-500/15 text-red-700 dark:bg-red-500/10 dark:text-red-400',
}

const iconStyles: Record<ConfidenceLevel, string> = {
  high: 'text-green-600 dark:text-green-400',
  medium: 'text-amber-600 dark:text-amber-400',
  low: 'text-red-600 dark:text-red-400',
}

const LevelIcon = ({ level, className }: { level: ConfidenceLevel; className?: string }) => {
  const iconClass = clsx('size-4', iconStyles[level], className)
  switch (level) {
    case 'high':
      return <CheckCircleIcon className={iconClass} />
    case 'medium':
      return <ExclamationTriangleIcon className={iconClass} />
    case 'low':
      return <QuestionMarkCircleIcon className={iconClass} />
  }
}

export function ConfidenceBadge({
  confidence,
  showPercentage = true,
  showIcon = true,
  size = 'sm',
  className,
}: ConfidenceBadgeProps) {
  const level = getConfidenceLevel(confidence)
  const percentage = Math.round(confidence * 100)

  return (
    <span
      className={clsx(
        'inline-flex items-center gap-x-1 rounded-md font-medium',
        size === 'sm' ? 'px-1.5 py-0.5 text-xs' : 'px-2 py-1 text-sm',
        levelStyles[level],
        className
      )}
      title={`Match confidence: ${percentage}%`}
    >
      {showIcon && <LevelIcon level={level} className={size === 'sm' ? 'size-3.5' : 'size-4'} />}
      {showPercentage ? `${percentage}%` : getConfidenceLabel(level)}
    </span>
  )
}

// Standalone progress bar for more detailed views
export function ConfidenceBar({
  confidence,
  className,
  showLabel = false,
}: {
  confidence: number
  className?: string
  showLabel?: boolean
}) {
  const level = getConfidenceLevel(confidence)
  const percentage = Math.round(confidence * 100)

  const barColors: Record<ConfidenceLevel, string> = {
    high: 'bg-green-500',
    medium: 'bg-amber-500',
    low: 'bg-red-500',
  }

  return (
    <div className={clsx('flex items-center gap-2', className)}>
      <div className="h-2 w-24 overflow-hidden rounded-full bg-zinc-300 dark:bg-zinc-600">
        <div
          className={clsx('h-full rounded-full transition-all', barColors[level])}
          style={{ width: `${Math.max(percentage, 2)}%` }}
        />
      </div>
      {showLabel && (
        <span className="text-xs tabular-nums text-zinc-500 dark:text-zinc-400">{percentage}%</span>
      )}
    </div>
  )
}
