import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import type { ChannelAddInput, ChannelUpdateInput, ChannelView, InputDeviceView, KeywordRuleInput, KeywordRuleView, OscSettingsInput, OscSettingsView } from '../types'

type ChannelSettingsPanelProps = {
  channels: ChannelView[]
  inputDevices: InputDeviceView[]
  keywords: KeywordRuleView[]
  osc: OscSettingsView
  saving: boolean
  onSaveChannel: (input: ChannelUpdateInput) => Promise<void>
  onAddChannel: (input: ChannelAddInput) => Promise<void>
  onRemoveChannel: (channelId: string) => Promise<void>
  onSaveKeywords: (rules: KeywordRuleInput[]) => Promise<void>
  onSaveOsc: (input: OscSettingsInput) => Promise<void>
}

export function ChannelSettingsPanel({ channels, inputDevices, keywords, osc, saving, onSaveChannel, onAddChannel, onRemoveChannel, onSaveKeywords, onSaveOsc }: ChannelSettingsPanelProps) {
  const [section, setSection] = useState<'channels' | 'keywords' | 'osc'>('channels')
  const [activeChannelId, setActiveChannelId] = useState<string>(channels[0]?.id ?? '')
  const [draft, setDraft] = useState<ChannelUpdateInput | null>(null)
  const [newChannelName, setNewChannelName] = useState('')
  const [newChannelLanguage, setNewChannelLanguage] = useState('en')
  const [keywordDrafts, setKeywordDrafts] = useState<KeywordRuleInput[]>(keywords)
  const [oscDraft, setOscDraft] = useState<OscSettingsInput>(osc)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (!channels.some((channel) => channel.id === activeChannelId)) {
      setActiveChannelId(channels[0]?.id ?? '')
    }
  }, [activeChannelId, channels])

  const activeChannel = channels.find((item) => item.id === activeChannelId)
  const normalizedChannelNames = channels.map((channel) => channel.name.trim().toLowerCase())
  const nextChannelID = toChannelID(newChannelName)
  const addChannelBlockedReason = getAddChannelBlockedReason({
    channels,
    normalizedChannelNames,
    nextChannelID,
    newChannelName,
  })
  const channelSaveBlockedReason = draft
    ? getChannelSaveBlockedReason({
        channels,
        draft,
      })
    : ''
  const keywordsSaveBlockedReason = getKeywordsSaveBlockedReason(keywordDrafts)
  const oscSaveBlockedReason = getOscSaveBlockedReason(oscDraft)

  useEffect(() => {
    setKeywordDrafts(keywords)
  }, [keywords])

  useEffect(() => {
    setOscDraft(osc)
  }, [osc])

  useEffect(() => {
    if (!activeChannel) {
      setDraft(null)
      return
    }

    // Preserve unsaved edits while live runtime updates stream in.
    setDraft((current) => (current && current.id === activeChannel.id ? current : toDraft(activeChannel)))
  }, [activeChannel, activeChannelId])

  useEffect(() => {
    setErrorMessage('')
  }, [section])

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
    <section className="rounded-3xl border border-white/10 bg-slate-900/80 p-4 shadow-xl shadow-black/10 sm:p-6">
      <h3 className="text-lg font-semibold text-slate-100">Settings</h3>
      <div className="mt-4 flex flex-wrap gap-2">
        <TabButton label="Channels" active={section === 'channels'} onClick={() => setSection('channels')} />
        <TabButton label="Keywords" active={section === 'keywords'} onClick={() => setSection('keywords')} />
        <TabButton label="OSC" active={section === 'osc'} onClick={() => setSection('osc')} />
      </div>

      {errorMessage ? <p className="mt-4 rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-100">{errorMessage}</p> : null}

      {section === 'channels' ? (
        <>
          <form
            className="mt-5 grid gap-4 md:grid-cols-2"
            onSubmit={async (event) => {
              event.preventDefault()
              if (channelSaveBlockedReason) {
                setErrorMessage(channelSaveBlockedReason)
                return
              }
              setErrorMessage('')
              await onSaveChannel(draft)
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

        <Field label={`Local Gain (${formatGainDb(draft.gainDb)} dB)`}>
          <input
            className={inputClassName}
            type="range"
            min={-24}
            max={36}
            step={1}
            value={draft.gainDb}
            onChange={(event) => setDraft({ ...draft, gainDb: Number(event.target.value) })}
          />
        </Field>

        <Field label="State">
          <label className="flex h-11 items-center gap-3 rounded-2xl border border-white/10 bg-black/20 px-4 text-sm text-slate-200">
            <input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} />
            Enabled
          </label>
        </Field>

        <div className="md:col-span-2 flex justify-end rounded-2xl border border-white/10 bg-black/20 px-4 py-4">
          <button className="w-full rounded-2xl bg-orange-500 px-5 py-3 text-sm font-medium text-slate-950 transition hover:bg-orange-400 disabled:cursor-not-allowed disabled:bg-slate-600 md:w-auto" type="submit" disabled={saving || Boolean(channelSaveBlockedReason)} title={channelSaveBlockedReason || ''}>
            {saving ? 'Saving...' : 'Save Channel'}
          </button>
        </div>
      </form>

      <div className="mt-4 rounded-2xl border border-white/10 bg-black/20 px-4 py-4">
        <div className="grid gap-3 lg:grid-cols-[1fr_180px_120px_auto]">
          <input
            className={inputClassName}
            placeholder="New channel name"
            value={newChannelName}
            onChange={(event) => setNewChannelName(event.target.value)}
          />
          <select className={inputClassName} value={newChannelLanguage} onChange={(event) => setNewChannelLanguage(event.target.value)}>
            <option value="en">English</option>
            <option value="no">Norwegian</option>
          </select>
          <button
            type="button"
            className="rounded-2xl border border-white/20 px-4 py-2 text-sm text-slate-200 disabled:opacity-50"
            disabled={saving || Boolean(addChannelBlockedReason)}
            title={addChannelBlockedReason || ''}
            onClick={async () => {
              if (addChannelBlockedReason) {
                setErrorMessage(addChannelBlockedReason)
                return
              }
              setErrorMessage('')
              const id = nextChannelID
              await onAddChannel({ id, name: newChannelName.trim(), color: '#94A3B8', icon: '🎧', inputDevice: '', language: newChannelLanguage, gainDb: 0, enabled: true })
              setNewChannelName('')
            }}
          >
            Add
          </button>
          <button
            type="button"
            className="rounded-2xl border border-red-400/40 px-4 py-2 text-sm text-red-200 disabled:opacity-50"
            disabled={saving || channels.length <= 1 || !activeChannelId}
            onClick={async () => {
              if (channels.length <= 1) {
                setErrorMessage('At least one channel must remain configured.')
                return
              }
              setErrorMessage('')
              await onRemoveChannel(activeChannelId)
              setActiveChannelId(channels.find((channel) => channel.id !== activeChannelId)?.id ?? '')
            }}
          >
            Remove Selected
          </button>
        </div>
        <p className="mt-2 text-xs text-slate-500">Channels: {channels.length}/8</p>
      </div>
        </>
      ) : null}

      {section === 'keywords' ? (
        <div className="mt-5 space-y-3">
          {keywordDrafts.map((rule, index) => (
            <div key={`${rule.phrase}-${index}`} className="grid gap-3 rounded-2xl border border-white/10 bg-black/20 p-3 lg:grid-cols-6">
              <input className={inputClassName} placeholder="Phrase" value={rule.phrase} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, phrase: event.target.value } : item)))} />
              <input className={inputClassName} placeholder="#RRGGBB" value={rule.highlightColor} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, highlightColor: event.target.value } : item)))} />
              <input className={inputClassName} placeholder="OSC Address" value={rule.oscAddress} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, oscAddress: event.target.value } : item)))} />
              <input className={inputClassName} placeholder="OSC Args (comma)" value={rule.oscArguments.join(',')} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, oscArguments: event.target.value.split(',').map((value) => value.trim()).filter(Boolean) } : item)))} />
              <label className="flex h-11 items-center gap-2 rounded-2xl border border-white/10 bg-black/20 px-3 text-sm text-slate-200"><input type="checkbox" checked={rule.wholeWord} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, wholeWord: event.target.checked } : item)))} />Whole word</label>
              <label className="flex h-11 items-center gap-2 rounded-2xl border border-white/10 bg-black/20 px-3 text-sm text-slate-200"><input type="checkbox" checked={rule.triggerEnabled} onChange={(event) => setKeywordDrafts(keywordDrafts.map((item, i) => (i === index ? { ...item, triggerEnabled: event.target.checked } : item)))} />OSC</label>
            </div>
          ))}
          <div className="flex flex-col gap-2 sm:flex-row">
            <button type="button" className="rounded-2xl border border-white/20 px-4 py-2 text-sm text-slate-200" onClick={() => setKeywordDrafts([...keywordDrafts, { phrase: '', highlightColor: '#22c55e', wholeWord: false, triggerEnabled: false, oscAddress: '', oscArguments: [] }])}>Add Rule</button>
            <button
              type="button"
              className="rounded-2xl bg-orange-500 px-4 py-2 text-sm font-medium text-slate-950 disabled:opacity-50"
              disabled={saving || Boolean(keywordsSaveBlockedReason)}
              title={keywordsSaveBlockedReason || ''}
              onClick={async () => {
                if (keywordsSaveBlockedReason) {
                  setErrorMessage(keywordsSaveBlockedReason)
                  return
                }
                setErrorMessage('')
                await onSaveKeywords(keywordDrafts)
              }}
            >
              Save Keywords
            </button>
          </div>
        </div>
      ) : null}

      {section === 'osc' ? (
        <form className="mt-5 grid gap-4 md:grid-cols-3" onSubmit={async (event) => {
          event.preventDefault()
          if (oscSaveBlockedReason) {
            setErrorMessage(oscSaveBlockedReason)
            return
          }
          setErrorMessage('')
          await onSaveOsc(oscDraft)
        }}>
          <Field label="Enabled">
            <label className="flex h-11 items-center gap-3 rounded-2xl border border-white/10 bg-black/20 px-4 text-sm text-slate-200">
              <input type="checkbox" checked={oscDraft.enabled} onChange={(event) => setOscDraft({ ...oscDraft, enabled: event.target.checked })} />
              Enable OSC
            </label>
          </Field>
          <Field label="Destination">
            <input className={inputClassName} value={oscDraft.destination} onChange={(event) => setOscDraft({ ...oscDraft, destination: event.target.value })} />
          </Field>
          <Field label="Port">
            <input className={inputClassName} type="number" min={1} max={65535} value={oscDraft.port} onChange={(event) => setOscDraft({ ...oscDraft, port: Number(event.target.value) })} />
          </Field>
          <div className="md:col-span-3 flex justify-end">
            <button className="w-full rounded-2xl bg-orange-500 px-5 py-3 text-sm font-medium text-slate-950 transition hover:bg-orange-400 disabled:cursor-not-allowed disabled:bg-slate-600 md:w-auto" type="submit" disabled={saving || Boolean(oscSaveBlockedReason)} title={oscSaveBlockedReason || ''}>
              {saving ? 'Saving...' : 'Save OSC'}
            </button>
          </div>
        </form>
      ) : null}
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
    gainDb: channel.gainDb,
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

function TabButton({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className={["rounded-2xl border px-4 py-2 text-sm", active ? 'border-orange-400 bg-orange-400/15 text-orange-100' : 'border-white/10 bg-black/20 text-slate-300'].join(' ')}>{label}</button>
  )
}

function toChannelID(value: string): string {
  const normalized = value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return normalized || `channel-${Date.now()}`
}

function getAddChannelBlockedReason(input: {
  channels: ChannelView[]
  normalizedChannelNames: string[]
  nextChannelID: string
  newChannelName: string
}): string {
  const trimmedName = input.newChannelName.trim()
  if (trimmedName === '') {
    return 'A channel name is required.'
  }
  if (input.channels.length >= 8) {
    return 'Maximum channel count reached (8).'
  }
  if (input.channels.some((channel) => channel.id === input.nextChannelID)) {
    return 'A channel with this generated ID already exists. Use a different name.'
  }
  if (input.normalizedChannelNames.includes(trimmedName.toLowerCase())) {
    return 'A channel with this name already exists.'
  }
  return ''
}

function getChannelSaveBlockedReason(input: {
  channels: ChannelView[]
  draft: ChannelUpdateInput
}): string {
  const trimmedName = input.draft.name.trim()
  if (trimmedName === '') {
    return 'Display name cannot be empty.'
  }
  const duplicate = input.channels.some((channel) => channel.id !== input.draft.id && channel.name.trim().toLowerCase() === trimmedName.toLowerCase())
  if (duplicate) {
    return 'Another channel already uses that display name.'
  }
  if (input.draft.enabled && input.draft.inputDevice.trim() === '') {
    return 'Enabled channels must have an input channel assigned.'
  }
  if (!Number.isFinite(input.draft.gainDb) || input.draft.gainDb < -24 || input.draft.gainDb > 36) {
    return 'Gain must be between -24 dB and +36 dB.'
  }
  return ''
}

function formatGainDb(value: number): string {
  const rounded = Math.round(value)
  return rounded > 0 ? `+${rounded}` : `${rounded}`
}

function getKeywordsSaveBlockedReason(rules: KeywordRuleInput[]): string {
  const seen = new Set<string>()
  for (let index = 0; index < rules.length; index += 1) {
    const rule = rules[index]
    const phrase = rule.phrase.trim()
    if (phrase === '') {
      return `Keyword rule ${index + 1} has an empty phrase.`
    }
    const normalized = phrase.toLowerCase()
    if (seen.has(normalized)) {
      return `Duplicate keyword phrase: ${phrase}`
    }
    seen.add(normalized)

    if (!/^#[0-9a-fA-F]{6}$/.test(rule.highlightColor.trim())) {
      return `Keyword rule ${index + 1} must use a valid #RRGGBB color.`
    }
    if (rule.triggerEnabled && rule.oscAddress.trim() !== '' && !rule.oscAddress.trim().startsWith('/')) {
      return `Keyword rule ${index + 1} OSC address must start with '/'.`
    }
  }
  return ''
}

function getOscSaveBlockedReason(input: OscSettingsInput): string {
  const port = Number(input.port)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return 'OSC port must be an integer between 1 and 65535.'
  }
  if (input.enabled && input.destination.trim() === '') {
    return 'OSC destination is required when OSC is enabled.'
  }
  return ''
}