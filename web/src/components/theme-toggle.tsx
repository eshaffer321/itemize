'use client'

import { useTheme } from '@/lib/theme-context'
import { DropdownItem, DropdownLabel } from '@/components/dropdown'
import { MoonIcon, SunIcon, ComputerDesktopIcon } from '@heroicons/react/16/solid'

export function ThemeToggleItems() {
  const { theme, setTheme } = useTheme()

  return (
    <>
      <DropdownItem onClick={() => setTheme('light')}>
        <SunIcon />
        <DropdownLabel>Light</DropdownLabel>
        {theme === 'light' && <span className="ml-auto text-xs text-zinc-500">Active</span>}
      </DropdownItem>
      <DropdownItem onClick={() => setTheme('dark')}>
        <MoonIcon />
        <DropdownLabel>Dark</DropdownLabel>
        {theme === 'dark' && <span className="ml-auto text-xs text-zinc-500">Active</span>}
      </DropdownItem>
      <DropdownItem onClick={() => setTheme('system')}>
        <ComputerDesktopIcon />
        <DropdownLabel>System</DropdownLabel>
        {theme === 'system' && <span className="ml-auto text-xs text-zinc-500">Active</span>}
      </DropdownItem>
    </>
  )
}
