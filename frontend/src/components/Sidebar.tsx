import type { AudioLevelsView, ChannelView } from '../types'

type SidebarProps = {
  channels: ChannelView[]
  audioLevels: AudioLevelsView
  selectedChannelId: string
  onSelectChannel: (channelId: string) => void
}

const utilityItems = [
  { id: 'all', name: 'All Channels', icon: '◌', color: '#94a3b8', description: 'Combined timeline' },
  { id: 'settings', name: 'Settings', icon: '⚙', color: '#f97316', description: 'Configuration view' },
]

export function Sidebar({ channels, audioLevels, selectedChannelId, onSelectChannel }: SidebarProps) {
  return (
    <aside className="w-full border-b border-white/10 bg-black/20 backdrop-blur md:w-80 md:border-b-0 md:border-r">
      <div className="border-b border-white/10 px-6 py-6">
        <p className="text-xs uppercase tracking-[0.4em] text-orange-300/80">PROCOM</p>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight">Live Transcript</h1>
        <p className="mt-2 text-sm text-slate-400">Offline channel monitoring for production communication.</p>
      </div>

      <nav className="p-4">
        {utilityItems.map((channel) => (
          <SidebarButton
            key={channel.id}
            channel={channel}
            meterLevel={0}
            selected={selectedChannelId === channel.id}
            onClick={onSelectChannel}
          />
        ))}

        <div className="mb-3 mt-6 px-3 text-xs uppercase tracking-[0.3em] text-slate-500">Channels</div>

        {channels.map((channel) => (
          <SidebarButton
            key={channel.id}
            channel={{ ...channel, description: channel.enabled ? `${channel.language.toUpperCase()} · ${channel.inputDevice || 'Unassigned'}` : 'Disabled' }}
            meterLevel={audioLevels[channel.id] ?? 0}
            selected={selectedChannelId === channel.id}
            onClick={onSelectChannel}
          />
        ))}
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
        'mb-2 flex w-full items-center gap-3 rounded-2xl border px-4 py-3 text-left transition',
        selected ? 'border-white/20 bg-white/12 shadow-lg shadow-black/20' : 'border-white/5 bg-white/5 hover:border-white/20 hover:bg-white/10',
      ].join(' ')}
      type="button"
      onClick={() => onClick(channel.id)}
    >
      <span className="flex h-10 w-10 items-center justify-center rounded-xl text-lg" style={{ backgroundColor: `${channel.color}22`, color: channel.color }}>
        {channel.icon}
      </span>
      <span>
        <span className="block text-sm font-medium text-slate-100">{channel.name}</span>
        <span className="block text-xs text-slate-400">{channel.description}</span>
        {channel.id !== 'all' && channel.id !== 'settings' ? (
          <span className="mt-2 block h-1.5 w-40 rounded-full bg-black/40">
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