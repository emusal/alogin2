import { useEffect, useRef, useImperativeHandle, forwardRef } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import type { Server } from '../types'
import '@xterm/xterm/css/xterm.css'
import './Terminal.css'

export interface TerminalHandle {
  sendData: (text: string) => void
  /** The root DOM element of this terminal — move it to attach to a new container */
  getElement: () => HTMLElement | null
  /** Re-fit xterm to current container size after DOM move */
  fit: () => void
}

interface Props {
  server: Server
  autoGW?: boolean
  app?: string
  onStatusChange?: (status: 'connecting' | 'connected' | 'disconnected' | 'error') => void
  compact?: boolean
}

export const Terminal = forwardRef<TerminalHandle, Props>(function Terminal(
  { server, autoGW = false, app, onStatusChange, compact },
  ref
) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const onStatusChangeRef = useRef(onStatusChange)
  onStatusChangeRef.current = onStatusChange

  useImperativeHandle(ref, () => ({
    sendData(text: string) {
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'data', data: text }))
      }
    },
    getElement() {
      return containerRef.current?.parentElement ?? null
    },
    fit() {
      fitAddonRef.current?.fit()
    },
  }))

  useEffect(() => {
    if (!containerRef.current) return

    // Initialize xterm.js
    const term = new XTerm({
      theme: {
        background: '#1a1a2e',
        foreground: '#e0e0e0',
        cursor: '#c792ea',
        selectionBackground: '#2a2a4a',
        black: '#1a1a2e',
        brightBlack: '#444',
        red: '#f07178',
        brightRed: '#f07178',
        green: '#c3e88d',
        brightGreen: '#c3e88d',
        yellow: '#ffcb6b',
        brightYellow: '#ffcb6b',
        blue: '#82aaff',
        brightBlue: '#82aaff',
        magenta: '#c792ea',
        brightMagenta: '#c792ea',
        cyan: '#89ddff',
        brightCyan: '#89ddff',
        white: '#e0e0e0',
        brightWhite: '#ffffff',
      },
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      fontSize: compact ? 11 : 14,
      lineHeight: 1.3,
      cursorBlink: true,
    })

    const fitAddon = new FitAddon()
    fitAddonRef.current = fitAddon
    const webLinksAddon = new WebLinksAddon()
    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)
    term.open(containerRef.current)
    fitAddon.fit()
    termRef.current = term

    // Connect WebSocket
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const params = new URLSearchParams()
    if (autoGW) params.set('auto_gw', 'true')
    if (app) params.set('app', app)
    const qs = params.toString() ? '?' + params.toString() : ''
    const wsURL = `${proto}//${window.location.host}/ws/terminal/${server.id}${qs}`
    const ws = new WebSocket(wsURL)
    wsRef.current = ws

    ws.onopen = () => {
      const gwLabel = autoGW ? ' (via GW)' : ''
      term.write('\x1b[32mConnecting to ' + server.user + '@' + server.host + gwLabel + '...\x1b[0m\r\n')
      onStatusChangeRef.current?.('connected')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'data') {
          term.write(msg.data)
        }
      } catch {
        term.write(event.data)
      }
    }

    ws.onclose = () => {
      term.write('\r\n\x1b[33m[Connection closed]\x1b[0m\r\n')
      onStatusChangeRef.current?.('disconnected')
    }

    ws.onerror = () => {
      term.write('\r\n\x1b[31m[WebSocket error]\x1b[0m\r\n')
      onStatusChangeRef.current?.('error')
    }

    // Terminal input → WebSocket
    term.onData(data => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'data', data }))
      }
    })

    // Window resize → fit + send resize event
    const onResize = () => {
      fitAddon.fit()
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          cols: term.cols,
          rows: term.rows,
        }))
      }
    }
    window.addEventListener('resize', onResize)

    return () => {
      window.removeEventListener('resize', onResize)
      ws.close()
      term.dispose()
    }
  }, [server.id, autoGW])

  return (
    <div className="terminal-container">
      {!compact && (
        <div className="terminal-header">
          <span className="terminal-title">
            {server.user}@{server.host}
            {server.port > 0 ? `:${server.port}` : ''}
          </span>
        </div>
      )}
      <div ref={containerRef} className="terminal-body" />
    </div>
  )
})
