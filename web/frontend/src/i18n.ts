/** Detect whether the current browser locale is Korean. */
export function isKorean(): boolean {
  const lang = navigator.language || ''
  return lang.toLowerCase().startsWith('ko')
}

export interface PageInfo {
  title: string
  lines: string[]
}

type PageKey = 'compute' | 'gateways' | 'clusters' | 'hosts' | 'tunnels' | 'plugins' | 'app-servers' | 'terminal'

const translations: Record<PageKey, { ko: PageInfo; en: PageInfo }> = {
  compute: {
    ko: {
      title: '서버 목록',
      lines: [
        '등록된 SSH 서버를 검색·접속합니다. 행을 더블클릭하거나 Connect 버튼으로 세션을 엽니다.',
        '게이트웨이가 설정된 서버는 GW 버튼으로 배스천 호스트를 경유해 접속할 수 있습니다.',
        '로컬 Hosts 매핑을 등록하면 IP 대신 이름으로도 접속할 수 있습니다.',
      ],
    },
    en: {
      title: 'Compute',
      lines: [
        'Browse and connect to registered SSH servers. Double-click a row or click Connect to open a session.',
        'Servers with a gateway show a GW button for bastion-routed connections.',
        'Register Local Host mappings to connect by name instead of IP address.',
      ],
    },
  },
  gateways: {
    ko: {
      title: '게이트웨이',
      lines: [
        '배스천 호스트를 통한 다중 홉 SSH 경로를 관리합니다.',
        '게이트웨이를 서버에 연결하면 서버 목록의 GW 버튼으로 자동 경유 접속합니다.',
        '여러 서버를 순서대로 연결하는 ProxyJump 체인을 지원합니다.',
      ],
    },
    en: {
      title: 'Gateways',
      lines: [
        'Manage multi-hop SSH routes via bastion hosts.',
        'Assigning a gateway to a server enables the GW button in the server list for automatic routing.',
        'Supports ordered ProxyJump chains across multiple gateway servers.',
      ],
    },
  },
  clusters: {
    ko: {
      title: '클러스터',
      lines: [
        '여러 서버에 동시 SSH 접속하는 클러스터를 관리합니다.',
        '클러스터 실행 시 tmux(크로스플랫폼) 또는 iTerm2 / Terminal.app(macOS)으로 분할 창을 엽니다.',
        '클러스터 멤버별로 로그인 사용자를 개별 지정할 수 있습니다.',
      ],
    },
    en: {
      title: 'Clusters',
      lines: [
        'Manage server clusters for simultaneous multi-host SSH sessions.',
        'Launching a cluster opens split panes in tmux (cross-platform) or iTerm2 / Terminal.app (macOS).',
        'Each cluster member can have its own login user override.',
      ],
    },
  },
  hosts: {
    ko: {
      title: '로컬 Hosts',
      lines: [
        'DNS 대신 사용할 호스트 이름 → IP 주소 매핑을 관리합니다.',
        '서버 접속 시 이 목록에서 먼저 IP를 조회하므로, 내부망 이름을 바로 사용할 수 있습니다.',
        '/etc/hosts와 유사하지만 alogin 전용으로 동작하며 OS 재부팅 없이 즉시 적용됩니다.',
      ],
    },
    en: {
      title: 'Local Hosts',
      lines: [
        'Manage hostname → IP mappings used instead of DNS for SSH connections.',
        'alogin looks up this list first when connecting by server name, enabling internal hostnames.',
        'Works like /etc/hosts but scoped to alogin — changes apply instantly without an OS restart.',
      ],
    },
  },
  tunnels: {
    ko: {
      title: '터널',
      lines: [
        'SSH 포트 포워드 터널을 저장하고 tmux 세션으로 백그라운드에서 유지합니다.',
        'Start 버튼으로 터널을 시작하면 detached tmux 세션이 생성되어 연결이 유지됩니다.',
        '-L (로컬 포워드)와 -R (리모트 포워드) 두 방향을 모두 지원합니다.',
      ],
    },
    en: {
      title: 'Tunnels',
      lines: [
        'Save SSH port-forward configurations and maintain them as detached tmux sessions.',
        'Click Start to launch a tunnel — it runs in a named tmux session until you Stop it.',
        'Supports both -L (local forward) and -R (remote forward) directions.',
      ],
    },
  },
  plugins: {
    ko: {
      title: '앱 플러그인',
      lines: [
        '설치된 애플리케이션 플러그인 목록입니다. 플러그인은 ~/.config/alogin/plugins/*.yaml 에 위치합니다.',
        'SSH 접속 시 --app <name> 플래그로 플러그인을 실행하면 자격증명이 자동 주입됩니다.',
        '--cmd 옵션으로 비대화형 명령을 실행할 수 있습니다.',
      ],
    },
    en: {
      title: 'App Plugins',
      lines: [
        'Installed application plugins. Place *.yaml files in ~/.config/alogin/plugins/ to add plugins.',
        'Use --app <name> when connecting via SSH to auto-inject credentials for the application.',
        'Pass --cmd to run a non-interactive query instead of opening an interactive session.',
      ],
    },
  },
  'app-servers': {
    ko: {
      title: '앱 서버',
      lines: [
        '컴퓨트 서버와 애플리케이션 플러그인을 묶은 단축 바인딩을 관리합니다.',
        'Connect 버튼으로 바인딩된 서버에 자동으로 플러그인을 실행하며 접속합니다.',
        '등록해두면 --app 플래그 없이 이름 하나만으로 앱에 접속할 수 있습니다.',
      ],
    },
    en: {
      title: 'App Servers',
      lines: [
        'Manage named bindings that pair a compute server with an application plugin.',
        'Click Connect to open a terminal session with the plugin launched automatically.',
        'Once registered, connect by name without specifying --app each time.',
      ],
    },
  },
  terminal: {
    ko: {
      title: 'SSH 터미널',
      lines: [
        '브라우저에서 SSH 세션을 실행 중입니다. 상단 탭으로 여러 세션을 동시에 관리할 수 있습니다.',
        '[GW] 표시는 게이트웨이(배스천 호스트)를 경유한 접속입니다.',
        '탭의 × 버튼을 클릭하면 세션을 종료하고 탭을 닫습니다.',
      ],
    },
    en: {
      title: 'SSH Terminal',
      lines: [
        'An SSH session is running in your browser. Use the tabs above to manage multiple sessions at once.',
        '[GW] indicates a connection routed through a gateway (bastion host).',
        'Click the × on a tab to close the session and remove the tab.',
      ],
    },
  },
}

export function getPageInfo(key: PageKey): PageInfo {
  const t = translations[key]
  return isKorean() ? t.ko : t.en
}
