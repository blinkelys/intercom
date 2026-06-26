import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import type { ChannelUpdateInput, ChannelView, InputDeviceView } from '../types'

type ChannelSettingsPanelProps = {
  channels: ChannelView[]
  inputDevices: InputDeviceView[]
  saving: boolean
  onSave: (input: ChannelUpdateInput) => Promise<void>
}

export function ChannelSettingsPanel({ channels, inputDevices, saving, onSave }: ChannelSettingsPanelProps) {
  const [activeChannelId, setActiveChannelId] = useState<string>(channels[0]?.id ?? '')
  const [draft, setDraft] = useState<ChannelUpdateInput | null>(null)

  useEffect(() => {
    if (!channels.some((channel) => channel.id === activeChannelId)) {
      setActiveChannelId(channels[0]?.id ?? '')
    }
  }, [activeChannelId, channels])

  const activeChannel = channels.find((item) => item.id === activeChannelId)

  useEffect(() => {
    if (!activeChannel) {
      setDraft(null)
      return
    }

    // Preserve unsaved edits while live runtime updates stream in.
    setDraft((current) => (current && current.id === activeChannel.id ? current : toDraft(activeChannel)))
  }, [activeChannel, activeChannelId])

  if (!draft) {
    return (
      <section className="rounded-3xl border border-white/10 bg-slate-900/80 p-6 text-slate-400 shadow-xl shadow-black/10">
        No channels configured.
      </section>
    )
  }

  const inputDeviceOptions = draft.inputDevice && !inputDevices.some((device) => device.id === draft.inputDevice)
    ? [{ id: draft.inputDevice, name: draft.inputDevice }, ...inputDevices]
    : inputDevices

  return (
    <section className="rounded-3xl border border-white/10 bg-slate-900/80 p-6 shadow-xl shadow-black/10">
      <h3 className="text-lg font-semibold text-slate-100">Channel Settings</h3>

      <form
        className="mt-5 grid gap-4 md:grid-cols-2"
        onSubmit={async (event) => {
          event.preventDefault()
          await onSave(draft)
        }}
      >
        <Field label="Channel">
          <select className={inputClassName} value={activeChannelId} onChange={(event) => setActiveChannelId(event.target.value)}>
            {channels.map((channel) => (
              <option key={channel.id} value={channel.id}>
                {channel.name}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Display Name">
          <input className={inputClassName} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        </Field>

        <Field label="Language">
          <select className={inputClassName} value={draft.language} onChange={(event) => setDraft({ ...draft, language: event.target.value })}>
            <option value="en">English</option>
            <option value="no">Norwegian</option>
          </select>
        </Field>

        <Field label="Input Channel">
          <select className={inputClassName} value={draft.inputDevice} onChange={(event) => setDraft({ ...draft, inputDevice: event.target.value })}>
            <option value="">Unassigned</option>
            {inputDeviceOptions.map((device) => (
              <option key={device.id} value={device.id}>
                {device.name}
              </option>
            ))}
          </select>
        </Field>

        <Field label="State">
          <label className="flex h-11 items-center gap-3 rounded-2xl border border-white/10 bg-black/20 px-4 text-sm text-slate-200">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} />
            Enabled
          </label>
        </Field>

        <div className="md:col-span-2 flex justify-end rounded-2xl border border-white/10 bg-black/20 px-4 py-4">
          <button className="rounded-2xl bg-orange-500 px-5 py-3 text-sm font-medium text-slate-950 transition hover:bg-orange-400 disabled:cursor-not-allowed disabled:bg-slate-600" type="submit" disabled={saving}>
            {saving ? 'Saving…' : 'Save Channel'}
          </button>
        </div>
      </form>
    </section>
  )
}

function toDraft(channel: ChannelView): ChannelUpdateInput {
  return {
    id: channel.id,
    name: channel.name,
    color: channel.color,
    icon: channel.icon,
    inputDevice: channel.inputDevice,
    language: channel.language,
    enabled: channel.enabled,
  }
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-500">{label}</span>
      {children}
    </label>
  )
}

const inputClassName = 'h-11 w-full rounded-2xl border border-white/10 bg-black/20 px-4 text-sm text-slate-100 outline-none transition focus:border-orange-400'