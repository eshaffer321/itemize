'use client'

import { ChevronDownIcon } from '@heroicons/react/16/solid'
import clsx from 'clsx'
import { useState } from 'react'

interface CollapsibleSectionProps {
  title: React.ReactNode
  children: React.ReactNode
  defaultOpen?: boolean
  itemCount?: number
  className?: string
}

export function CollapsibleSection({
  title,
  children,
  defaultOpen = true,
  itemCount,
  className,
}: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div className={className}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="flex w-full items-center justify-between gap-2 rounded-lg px-1 py-2 text-left transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
      >
        <span className="flex items-center gap-3">
          {title}
          {itemCount !== undefined && (
            <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
              {itemCount}
            </span>
          )}
        </span>
        <ChevronDownIcon
          className={clsx(
            'size-5 text-zinc-400 transition-transform duration-200 dark:text-zinc-500',
            isOpen ? 'rotate-0' : '-rotate-90'
          )}
        />
      </button>
      <div
        className={clsx(
          'overflow-hidden transition-all duration-200',
          isOpen ? 'max-h-[5000px] opacity-100' : 'max-h-0 opacity-0'
        )}
      >
        {children}
      </div>
    </div>
  )
}
