import type { ReactNode } from 'react'
import type { TranscriptEntryView, TranscriptHighlightView, TranscriptPartialView } from '../types'

type TimelineProps = {
  entries: TranscriptEntryView[]
  partials: TranscriptPartialView[]
  emptyLabel: string
}

export function Timeline({ entries, partials, emptyLabel }: TimelineProps) {
  if (entries.length === 0 && partials.length === 0) {
    return (
      <section className="rounded-3xl border border-dashed border-white/10 bg-slate-900/50 p-6 text-center text-slate-400 sm:p-10">
        {emptyLabel}
      </section>
    )
  }

  const orderedEntries = [...entries].reverse()

  return (
    <section className="space-y-3 sm:space-y-4">
      {partials.map((partial) => (
        <article key={`partial-${partial.channelId}`} className="rounded-3xl border border-orange-300/20 bg-orange-300/6 p-4 shadow-xl shadow-black/10 sm:p-5">
          <MessageHeader
            icon={partial.icon}
            channelName={partial.channelName}
            color={partial.color}
            timestamp={partial.timestamp}
            badge="Listening"
          />
          <p className="mt-3 break-words text-sm leading-6 text-slate-100 sm:text-base sm:leading-7">{partial.text}</p>
        </article>
      ))}

      {orderedEntries.map((entry) => (
        <article key={entry.id} className="rounded-3xl border border-white/10 bg-slate-900/80 p-4 shadow-xl shadow-black/10 sm:p-5">
          <MessageHeader
            icon={entry.icon}
            channelName={entry.channelName}
            color={entry.color}
            timestamp={entry.timestamp}
            badge="Final"
          />
          <p className="mt-3 break-words text-sm leading-6 text-slate-100 sm:text-base sm:leading-7">{renderHighlightedText(entry.text, entry.highlights ?? [])}</p>
          {(entry.keywords ?? []).length > 0 ? (
            <div className="mt-4 flex flex-wrap gap-2">
              {(entry.keywords ?? []).map((keyword) => (
                <span key={`${entry.id}-${keyword}`} className="rounded-full border border-white/10 bg-black/20 px-2.5 py-1 text-xs uppercase tracking-[0.18em] text-slate-300">
                  {keyword}
                </span>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </section>
  )
}

function renderHighlightedText(text: string, highlights: TranscriptHighlightView[]): ReactNode {
  if (highlights.length === 0) {
    return text
  }

  const characters = Array.from(text)
  const fragments: ReactNode[] = []
  let cursor = 0

  highlights.forEach((highlight, index) => {
    if (highlight.start > cursor) {
      fragments.push(<span key={`text-${index}-${cursor}`}>{characters.slice(cursor, highlight.start).join('')}</span>)
    }
    fragments.push(
      <mark
        key={`mark-${index}-${highlight.start}`}
        className="rounded-lg px-1.5 py-0.5 text-slate-950"
        style={{ backgroundColor: highlight.color }}
      >
        {characters.slice(highlight.start, highlight.end).join('')}
      </mark>,
    )
    cursor = highlight.end
  })

  if (cursor < characters.length) {
    fragments.push(<span key={`tail-${cursor}`}>{characters.slice(cursor).join('')}</span>)
  }

  return fragments
}

type MessageHeaderProps = {
  icon: string
  channelName: string
  color: string
  timestamp: string
  badge: string
}

function MessageHeader({ icon, channelName, color, timestamp, badge }: MessageHeaderProps) {
  return (
    <div className="flex items-start gap-3 sm:gap-4">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-base sm:h-12 sm:w-12 sm:text-xl" style={{ backgroundColor: `${color}22`, color }}>
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-2 sm:gap-3">
            <span className="truncate font-medium" style={{ color }}>{channelName}</span>
            <span className="rounded-full border border-white/10 px-2 py-0.5 text-xs uppercase tracking-[0.2em] text-slate-400">{badge}</span>
          </div>
          <time className="text-xs uppercase tracking-[0.15em] text-slate-500 sm:tracking-[0.2em]">{timestamp}</time>
        </div>
      </div>
    </div>
  )
}