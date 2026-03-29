import { useState, useRef } from 'react'
import type { BroadcastSummary } from '../types'

interface Props {
  selectedCount: number
  totalCount: number
  running: boolean
  summary: BroadcastSummary | null
  onBroadcast: (command: string) => void
  /** Called for every keystroke in realtime mode */
  onSendRaw: (data: string) => void
}

function BroadcastSummaryView({ summary }: { summary: BroadcastSummary }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="cd-broadcast-summary">
      <span className="cd-bs-ok">{summary.ok} OK</span>
      {summary.failed > 0 && (
        <>
          <span className="cd-bs-sep">/</span>
          <span
            className="cd-bs-fail"
            onClick={() => setExpanded(e => !e)}
            title="Click to see details"
          >
            {summary.failed} failed
          </span>
          {expanded && summary.failedDetails.length > 0 && (
            <div className="cd-bs-detail">
              {summary.failedDetails.map((d, i) => (
                <div key={i} className="cd-bs-detail-row">
                  <span className="cd-bs-detail-host">{d.host}</span>
                  <span className="cd-bs-detail-err">{d.error}</span>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}

/** Convert a KeyboardEvent to the terminal byte sequence it should produce */
function keyToData(e: React.KeyboardEvent<HTMLInputElement>): string | null {
  // Ctrl+key sequences
  if (e.ctrlKey && !e.altKey && !e.metaKey) {
    const k = e.key.toLowerCase()
    if (k.length === 1 && k >= 'a' && k <= 'z') {
      return String.fromCharCode(k.charCodeAt(0) - 96) // Ctrl+A=\x01 … Ctrl+Z=\x1a
    }
    if (e.key === 'Enter') return '\r'
    if (e.key === '[') return '\x1b'   // Ctrl+[  = ESC
    if (e.key === '\\') return '\x1c'
    if (e.key === ']') return '\x1d'
    if (e.key === '_') return '\x1f'
  }
  // Special keys → ANSI/VT sequences
  const map: Record<string, string> = {
    Enter:     '\r',
    Backspace: '\x7f',
    Delete:    '\x1b[3~',
    Tab:       '\t',
    Escape:    '\x1b',
    ArrowUp:   '\x1b[A',
    ArrowDown: '\x1b[B',
    ArrowRight:'\x1b[C',
    ArrowLeft: '\x1b[D',
    Home:      '\x1b[H',
    End:       '\x1b[F',
    PageUp:    '\x1b[5~',
    PageDown:  '\x1b[6~',
    F1: '\x1bOP', F2: '\x1bOQ', F3: '\x1bOR', F4: '\x1bOS',
    F5: '\x1b[15~', F6: '\x1b[17~', F7: '\x1b[18~', F8: '\x1b[19~',
    F9: '\x1b[20~', F10: '\x1b[21~', F11: '\x1b[23~', F12: '\x1b[24~',
  }
  if (map[e.key]) return map[e.key]
  // Regular printable character
  if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) return e.key
  return null
}

export function ClusterCommandBar({ selectedCount, totalCount, running, summary, onBroadcast, onSendRaw }: Props) {
  const [input, setInput] = useState('')
  const [realtimeMode, setRealtimeMode] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const submit = () => {
    const cmd = input.trim()
    if (!cmd || running) return
    onBroadcast(cmd)
    setInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!realtimeMode) {
      if (e.key === 'Enter') submit()
      return
    }

    // Realtime mode: intercept every key and send raw bytes
    const data = keyToData(e)
    if (data !== null) {
      e.preventDefault()
      onSendRaw(data)
      // Keep the input visually clear in realtime mode
      setInput('')
    }
  }

  return (
    <div className="cd-command-bar">
      <span className="cd-target-info">{selectedCount}/{totalCount} nodes</span>
      <input
        ref={inputRef}
        className="cd-command-input"
        placeholder={realtimeMode ? 'Realtime — keystrokes sent immediately…' : 'Command to broadcast…'}
        value={realtimeMode ? '' : input}
        disabled={running}
        readOnly={realtimeMode}
        onChange={e => { if (!realtimeMode) setInput(e.target.value) }}
        onKeyDown={handleKeyDown}
      />
      {!realtimeMode && (
        <button
          className="cd-send-btn"
          disabled={running || !input.trim() || selectedCount === 0}
          onClick={submit}
        >
          {running ? 'Running…' : 'Send'}
        </button>
      )}
      <label className="cd-rt-toggle" title="Send every keystroke in real time (like tmux synchronize-panes)">
        <input
          type="checkbox"
          checked={realtimeMode}
          onChange={e => {
            setRealtimeMode(e.target.checked)
            setInput('')
            // Keep focus on the input so typing goes straight in
            requestAnimationFrame(() => inputRef.current?.focus())
          }}
        />
        <span>realtime</span>
      </label>
      {summary && !realtimeMode && <BroadcastSummaryView summary={summary} />}
    </div>
  )
}
