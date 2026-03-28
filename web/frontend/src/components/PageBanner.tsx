import { getPageInfo } from '../i18n'
import './PageBanner.css'

type PageKey = 'compute' | 'gateways' | 'clusters' | 'hosts' | 'tunnels' | 'plugins' | 'app-servers' | 'terminal'

interface Props {
  page: PageKey
}

export function PageBanner({ page }: Props) {
  const info = getPageInfo(page)
  return (
    <div className="page-banner">
      <strong className="page-banner-title">{info.title}</strong>
      <span className="page-banner-desc">{info.lines.join('  ·  ')}</span>
    </div>
  )
}
