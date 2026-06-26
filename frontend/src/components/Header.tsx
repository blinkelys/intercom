import type { RuntimeStatus, SpeechDiagnosticsView } from '../types'

type HeaderProps = {
  visibleChannelName: string
  activeChannelCount: number
  keywordCount: number
  engineLabel: string
  status: RuntimeStatus
  selectedChannelId: string
  speech: SpeechDiagnosticsView
}

export function Header({ visibleChannelName, activeChannelCount, keywordCount, engineLabel, status, selectedChannelId, speech }: HeaderProps) {
  const effectiveLanguage = selectedChannelId === 'all' || selectedChannelId === 'settings'
    ? speech.lastLanguage || 'auto'
    : speech.channelLanguages[selectedChannelId] || 'auto'

  return (
    <header className="mb-8 rounded-3xl border border-white/10 bg-white/5 p-6 shadow-2xl shadow-black/20 backdrop-blur">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="text-sm uppercase tracking-[0.35em] text-orange-200/70">Session</p>
          <h2 className="mt-3 text-3xl font-semibold tracking-tight">{visibleChannelName}</h2>
        </div>
        <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
          <Metric label="Channels" value={`${activeChannelCount} active`} />
          <Metric label="Keywords" value={`${keywordCount} loaded`} />
          <Metric label="Engine" value={engineLabel} />
          <Metric label="Status" value={status} accent={statusColor(status)} />
        </div>
      </div>

      <div className="mt-5 grid grid-cols-1 gap-3 border-t border-white/10 pt-5 text-sm md:grid-cols-3">
        <Metric label="Effective Language" value={effectiveLanguage.toUpperCase()} />
        <Metric label="Decode" value={`T=${speech.temperature} · B=${speech.beamSize} · bestOf=${speech.bestOf}`} />
        <Metric label="Model/Task" value={`${speech.model} · ${speech.task}`} />
        <Metric label="Last Inference" value={speech.lastInferenceMs > 0 ? `${speech.lastInferenceMs} ms` : 'waiting'} accent={speech.lastInferenceMs > 0 ? '#22c55e' : undefined} />
        <Metric label="Last Text" value={speech.lastTextChars > 0 ? `${speech.lastTextChars} chars` : 'none yet'} />
        <Metric label="Worker Error" value={speech.lastError || 'none'} accent={speech.lastError ? '#f97316' : '#22c55e'} />
      </div>
    </header>
  )
}

type MetricProps = {
  label: string
  value: string
  accent?: string
}

function Metric({ label, value, accent }: MetricProps) {
  return (
    <div className="rounded-2xl border border-white/10 bg-black/20 px-4 py-3">
      <span className="block text-xs uppercase tracking-[0.2em] text-slate-500">{label}</span>
      <span className="mt-1 block text-sm font-medium" style={{ color: accent ?? '#f8fafc' }}>{value}</span>
    </div>
  )
}

function statusColor(status: RuntimeStatus): string {
  switch (status) {
    case 'live':
      return '#22c55e'
    case 'offline':
      return '#f97316'
    default:
      return '#facc15'
  }
}