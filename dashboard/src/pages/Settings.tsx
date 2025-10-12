import { useState } from 'react'
import { Save, AlertCircle, CheckCircle, Loader2 } from 'lucide-react'
import { clsx } from 'clsx'

export default function Settings() {
  const [settings, setSettings] = useState({
    monarchApiKey: '',
    openaiApiKey: '',
    walmartEmail: '',
    dryRunMode: true,
    autoSync: false,
    syncInterval: '24',
    matchTolerance: '0.50',
    tipPercentage: '20',
    dateRangeDays: '3',
  })

  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null)

  const handleSave = async () => {
    setSaving(true)
    setMessage(null)
    
    try {
      // TODO: Implement API call to save settings
      await new Promise(resolve => setTimeout(resolve, 1000))
      setMessage({ type: 'success', text: 'Settings saved successfully!' })
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to save settings. Please try again.' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Message */}
      {message && (
        <div className={clsx(
          'flex items-center space-x-3 p-4 rounded-lg',
          message.type === 'success' ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'
        )}>
          {message.type === 'success' ? (
            <CheckCircle className="h-5 w-5" />
          ) : (
            <AlertCircle className="h-5 w-5" />
          )}
          <span>{message.text}</span>
        </div>
      )}

      {/* API Keys */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">API Configuration</h2>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Monarch Money API Key
            </label>
            <input
              type="password"
              value={settings.monarchApiKey}
              onChange={(e) => setSettings(prev => ({ ...prev, monarchApiKey: e.target.value }))}
              placeholder="Enter your Monarch Money API key"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Required for syncing transactions with Monarch Money
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              OpenAI API Key
            </label>
            <input
              type="password"
              value={settings.openaiApiKey}
              onChange={(e) => setSettings(prev => ({ ...prev, openaiApiKey: e.target.value }))}
              placeholder="Enter your OpenAI API key"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Required for AI-powered categorization
            </p>
          </div>
        </div>
      </div>

      {/* Provider Settings */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">Provider Settings</h2>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Walmart Email
            </label>
            <input
              type="email"
              value={settings.walmartEmail}
              onChange={(e) => setSettings(prev => ({ ...prev, walmartEmail: e.target.value }))}
              placeholder="your.email@example.com"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Email associated with your Walmart account
            </p>
          </div>
        </div>
      </div>

      {/* Sync Settings */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">Sync Configuration</h2>
        </div>
        <div className="p-6 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium text-gray-700">Dry Run Mode</label>
              <p className="text-sm text-gray-500">Test syncing without making actual changes</p>
            </div>
            <button
              onClick={() => setSettings(prev => ({ ...prev, dryRunMode: !prev.dryRunMode }))}
              className={clsx(
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                settings.dryRunMode ? 'bg-primary-600' : 'bg-gray-200'
              )}
            >
              <span
                className={clsx(
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  settings.dryRunMode ? 'translate-x-6' : 'translate-x-1'
                )}
              />
            </button>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <label className="text-sm font-medium text-gray-700">Auto Sync</label>
              <p className="text-sm text-gray-500">Automatically sync orders periodically</p>
            </div>
            <button
              onClick={() => setSettings(prev => ({ ...prev, autoSync: !prev.autoSync }))}
              className={clsx(
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                settings.autoSync ? 'bg-primary-600' : 'bg-gray-200'
              )}
            >
              <span
                className={clsx(
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  settings.autoSync ? 'translate-x-6' : 'translate-x-1'
                )}
              />
            </button>
          </div>

          {settings.autoSync && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Sync Interval (hours)
              </label>
              <input
                type="number"
                value={settings.syncInterval}
                onChange={(e) => setSettings(prev => ({ ...prev, syncInterval: e.target.value }))}
                min="1"
                max="168"
                className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>
          )}
        </div>
      </div>

      {/* Matching Settings */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">Transaction Matching</h2>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Amount Tolerance ($)
            </label>
            <input
              type="number"
              value={settings.matchTolerance}
              onChange={(e) => setSettings(prev => ({ ...prev, matchTolerance: e.target.value }))}
              min="0"
              step="0.01"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Maximum difference allowed when matching transactions
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Tip Percentage (%)
            </label>
            <input
              type="number"
              value={settings.tipPercentage}
              onChange={(e) => setSettings(prev => ({ ...prev, tipPercentage: e.target.value }))}
              min="0"
              max="100"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Expected tip percentage for delivery orders
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Date Range (days)
            </label>
            <input
              type="number"
              value={settings.dateRangeDays}
              onChange={(e) => setSettings(prev => ({ ...prev, dateRangeDays: e.target.value }))}
              min="1"
              max="30"
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
            <p className="mt-1 text-sm text-gray-500">
              Number of days before/after order date to search for transactions
            </p>
          </div>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="flex items-center space-x-2 px-6 py-3 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? (
            <Loader2 className="h-5 w-5 animate-spin" />
          ) : (
            <Save className="h-5 w-5" />
          )}
          <span>{saving ? 'Saving...' : 'Save Settings'}</span>
        </button>
      </div>
    </div>
  )
}