<div align="center">
  <img src="docs/screenshots/alogin2-banner.png" alt="alogin 2">
  <a href="https://github.com/emusal/alogin2/releases"><img src="https://img.shields.io/github/v/release/emusal/alogin2" alt="Version"></a>
  <a href="https://github.com/emusal/alogin2/blob/main/LICENSE"><img src="https://img.shields.io/github/license/emusal/alogin2" alt="License"></a>
</div>

---

**alogin 2**는 AI 에이전트, LLM, 시스템 관리자가 인프라에 안전하게 접근할 수 있도록 설계된 보안 게이트웨이입니다.

2000년대 초 Bash + Expect 스크립트로 만들어진 [alogin v1](https://github.com/emusal/alogin)을 Go로 완전히 재작성한 버전입니다. AI 에이전트가 서버에 접근할 수 있는 안전한 통로 역할을 하면서, 풍부한 대화형 TUI, 암호화 자격증명 저장소, 멀티홉 게이트웨이 라우팅, 클러스터 세션 기능도 제공합니다.

**언어** : 한국어 | [English](README.md)

<img src="docs/screenshots/tui-picker.gif" width="640">

## 주요 기능

alogin 2는 사람과 AI 에이전트 사이의 명확한 역할 분리를 중심으로 설계되었습니다.

### 🧑‍💻 사람 운영자를 위한 기능
- **대화형 TUI** — 화살표 키 + 퍼지 검색으로 호스트 선택 (호스트명 전체 입력 불필요)
- **클러스터 세션** — tmux(크로스플랫폼) 또는 iTerm2 / Terminal.app(macOS)으로 다중 호스트 동시 접속
- **셸 단축 명령어** — `t`, `r`, `s`, `f`, `m`, `ct`, `cr` 단축 명령어 및 탭 자동완성
- **Web UI** — 브라우저 기반 SSH 터미널 + 서버 관리 대시보드 (`alogin web`)
- **암호화 자격증명 저장소** — macOS Keychain, Linux Secret Service, 또는 `age` 암호화 파일

### 🤖 AI 에이전트를 위한 기능 (MCP)
- **AI 에이전트 통합** — 내장 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 서버로 LLM 클라이언트와 원활하게 연결
- **추상화된 접속** — 에이전트는 비밀번호 복호화나 ProxyJump를 이해할 필요 없이 추상적인 "서버 ID"로 명령 실행 요청만 하면 됨
- **구조화 출력** — 모든 CLI 명령에서 `--format=json` 지원, LLM 파싱 용이
- **완전한 감사 추적** — AI 에이전트가 실행하는 모든 명령이 `JSONL` 감사 로그(`audit.jsonl`)에 기록됨

목차
----

* [설치](#설치)
    * [셸 통합 설정](#셸-통합-설정)
* [핵심 개념: 사람과 에이전트의 역할](#핵심-개념-사람과-에이전트의-역할)
* [활용 시나리오](#활용-시나리오)
    * [시나리오 1: 사람 운영자 (CLI & TUI)](#시나리오-1-사람-운영자-cli--tui)
    * [시나리오 2: AI 인프라 관리 (MCP)](#시나리오-2-ai-인프라-관리-mcp)
* [테스트 환경 (`testenv`)](#테스트-환경-testenv)
* [사용 가이드](#사용-가이드)
    * [빠른 시작](#빠른-시작)
    * [명령어 체계](#명령어-체계)
    * [접속 & 터널](#접속--터널)
* [AI 에이전트 통합 (MCP)](#ai-에이전트-통합-mcp)
    * [MCP 도구 목록](#mcp-도구-목록)
* [고급 주제](#고급-주제)
    * [멀티홉 게이트웨이 라우팅](#멀티홉-게이트웨이-라우팅)
    * [클러스터 세션](#클러스터-세션)
    * [보안 & 자격증명 저장소](#보안--자격증명-저장소)
* [라이선스](#라이선스)

---

## 핵심 개념: 사람과 에이전트의 역할

alogin 2는 사람 관리자가 초기 "신뢰 레이어"를 구성하면 AI 에이전트가 안전하게 운영할 수 있도록 설계되었습니다.

1. **사람 관리자**는 신뢰 관계를 구성할 책임이 있습니다. 서버를 등록하고, 게이트웨이 경로(점프 호스트)를 정의하며, 보안 볼트에 비밀번호를 저장하고, 서버들을 클러스터로 묶습니다.
2. **AI 에이전트**는 MCP 서버를 통해 연결합니다. 사람이 이미 안전한 경로와 자격증명을 구성해두었기 때문에, 에이전트는 *인증 방법*을 알 필요 없이 레지스트리를 조회(`list_servers`)하고, 클러스터를 분석(`get_cluster`)하고, 병렬 SSH 작업을 실행(`exec_on_cluster`)할 수 있습니다.

## 활용 시나리오

### 시나리오 1: 사람 운영자 (CLI & TUI)

일상적인 운영에서 사람은 빠르고 직관적인 접속 방법을 원합니다:

```bash
# 셸 단축어로 주 데이터베이스에 즉시 접속
t db-primary

# 시각적 퍼지 검색 인터페이스로 서버 찾기
alogin tui

# SSHFS로 원격 파일 시스템을 로컬에 마운트
m nas-server /mnt/local_nas

# tmux로 운영 웹 서버 3대에 동시 접속
ct prod-web-cluster
```

### 시나리오 2: AI 인프라 관리 (MCP)

AI 에이전트가 서버를 관리하려면 먼저 사람이 레지스트리를 준비해야 합니다.

**1. 사람 준비 단계**
관리자가 AI가 접근할 수 있도록 원격 인프라를 등록합니다:
```bash
# 1. 서버를 암호화 레지스트리에 추가
alogin compute add --host 10.0.0.10 --user admin  # 볼트 비밀번호 입력 요청
alogin compute add --host 10.0.0.11 --user admin

# 2. AI가 일괄 작업을 쉽게 실행할 수 있도록 클러스터로 묶기
alogin access cluster add web-cluster 10.0.0.10 10.0.0.11
```

**2. 에이전트 실행 단계**
사람이 [Claude Desktop](https://claude.ai/download) 설정에 `alogin agent mcp` 명령을 등록합니다:
```json
{
  "mcpServers": {
    "alogin": {
      "command": "/usr/local/bin/alogin",
      "args": ["agent", "mcp"]
    }
  }
}
```

이제 Claude에게 자연어로 지시할 수 있습니다:
> **사람:** *"web-cluster 전체 디스크 공간 확인해줘."*
> **Claude:** `get_cluster_info`로 노드를 확인하고, `exec_on_cluster`로 두 노드에 `df -h`를 병렬 실행합니다. stdout을 읽고 위험 요소를 요약해 보고합니다.

## 테스트 환경 (`testenv`)

alogin 2는 `testenv/` 디렉토리에 완전히 가상화된 **Docker Compose** 샌드박스를 포함합니다. 에이전트 동작 테스트, 멀티홉 SSH 라우팅 스크립트 작성, 크로스 OS 호환성 검증에 활용할 수 있습니다.

**포함된 노드:**
* `bastion` (Ubuntu 22.04) — 호스트 머신에서 접근 가능한 유일한 노드 (점프 라우팅 테스트용)
* `target-ubuntu` (Ubuntu 24.04) — 프라이빗 백넷의 표준 현대 테스트 노드
* `target-centos7` (CentOS 7) — 레거시 EOL OS(sysvinit, 구버전 패키지 매니저)와의 에이전트 호환성 테스트
* `target-alpine` (Alpine) — 경량 컨테이너와의 상호작용 테스트

**실행 방법:**
```bash
cd testenv/
docker-compose up -d --build
```
이제 `bastion`을 게이트웨이로 등록하고 에이전트에게 `target-ubuntu` 접속을 안전하게 요청할 수 있습니다.

---

## 설치

### 스크립트 설치 (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | sh
```

`~/.local/bin/alogin`에 Web UI 포함 바이너리를 설치합니다. 환경변수로 커스터마이징 가능:

```bash
# CLI-only 버전 (Web UI 제외, 더 작은 파일)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_NO_WEB=1 sh

# 특정 버전 설치
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_VERSION=2.0.3 sh

# 커스텀 설치 경로 (예: /usr/local/bin, sudo 필요 시 별도 처리)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_INSTALL_DIR=/usr/local/bin sh
```

### Homebrew (macOS, 권장)

```bash
brew tap emusal/alogin --custom-remote git@github.com:emusal/alogin2.git
brew install alogin
```

### Windows

네이티브 Windows 바이너리는 미지원입니다. WSL(Windows Subsystem for Linux) 환경에서 위 스크립트로 설치하세요.

### 바이너리 직접 다운로드

[Releases](https://github.com/emusal/alogin2/releases) 페이지에서 직접 받을 수도 있습니다.

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-darwin-arm64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# macOS (Intel)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-darwin-amd64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# Linux (amd64)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-linux-amd64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# Linux (arm64)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-linux-arm64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin
```

### 소스 빌드

Go 1.23 이상 필요.

```bash
git clone https://github.com/emusal/alogin2.git
cd alogin2
go build -o alogin ./cmd/alogin
sudo mv alogin /usr/local/bin/
```

### 업그레이드

```bash
# 최신 버전으로 업그레이드
alogin upgrade

# 확인 프롬프트 건너뜀
alogin upgrade --yes
```

Homebrew로 설치한 경우에는 `brew upgrade alogin`을 사용하세요.

### 제거

```bash
# 바이너리, 완성 스크립트, 설정 제거 (데이터베이스·볼트는 보존)
alogin uninstall

# 모든 데이터 포함 완전 제거 (데이터베이스·볼트까지 삭제, 복구 불가)
alogin uninstall --purge

# 스크립트로 제거 (바이너리가 없거나 원격 실행 시)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/uninstall.sh | sh

# 완전 제거 (스크립트)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/uninstall.sh | ALOGIN_PURGE=1 sh
```

### 셸 통합 설정

`~/.zshrc` 또는 `~/.bashrc`에 다음 줄을 추가하면 단축 별칭과 탭 자동완성이 활성화됩니다:
```bash
source <(alogin shell-init)
```

---

사용 가이드
----------

### 빠른 시작
**1. 설치 확인**
```bash
alogin version
```
**2. 서버 추가 & 접속**
```bash
alogin compute add
alogin access ssh web-01       # 또는 't web-01'
```

### 명령어 체계
기존 v1 명령어는 모두 하위 호환 별칭으로 유지됩니다.

```
alogin compute          서버 레지스트리 관리 (alias: server)
alogin access           원격 접속 (alias: connect, t, r)
alogin auth             자격증명, 게이트웨이 경로, 호스트 별칭 관리
alogin agent            AI MCP 서버, 클라이언트 설정 도구
alogin net              호스트 정의, 백그라운드 SSH 터널
```

**스크립트용 JSON 출력:** 모든 목록 명령에서 `--format=json` 지원.

### 접속 & 터널
```bash
alogin access ssh gw-01 web-01                 # 명시적 2홉 경로
alogin access ssh web-01 --auto-gw             # 등록된 게이트웨이 경유 자동 라우팅
alogin access ssh web-01 --cmd "uptime"        # 명령 실행 후 종료
```

**터널:** tmux 백그라운드 세션으로 SSH 포트포워딩을 영구 유지합니다. 터미널이 종료되어도 살아있습니다.
```bash
alogin net tunnel add web-local --server web-01 --local-port 8080 --remote-port 80
alogin net tunnel start web-local
```

---

## AI 에이전트 통합 (MCP)

alogin에는 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 서버가 내장되어 있어 LLM 클라이언트가 자격증명이나 SSH 라우팅을 직접 다루지 않고도 인프라를 안전하게 관리할 수 있습니다.

> 빠른 시작은 [SKILL.md](SKILL.md), 전체 시스템 프롬프트 레퍼런스는 [docs/SYSTEM_PROMPT.md](docs/SYSTEM_PROMPT.md)를 참고하세요.

### MCP 도구 목록

#### 조회 도구 (읽기 전용)

| 도구 | 설명 |
|------|------|
| `list_servers` | 레지스트리의 서버 목록 조회 / 검색 |
| `get_server` | 단일 서버 상세 정보 조회 |
| `list_clusters` | 클러스터 그룹 목록 및 멤버 수 조회 |
| `get_cluster` | 클러스터 및 멤버 서버 상세 정보 조회 |
| `list_tunnels` | 터널 설정 목록 및 실시간 실행 상태 조회 |
| `get_tunnel` | 단일 터널 상세 정보 및 상태 조회 |
| `inspect_node` | 서버 상태 스냅샷 — CPU, 메모리, 디스크, 상위 프로세스 |

#### 실행 도구 (쓰기)

| 도구 | 설명 |
|------|------|
| `exec_command` | 단일 서버에 SSH 명령 실행 (비대화형 또는 PTY 모드) |
| `exec_on_cluster` | 클러스터 내 모든 서버에 SSH 명령 병렬 실행 |

#### 터널 수명주기 도구

| 도구 | 설명 |
|------|------|
| `start_tunnel` | 저장된 터널을 tmux 백그라운드 세션으로 시작 |
| `stop_tunnel` | 실행 중인 터널 중지 |

`exec_command`, `exec_on_cluster`, `inspect_node` 호출은 모두 `~/.config/alogin/audit.jsonl`에 기록됩니다.

---

고급 주제
---------

### 멀티홉 게이트웨이 라우팅
Go 네이티브 SSH 라이브러리가 ProxyJump를 직접 처리합니다. 경로를 정의(`alogin auth gateway add`)하고 할당하기만 하면 됩니다. alogin은 TCP 스트림에서 `ProxyCommand` 셸을 완전히 우회합니다. 중간 홉에서 `AllowTcpForwarding=no`가 설정된 경우, alogin이 이를 감지하고 자동으로 중첩 `ssh -tt` 가상 터미널 체이닝으로 폴백합니다.

### 클러스터 세션
`ct <cluster>` 실행 시 그룹 내 모든 멤버에 동시 접속하고 입력을 동기화합니다.
```bash
alogin access cluster prod-web --mode tmux      # tmux 창 (macOS + Linux)
alogin access cluster prod-web --mode iterm     # iTerm2 분할 창 (macOS)
```

### 보안 & 자격증명 저장소
비밀번호는 명시적으로 강제하지 않는 한 로컬 SQLite DB에 평문으로 저장되지 않습니다. 우선순위:
1. `macOS Keychain` / `Linux Secret Service`
2. `age 암호화 파일` (폴백)
3. 직접 `~/.ssh/config` 에이전트 처리 (권장! 서버에 키를 먼저 배포하세요)

라이선스
--------
Apache 2.0
