import { useMemo } from 'react'
import { createBackendClient } from './backend/createClient'
import { ChannelSettingsPanel } from './components/ChannelSettingsPanel'
import { Header } from './components/Header'
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
  const speech = useTranscriptStore((state) => state.speech)
  const selectedChannelId = useTranscriptStore((state) => state.selectedChannelId)
  const status = useTranscriptStore((state) => state.status)
  const engineLabel = useTranscriptStore((state) => state.engineLabel)
  const keywordCount = useTranscriptStore((state) => state.keywordCount)
  const initialized = useTranscriptStore((state) => state.initialized)
  const savingChannel = useTranscriptStore((state) => state.savingChannel)
  const selectChannel = useTranscriptStore((state) => state.selectChannel)
  const saveChannel = useTranscriptStore((state) => state.saveChannel)

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

  const visibleChannelName = useMemo(() => {
    if (selectedChannelId === 'all') {
      return 'Current Production Feed'
    }
    if (selectedChannelId === 'settings') {
      return 'Settings Preview'
    }
    return channels.find((channel) => channel.id === selectedChannelId)?.name ?? 'Current Production Feed'
  }, [channels, selectedChannelId])

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(249,115,22,0.16),_transparent_28%),linear-gradient(180deg,_#020617,_#111827)] text-slate-100">
      <div className="mx-auto flex min-h-screen max-w-[1440px] flex-col md:flex-row">
        <Sidebar channels={channels} audioLevels={audioLevels} selectedChannelId={selectedChannelId} onSelectChannel={selectChannel} />

        <main className="flex-1 px-4 py-6 md:px-8 md:py-8">
          <Header
            visibleChannelName={visibleChannelName}
            activeChannelCount={channels.filter((channel) => channel.enabled).length}
            keywordCount={keywordCount}
            engineLabel={engineLabel}
            status={status}
            selectedChannelId={selectedChannelId}
            speech={speech}
          />

          {selectedChannelId === 'settings' ? (
            <ChannelSettingsPanel channels={channels} inputDevices={inputDevices} saving={savingChannel} onSave={(input) => saveChannel(client, input)} />
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
