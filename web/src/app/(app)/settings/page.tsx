import { Badge } from '@/components/badge'
import { Divider } from '@/components/divider'
import { Heading, Subheading } from '@/components/heading'
import { Text } from '@/components/text'
import {
  ArrowPathIcon,
  CheckCircleIcon,
  ClipboardDocumentIcon,
  CommandLineIcon,
  Cog6ToothIcon,
  ServerIcon,
} from '@heroicons/react/20/solid'
import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Quick Start',
}

function CodeBlock({ children, copyable = true }: { children: string; copyable?: boolean }) {
  return (
    <div className="group relative rounded-lg bg-zinc-900 p-4 dark:bg-zinc-950">
      <code className="text-sm text-zinc-100">{children}</code>
      {copyable && (
        <button
          className="absolute right-2 top-2 rounded p-1 opacity-0 transition hover:bg-zinc-700 group-hover:opacity-100"
          title="Copy to clipboard"
        >
          <ClipboardDocumentIcon className="size-4 text-zinc-400" />
        </button>
      )}
    </div>
  )
}

function Step({
  number,
  title,
  description,
  children,
}: {
  number: number
  title: string
  description: string
  children: React.ReactNode
}) {
  return (
    <div className="relative pl-12">
      <div className="absolute left-0 top-0 flex size-8 items-center justify-center rounded-full bg-blue-600 text-sm font-semibold text-white">
        {number}
      </div>
      <div className="space-y-3">
        <div>
          <h3 className="font-semibold text-zinc-900 dark:text-white">{title}</h3>
          <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">{description}</p>
        </div>
        {children}
      </div>
    </div>
  )
}

export default function Settings() {
  return (
    <div className="mx-auto max-w-4xl">
      <Heading>Quick Start</Heading>
      <Text className="mt-2">
        Retail Sync is a CLI tool that syncs your retail purchases with Monarch Money. Here&apos;s how to get started.
      </Text>

      <Divider className="my-10" />

      <div className="space-y-12">
        <Step
          number={1}
          title="Configure Environment"
          description="Set your API tokens as environment variables or in config.yaml"
        >
          <CodeBlock>
            {`export MONARCH_TOKEN="your-monarch-api-token"
export OPENAI_APIKEY="your-openai-api-key"`}
          </CodeBlock>
        </Step>

        <Step number={2} title="Start the API Server" description="Run the API server to enable this dashboard">
          <CodeBlock>./monarch-sync serve -port 8085</CodeBlock>
          <div className="mt-2 flex items-center gap-2">
            <ServerIcon className="size-4 text-zinc-400" />
            <Text className="text-sm">The dashboard connects to http://localhost:8085</Text>
          </div>
        </Step>

        <Step
          number={3}
          title="Run Your First Sync"
          description="Sync orders from your providers. Use -dry-run to preview without making changes."
        >
          <div className="space-y-3">
            <div>
              <div className="mb-1 flex items-center gap-2">
                <Badge color="blue">Walmart</Badge>
              </div>
              <CodeBlock>./monarch-sync walmart -days 14 -dry-run</CodeBlock>
            </div>
            <div>
              <div className="mb-1 flex items-center gap-2">
                <Badge color="zinc">Costco</Badge>
              </div>
              <CodeBlock>./monarch-sync costco -days 7 -dry-run</CodeBlock>
            </div>
            <div>
              <div className="mb-1 flex items-center gap-2">
                <Badge color="amber">Amazon</Badge>
              </div>
              <CodeBlock>./monarch-sync amazon -days 14 -dry-run</CodeBlock>
            </div>
          </div>
        </Step>

        <Step number={4} title="Apply Changes" description="Once you're happy with the dry run, remove the flag to apply">
          <CodeBlock>./monarch-sync walmart -days 14</CodeBlock>
        </Step>
      </div>

      <Divider className="my-10" />

      <Subheading>Common Options</Subheading>
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <CommandLineIcon className="size-5 text-zinc-500" />
            <span className="font-mono text-sm">-dry-run</span>
          </div>
          <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
            Preview changes without modifying Monarch transactions
          </p>
        </div>
        <div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <ArrowPathIcon className="size-5 text-zinc-500" />
            <span className="font-mono text-sm">-force</span>
          </div>
          <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">Reprocess orders even if already synced</p>
        </div>
        <div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <Cog6ToothIcon className="size-5 text-zinc-500" />
            <span className="font-mono text-sm">-verbose</span>
          </div>
          <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">Show detailed logging for debugging</p>
        </div>
        <div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <CheckCircleIcon className="size-5 text-zinc-500" />
            <span className="font-mono text-sm">-max N</span>
          </div>
          <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">Limit processing to N orders per run</p>
        </div>
      </div>

      <Divider className="my-10" />

      <Subheading>How It Works</Subheading>
      <div className="mt-4 rounded-lg border border-zinc-200 p-6 dark:border-zinc-700">
        <ol className="space-y-4 text-sm text-zinc-600 dark:text-zinc-400">
          <li className="flex gap-3">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-zinc-100 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
              1
            </span>
            <span>
              <strong className="text-zinc-900 dark:text-white">Fetch orders</strong> from your provider (Walmart,
              Costco, Amazon) including item details
            </span>
          </li>
          <li className="flex gap-3">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-zinc-100 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
              2
            </span>
            <span>
              <strong className="text-zinc-900 dark:text-white">Match to Monarch transactions</strong> using fuzzy
              matching (amount ± $0.50, date ± 5 days)
            </span>
          </li>
          <li className="flex gap-3">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-zinc-100 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
              3
            </span>
            <span>
              <strong className="text-zinc-900 dark:text-white">Categorize items</strong> using OpenAI GPT-4 (results
              cached to minimize API costs)
            </span>
          </li>
          <li className="flex gap-3">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-zinc-100 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
              4
            </span>
            <span>
              <strong className="text-zinc-900 dark:text-white">Create splits</strong> in Monarch with proportional tax
              allocation by category
            </span>
          </li>
        </ol>
      </div>
    </div>
  )
}
