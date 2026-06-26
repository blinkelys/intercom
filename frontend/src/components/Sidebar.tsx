import type { AudioLevelsView, ChannelView } from '../types'

type SidebarProps = {
  channels: ChannelView[]
  audioLevels: AudioLevelsView
  selectedChannelId: string
  onSelectChannel: (channelId: string) => void
}

const utilityItems = [
  { id: 'all', name: 'All Channels', icon: '◌', color: '#94a3b8', description: 'Combined timeline' },
  { id: 'settings', name: 'Settings', icon: '⚙', color: '#f97316', description: 'Channels, keywords, and OSC' },
]

export function Sidebar({ channels, audioLevels, selectedChannelId, onSelectChannel }: SidebarProps) {
  return (
    <aside className="w-full border-b border-white/10 bg-black/20 backdrop-blur md:w-80 md:border-b-0 md:border-r xl:w-96">
      <div className="border-b border-white/10 px-4 py-4 sm:px-6 sm:py-6">
        <p className="text-xs uppercase tracking-[0.4em] text-orange-300/80">INTERCOM</p>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight">Live Feed</h1>
        <p className="mt-2 text-sm text-slate-400">Production comms transcription.</p>
      </div>

      <nav className="p-3 sm:p-4">
        <div className="-mx-1 flex gap-2 overflow-x-auto pb-2 md:mx-0 md:block md:overflow-visible md:pb-0">
          {utilityItems.map((channel) => (
            <SidebarButton
              key={channel.id}
              channel={channel}
              meterLevel={0}
              selected={selectedChannelId === channel.id}
              onClick={onSelectChannel}
            />
          ))}
        </div>

        <div className="mb-3 mt-4 px-2 text-xs uppercase tracking-[0.3em] text-slate-500 md:mt-6 md:px-3">Channels</div>

        <div className="-mx-1 flex gap-2 overflow-x-auto pb-1 md:mx-0 md:block md:overflow-visible md:pb-0">
          {channels.map((channel) => (
            <SidebarButton
              key={channel.id}
              channel={{ ...channel, description: channel.enabled ? `${channel.language.toUpperCase()} · ${channel.inputDevice || 'Unassigned'}` : 'Disabled' }}
              meterLevel={audioLevels[channel.id] ?? 0}
              selected={selectedChannelId === channel.id}
              onClick={onSelectChannel}
            />
          ))}
        </div>
      </nav>
    </aside>
  )
}

type SidebarButtonProps = {
  channel: {
    id: string
    name: string
    icon: string
    color: string
    description: string
  }
  meterLevel: number
  selected: boolean
  onClick: (channelId: string) => void
}

function SidebarButton({ channel, meterLevel, selected, onClick }: SidebarButtonProps) {
  const clampedLevel = Math.max(0, Math.min(1, meterLevel))

  return (
    <button
      className={[
        'mb-0 flex min-w-[240px] items-center gap-3 rounded-2xl border px-3 py-2.5 text-left transition sm:min-w-[260px] md:mb-2 md:w-full md:min-w-0 md:px-4 md:py-3',
        selected ? 'border-white/20 bg-white/12 shadow-lg shadow-black/20' : 'border-white/5 bg-white/5 hover:border-white/20 hover:bg-white/10',
      ].join(' ')}
      type="button"
      onClick={() => onClick(channel.id)}
    >
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-base sm:h-10 sm:w-10 sm:text-lg" style={{ backgroundColor: `${channel.color}22`, color: channel.color }}>
        {channel.icon}
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium text-slate-100">{channel.name}</span>
        <span className="block truncate text-xs text-slate-400">{channel.description}</span>
        {channel.id !== 'all' && channel.id !== 'settings' ? (
          <span className="mt-2 block h-1.5 w-32 rounded-full bg-black/40 sm:w-40">
            <span
              className="block h-full rounded-full bg-emerald-400 transition-all duration-100"
              style={{ width: `${Math.max(4, clampedLevel * 100)}%`, opacity: clampedLevel > 0.01 ? 1 : 0.35 }}
            />
          </span>
        ) : null}
      </span>
    </button>
  )
}