import { useMemo } from 'react'
import { createBackendClient } from './backend/createClient'
import { ChannelSettingsPanel } from './components/ChannelSettingsPanel'
import { Sidebar } from './components/Sidebar'
import { Timeline } from './components/Timeline'
import { useTranscriptRuntime } from './hooks/useTranscriptRuntime'
import { useTranscriptStore } from './store/transcriptStore'

const client = createBackendClient()

function App() {
  useTranscriptRuntime(client)

  const channels = useTranscriptStore((state) => state.channels)
  const inputDevices = useTranscriptStore((state) => state.inputDevices)
  const audioLevels = useTranscriptStore((state) => state.audioLevels)
  const entries = useTranscriptStore((state) => state.entries)
  const partials = useTranscriptStore((state) => state.partials)
  const keywords = useTranscriptStore((state) => state.keywords)
  const osc = useTranscriptStore((state) => state.osc)
  const selectedChannelId = useTranscriptStore((state) => state.selectedChannelId)
  const initialized = useTranscriptStore((state) => state.initialized)
  const savingChannel = useTranscriptStore((state) => state.savingChannel)
  const selectChannel = useTranscriptStore((state) => state.selectChannel)
  const saveChannel = useTranscriptStore((state) => state.saveChannel)
  const addChannel = useTranscriptStore((state) => state.addChannel)
  const removeChannel = useTranscriptStore((state) => state.removeChannel)
  const saveKeywords = useTranscriptStore((state) => state.saveKeywords)
  const saveOsc = useTranscriptStore((state) => state.saveOsc)

  const filteredEntries = useMemo(() => {
    if (selectedChannelId === 'all' || selectedChannelId === 'settings') {
      return entries
    }
    return entries.filter((entry) => entry.channelId === selectedChannelId)
  }, [entries, selectedChannelId])

  const filteredPartials = useMemo(() => {
    const values = Object.values(partials)
    if (selectedChannelId === 'all' || selectedChannelId === 'settings') {
      return values
    }
    return values.filter((partial) => partial.channelId === selectedChannelId)
  }, [partials, selectedChannelId])

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(249,115,22,0.16),_transparent_28%),linear-gradient(180deg,_#020617,_#111827)] text-slate-100">
      <div className="mx-auto flex min-h-screen w-full max-w-[1680px] flex-col md:flex-row">
        <Sidebar channels={channels} audioLevels={audioLevels} selectedChannelId={selectedChannelId} onSelectChannel={selectChannel} />

        <main className="min-w-0 flex-1 px-3 py-3 sm:px-4 sm:py-4 md:px-6 md:py-6 lg:px-8 lg:py-8">
          <section className="mb-4 rounded-2xl border border-white/10 bg-black/20 p-3 sm:p-4">
            <label className="mb-2 block text-xs uppercase tracking-[0.24em] text-slate-400">View Channel</label>
            <select
              className="h-11 w-full rounded-2xl border border-white/10 bg-black/30 px-4 text-sm text-slate-100 outline-none transition focus:border-orange-400"
              value={selectedChannelId}
              onChange={(event) => selectChannel(event.target.value)}
            >
              <option value="all">All Channels</option>
              <option value="settings">Settings</option>
              {channels.map((channel) => (
                <option key={channel.id} value={channel.id}>
                  {channel.name}
                </option>
              ))}
            </select>
          </section>

          {selectedChannelId === 'settings' ? (
            <ChannelSettingsPanel
              channels={channels}
              inputDevices={inputDevices}
              keywords={keywords}
              osc={osc}
              saving={savingChannel}
              onSaveChannel={(input) => saveChannel(client, input)}
              onAddChannel={(input) => addChannel(client, input)}
              onRemoveChannel={(channelId) => removeChannel(client, channelId)}
              onSaveKeywords={(rules) => saveKeywords(client, rules)}
              onSaveOsc={(input) => saveOsc(client, input)}
            />
          ) : initialized ? (
            <Timeline
              entries={filteredEntries}
              partials={filteredPartials}
              emptyLabel="No transcript activity yet for this view."
            />
          ) : (
            <section className="rounded-3xl border border-white/10 bg-slate-900/80 p-6 text-slate-400 shadow-xl shadow-black/10">
              Connecting transcript runtime...
            </section>
          )}
        </main>
      </div>
    </div>
  )
}

export default App
