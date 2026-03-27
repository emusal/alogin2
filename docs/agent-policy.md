# agent-policy.yaml 작성 가이드

alogin MCP 서버의 에이전트 안전 정책(HITL/RBAC)을 YAML로 정의하는 방법을 설명합니다.

---

## 파일 위치

| 종류 | 경로 |
|------|------|
| 글로벌 정책 | `~/.config/alogin/agent-policy.yaml` |
| 서버별 정책 | DB에 인라인 YAML (`alogin agent server-policy set <id>`) |

파일이 없으면 **빌트인 파괴적 명령 감지**만 동작합니다 (`rm`, `shutdown`, `dd` 등은 자동으로 `require_approval`).

---

## 최상위 구조

```yaml
version: 1
default_action: allow        # 어떤 규칙에도 매칭되지 않을 때의 기본 동작
hitl_timeout_sec: 120        # HITL 승인 대기 시간 (초, 기본값 120)
rules:
  - name: "규칙 이름"
    match:
      commands:    [...]
      agent_id:    [...]
      server_ids:  [...]
      cluster_ids: [...]
      time_window: "HH:MM-HH:MM"
    action: allow | deny | require_approval
```

### `default_action`

| 값 | 설명 |
|----|------|
| `allow` | 매칭 규칙 없으면 허용 (기본값) |
| `deny` | 매칭 규칙 없으면 차단 |
| `require_approval` | 매칭 규칙 없으면 HITL 승인 요청 |

---

## 규칙 평가 방식

- **순서대로 평가**, 첫 번째 매칭 규칙에서 즉시 결정 (first-match-wins)
- `match` 필드는 모두 **AND 조건** — 여러 필드를 지정하면 모두 충족해야 매칭
- 각 필드를 생략하면 해당 조건은 **와일드카드** (모두 매칭)

---

## match 필드 상세

### `commands` — 정규식 패턴 목록

명령어 중 **하나라도** 패턴 중 하나에 매칭되면 조건 충족 (OR).

```yaml
match:
  commands:
    - "^rm\\s"          # rm으로 시작
    - "^(shutdown|reboot|halt)$"
    - "DROP\\s+TABLE"   # 대소문자 구분
    - "(?i)truncate"    # (?i) 플래그로 대소문자 무시
```

> Go `regexp` 문법 사용. `\b`, `(?i)`, `(?:...)` 등 지원.
> [Go regexp 문법 참조](https://pkg.go.dev/regexp/syntax)

### `agent_id` — glob 패턴 목록

MCP 클라이언트가 전달하는 `X-Agent-ID` 헤더 값과 매칭. `path.Match` 글로브 사용.

```yaml
match:
  agent_id:
    - "claude-*"      # claude-로 시작하는 모든 에이전트
    - "prod-agent"    # 정확히 "prod-agent"
```

에이전트 ID를 전달하지 않는 클라이언트는 빈 문자열 `""`로 평가됩니다.

### `server_ids` — 서버 ID 목록 (정수)

`alogin server list`에서 확인하는 서버 ID (정확히 일치).

```yaml
match:
  server_ids: [3, 7, 12]   # ID 3, 7, 12번 서버에만 적용
```

### `cluster_ids` — 클러스터 ID 목록 (정수)

`alogin cluster list`에서 확인하는 클러스터 ID.

```yaml
match:
  cluster_ids: [1, 2]
```

### `time_window` — UTC 시간 범위

`"HH:MM-HH:MM"` 형식, UTC 기준. 자정을 넘는 범위도 지원.

```yaml
match:
  time_window: "22:00-06:00"  # 야간 (UTC 22시~06시)
  time_window: "09:00-18:00"  # 업무 시간
```

---

## action 값

| 값 | 동작 |
|----|------|
| `allow` | 즉시 허용 |
| `deny` | 즉시 차단 (에러 반환) |
| `require_approval` | 파일 기반 HITL 승인 대기 (`alogin agent approve/deny <token>`) |

---

## 클러스터 정책 전략

`exec_on_cluster` 호출 시 멤버 서버별 정책을 각각 평가하여 **가장 엄격한 결과**를 채택합니다.

엄격도 순서: `allow` < `require_approval` < `deny`

예: 10개 멤버 중 1개 서버가 `deny`이면 전체 클러스터 exec가 차단됩니다.

---

## 예제

### 1. 최소 설정 — 파괴적 명령만 승인 필요

```yaml
version: 1
default_action: allow
hitl_timeout_sec: 120

rules:
  - name: block-destructive
    match:
      commands:
        - "^rm\\s+-[rRfF]*[rR]"   # rm -rf 계열
        - "^dd\\s"
        - "^mkfs"
        - "^(shutdown|reboot|halt|poweroff)$"
    action: require_approval
```

### 2. 프로덕션 서버 보호 — 특정 서버는 읽기 전용

```yaml
version: 1
default_action: allow

rules:
  # prod 서버(ID 5, 6)에서 상태 변경 명령은 전면 차단
  - name: prod-write-deny
    match:
      server_ids: [5, 6]
      commands:
        - "^(rm|mv|cp|chmod|chown|apt|yum|pip|npm)\\s"
        - "^(systemctl|service)\\s+(start|stop|restart|enable|disable)"
        - "^(shutdown|reboot|halt)"
    action: deny

  # prod 서버 모든 exec는 승인 필요
  - name: prod-exec-approval
    match:
      server_ids: [5, 6]
    action: require_approval
```

### 3. 업무 시간 외 차단

```yaml
version: 1
default_action: allow

rules:
  # 업무 시간(UTC 00:00-09:00 = KST 09:00-18:00) 외 모든 exec 차단
  - name: after-hours-deny
    match:
      time_window: "09:00-00:00"   # UTC 09:00 이후 (KST 18:00 이후)
    action: deny
```

### 4. 특정 에이전트만 허용, 나머지 차단

```yaml
version: 1
default_action: deny   # 등록된 에이전트 외 모두 차단

rules:
  - name: allow-known-agents
    match:
      agent_id:
        - "claude-desktop-*"
        - "my-trusted-agent"
    action: allow
```

### 5. 야간 위험 명령 HITL + 나머지 허용

```yaml
version: 1
default_action: allow
hitl_timeout_sec: 300   # 5분 대기

rules:
  - name: night-destructive-hitl
    match:
      time_window: "21:00-09:00"
      commands:
        - "^(rm|dd|mkfs|shutdown|reboot)\\s*"
    action: require_approval

  - name: always-deny-format
    match:
      commands: ["^mkfs\\."]
    action: deny
```

---

## 서버별 정책 설정

글로벌 정책과 별개로 특정 서버에만 다른 정책을 적용할 수 있습니다.

```bash
# YAML 파일로 설정
alogin agent server-policy set 5 --file prod-policy.yaml

# stdin으로 설정
cat <<'EOF' | alogin agent server-policy set 5 --stdin
version: 1
default_action: require_approval
rules:
  - name: allow-safe-reads
    match:
      commands: ["^(cat|ls|ps|df|free|uptime|who|w)\\b"]
    action: allow
EOF

# 확인
alogin agent server-policy show 5

# 제거 (글로벌 정책으로 복귀)
alogin agent server-policy clear 5
```

> 서버별 정책은 글로벌 정책을 **완전히 대체**합니다 (fallback 없음).
> 클러스터 exec에서는 멤버마다 자신의 정책을 적용 후 가장 엄격한 결과를 사용합니다.

---

## 정책 검증

```bash
# 문법 오류 및 정규식 오류 확인
alogin agent policy validate

# 현재 적용 중인 정책 내용 출력
alogin agent policy show
```

---

## 빌트인 파괴적 명령 패턴

`agent-policy.yaml`이 없어도 아래 패턴은 항상 `require_approval`로 처리됩니다.

| 패턴 | 대상 명령 |
|------|-----------|
| `^rm\s` / `^rm$` | rm |
| `\brm\s+-[rRfFi]*[rR]` | rm -rf 계열 |
| `^dd\s` | dd |
| `^mkfs` | mkfs |
| `^(shutdown\|reboot\|halt\|poweroff)` | 시스템 종료 |
| `^(systemctl\|service)\s+(stop\|disable\|mask)` | 서비스 중지 |
| `(?i)(^drop\|^truncate)\s+.*table` | DB 테이블 삭제 |
| `^>\s` | 파일 덮어쓰기 리다이렉션 |

---

## HITL 워크플로우

정책 결과가 `require_approval`이면:

1. 에이전트 실행이 **일시 중단**됨
2. stderr에 승인 요청 메시지와 토큰 출력
3. 운영자가 다른 터미널에서 승인/거부:
   ```bash
   alogin agent pending           # 대기 중인 요청 목록
   alogin agent approve <token>   # 승인 → 에이전트 재개
   alogin agent deny    <token>   # 거부 → 에이전트에 오류 반환
   ```
4. `hitl_timeout_sec` 초 내 응답 없으면 자동 거부

승인 파일 위치: `~/.config/alogin/hitl/{pending,approved,denied}/`
