# alogin

**macOS · Linux용 SSH 접속 관리 도구** — 대화형 TUI 호스트 선택기, 암호화 자격증명 저장소, 멀티홉 게이트웨이 라우팅, 클러스터 세션, 브라우저 기반 웹 터미널.

> 2000년대 초 Bash + Expect 스크립트로 만들어진 [alogin v1](https://github.com/emusal/alogin2)을 Go로 완전히 재작성한 버전입니다.

**언어** : 한국어 | [English](README.en.md)

---

## 기능

- **대화형 TUI** — 화살표 키 + 퍼지 검색으로 호스트 선택 (호스트명 전체 입력 불필요)
- **순수 Go SSH 클라이언트** — `expect` 없음, 프롬프트 패턴 파싱 없음
- **멀티홉 게이트웨이** — 배스천 호스트를 통한 투명한 ProxyJump 체이닝
- **암호화 자격증명 저장소** — macOS Keychain, Linux Secret Service, 또는 `age` 암호화 파일
- **클러스터 세션** — tmux(크로스플랫폼) 또는 iTerm2 / Terminal.app(macOS)으로 다중 호스트 동시 접속
- **Web UI** — 브라우저 기반 SSH 터미널 + 서버 관리 대시보드 (`alogin web`)
- **v1 호환** — 얇은 셸 심(shim)으로 `t`, `r`, `s`, `f`, `m`, `ct`, `cr` 명령어 그대로 사용
- **마이그레이션 도구** — `server_list`, `gateway_list`, `clusters` 등 v1 파일을 한 번에 가져오기

---

## 설치

### Homebrew (권장)

```bash
brew tap emusal/alogin --custom-remote git@github.com:emusal/alogin2.git
brew install alogin
```

### 바이너리 직접 다운로드

[Releases](https://github.com/emusal/alogin2/releases) 페이지에서 최신 버전을 받습니다.

```bash
# macOS (Apple Silicon)
curl -L https://github.com/emusal/alogin2/releases/latest/download/alogin-darwin-arm64 -o /usr/local/bin/alogin
chmod +x /usr/local/bin/alogin

# macOS (Intel)
curl -L https://github.com/emusal/alogin2/releases/latest/download/alogin-darwin-amd64 -o /usr/local/bin/alogin
chmod +x /usr/local/bin/alogin

# Linux (amd64)
curl -L https://github.com/emusal/alogin2/releases/latest/download/alogin-linux-amd64 -o /usr/local/bin/alogin
chmod +x /usr/local/bin/alogin
```

### 소스 빌드

Go 1.23 이상 필요.

```bash
git clone https://github.com/emusal/alogin2.git
cd alogin2
go build -o alogin ./cmd/alogin
sudo mv alogin /usr/local/bin/
```

---

## 빠른 시작

### 1. 설치 확인

```bash
alogin version
```

데이터베이스는 첫 실행 시 `~/.local/share/alogin/alogin.db`에 자동 생성됩니다.

### 2. v1에서 마이그레이션

기존 v1 설치가 있다면:

```bash
alogin migrate --from /path/to/old/alogin
```

`server_list`, `gateway_list`, `alias_hosts`, `clusters`, `term_themes`를 SQLite로 가져오고, 비밀번호는 시스템 키체인으로 이동합니다.

### 3. 서버 추가

```bash
alogin server add
```

프로토콜, 호스트, 사용자, 포트, 게이트웨이, 로케일을 입력합니다.
비밀번호는 시스템 키체인(macOS Keychain / Linux Secret Service)에 저장되며 데이터베이스에는 저장되지 않습니다.

### 4. 접속

```bash
alogin connect              # 대화형 TUI 선택기 실행
alogin connect web-01       # 호스트명으로 직접 접속
alogin connect admin@web-01 # 사용자 지정
```

### 5. v1 사용자 — 셸 호환 설정

`~/.zshrc` 또는 `~/.bashrc`에 추가:

```bash
source <(alogin shell-init)
```

기존 v1 명령어를 그대로 사용할 수 있습니다:

```bash
t web-01          # SSH 접속 (직접)
r admin@bastion   # SSH 접속 (게이트웨이 자동 감지)
s web-01          # SFTP
f ftp-server      # FTP
m web-01          # SSHFS 마운트
ct prod-cluster   # 클러스터 접속 (타일 창)
cr prod-cluster   # 게이트웨이 경유 클러스터 접속
```

---

## 명령어

### 접속

```
alogin connect [user@]host... [flags]

  --auto-gw              게이트웨이 자동 감지 (v1 'r' 동작)
  --dry-run              실제 접속 없이 경로만 출력
  -c, --cmd string       로그인 후 실행할 명령어
  -L, --local-forward    로컬 포트 포워딩  (local:host:port)
  -R, --remote-forward   원격 포트 포워딩 (remote:host:port)
```

```bash
alogin connect                     # TUI 선택기
alogin connect web-01              # 직접 접속
alogin connect gw-01 web-01        # 명시적 2홉
alogin connect gw-01 gw-02 web-01  # 명시적 3홉
alogin connect web-01 --auto-gw    # 등록된 게이트웨이 경유
```

### 파일 전송

```bash
alogin sftp [user@]host [-p 로컬파일] [-g 원격파일]
alogin ftp  [user@]host
alogin mount [user@]host [원격경로]   # SSHFS 마운트
```

### 클러스터

```bash
alogin cluster [name] [flags]

  --gateway          게이트웨이 경유 (v1 'cr')
  --mode string      세션 모드: tmux (기본값), iterm, terminal
  -x, --tile-x int   타일 열 수 (0=자동)
```

### 서버 관리

```bash
alogin server list
alogin server add [--proto ssh] [--host host] [--user user] [--gateway name] [--locale loc]
alogin server show [user@]host
alogin server delete [user@]host
alogin server passwd [user@]host
alogin server getpwd [user@]host    # 저장된 비밀번호 확인
```

### 게이트웨이 관리

```bash
alogin gateway list
alogin gateway add
alogin gateway show name
alogin gateway delete name
```

### 별칭 관리

```bash
alogin alias list
alogin alias add
alogin alias show name
alogin alias delete name
```

### 마이그레이션

```bash
alogin migrate --from /path/to/alogin_root [--dry-run]
```

### Web UI

```bash
alogin web [--port 8484] [--no-browser]
```

`http://localhost:8484`을 자동으로 엽니다.

### 셸 자동완성

```bash
alogin completion install              # zsh (기본값)
alogin completion install --shell bash # bash
```

---

## 설정

기본 경로 (XDG 규격):

| 경로 | 설명 |
|------|------|
| `~/.local/share/alogin/alogin.db` | SQLite 데이터베이스 |
| `~/.local/share/alogin/vault.age` | age 암호화 저장소 (폴백) |
| `~/.config/alogin/config.toml` | 설정 파일 |
| `~/.local/share/alogin/alogin.log` | 로그 파일 |

환경 변수로 오버라이드:

```bash
ALOGIN_DB            # SQLite 데이터베이스 경로
ALOGIN_CONFIG        # config.toml 경로
ALOGIN_LOG_LEVEL     # 0=오류, 1=정보, 2=디버그 (기본값: 0)
ALOGIN_LANG          # 기본 로케일 (기본값: 시스템)
ALOGIN_SSHOPT        # 추가 SSH 옵션
ALOGIN_SSHCMD        # 커스텀 SSH 바이너리 경로
ALOGIN_KEYCHAIN_USE  # 설정 시 Keychain 강제 사용
ALOGIN_ROOT          # 레거시: DB + 설정 상위 디렉토리
```

`config.toml` 예시:

```toml
[ssh]
default_options = "-o StrictHostKeyChecking=no -o ServerAliveInterval=30"
connect_timeout = 10

[vault]
backend = "keychain"   # keychain | libsecret | age | plaintext

[web]
port = 8484
```

---

## 보안

### 자격증명 저장

비밀번호는 **데이터베이스에 절대 저장되지 않습니다**. `password` 컬럼은 센티널값 `_HIDDEN_`을 보유합니다. 실제 자격증명은 다음에 저장됩니다:

1. **macOS Keychain** (macOS 기본값) — `Security.framework` 사용
2. **Linux Secret Service** (Linux 기본값) — D-Bus를 통한 GNOME Keyring / KWallet
3. **age 암호화 파일** — 크로스플랫폼 폴백, 마스터 패스프레이즈로 잠금
4. **플레인텍스트** — 마이그레이션 중 레거시 `server_list` 읽기 전용

### SSH 키 인증

alogin은 `~/.ssh/config` 및 SSH 에이전트를 자동으로 사용합니다. SSH 키를 배포한 호스트는 비밀번호 입력 없이 접속됩니다.

---

## 멀티홉 게이트웨이

게이트웨이 경로 정의:

```bash
alogin gateway add
# name: prod-route
# hops: bastion-01 → internal-gw → (목적지)
```

서버에 할당:

```bash
alogin server add
# ...
# gateway: prod-route
```

alogin은 Go 네이티브 SSH 라이브러리로 각 홉을 순서대로 다이얼합니다:

```
로컬 → bastion-01:22 → internal-gw:22 → web-01:22
```

중간 홉에서 `AllowTcpForwarding`이 비활성화된 경우, alogin은 자동으로 **셸 체인 방식**으로 폴백합니다 (각 홉의 셸 안에서 `ssh -tt` 실행 — v1의 `conn.exp` 동작과 동일).

---

## 클러스터 세션

클러스터의 모든 멤버에 동시 접속:

```bash
alogin cluster prod-web --mode tmux      # tmux 분할 창 (macOS + Linux)
alogin cluster prod-web --mode iterm     # iTerm2 분할 창 (macOS)
alogin cluster prod-web --mode terminal  # Terminal.app 타일 (macOS)
```

클러스터 관리:

```bash
alogin cluster add prod-web web-01 web-02 web-03
alogin cluster list
alogin cluster show prod-web
alogin cluster delete prod-web
```

---

## Web UI

```bash
alogin web
```

`http://localhost:8484`에서 다음을 제공합니다:

- **서버 목록** — 검색, 탐색, 접속
- **웹 터미널** — 브라우저의 xterm.js 기반 완전한 SSH 세션
- **클러스터 관리** — 클러스터 생성 및 편집

웹 서버는 기본적으로 로컬 전용입니다. 인증 없이 네트워크에 노출하지 마세요.

---

## v1 마이그레이션

alogin v2는 v1 데이터 파일과 완전히 호환됩니다. 한 번만 실행하면 됩니다:

```bash
alogin migrate --from $ALOGIN_ROOT
```

마이그레이션 내용:
- `server_list`, `gateway_list`, `alias_hosts`, `clusters`, `term_themes` 파싱
- `<space>` / `<tab>` 리터럴을 실제 문자로 변환
- 모든 데이터를 SQLite로 가져오기
- 비밀번호를 시스템 키체인으로 이동 (데이터베이스에서 제거)
- 원본 파일은 그대로 보존

마이그레이션 후 호환 셸 심을 로드하면 `t`, `r`, `s` 등을 계속 사용할 수 있습니다:

```bash
source <(alogin shell-init)
```

---

## 개발

### 사전 요구사항

- Go 1.23+
- Node.js 20+ (Web UI 프론트엔드 빌드 시에만 필요)

### 빌드

```bash
# CLI만
go build ./cmd/alogin

# Web UI 프론트엔드 (임베드 전 필요)
cd web/frontend && npm install && npm run build

# Web UI 포함 전체 빌드
go build -tags web ./cmd/alogin
```

### 테스트

```bash
go test ./...
go vet ./...
```

### 크로스 컴파일

```bash
make dist       # darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
make dist-web   # Web UI 포함 macOS 빌드
```

### 프로젝트 구조

```
cmd/alogin/          진입점
internal/
  cli/               Cobra 커맨드 정의
  config/            설정 로딩 (XDG + 환경 변수)
  model/             데이터 타입 (Server, Gateway, Cluster ...)
  db/                SQLite 저장소 (스키마: internal/db/schema.sql)
  vault/             시크릿 백엔드 (Keychain / libsecret / age / plaintext)
  ssh/               네이티브 SSH 클라이언트 (멀티홉, PTY, 터널, SFTP, SSHFS)
  migrate/           v1 TSV → SQLite 마이그레이션 파서
  cluster/           클러스터 세션 오케스트레이션 (tmux / iTerm2 / Terminal.app)
  tui/               Bubbletea 대화형 호스트 선택기
  web/               HTTP 서버 + WebSocket 터미널 + REST API
web/frontend/        React + xterm.js (Vite)
completions/         셸 심 + zsh/bash 자동완성 스크립트
docs/                상세 문서
```

---

## 라이선스

MIT
