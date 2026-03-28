# alogin2 Test Environment

이 환경은 `alogin2` 에이전트의 다양한 기능(인프라 정보 수집, 멀티홉 동작, 레거시 OS 접근성, 플러그인 시스템 등)을 테스트하기 위한 **Docker Compose** 기반 가상화 환경입니다.

## 아키텍처 및 구성

이 환경은 외부망(`front_net`)과 내부망(`back_net`)으로 분리된 네트워크 구조를 가집니다.

### SSH 타겟 서버

| 노드 이름 | OS / 버전 | 네트워크 | 목적 |
|---|---|---|---|
| `bastion` | Ubuntu 22.04 | `front_net`, `back_net` | 외부에서 접근 가능한 유일한 서버, 멀티홉(Jump 호스트) 기반 접속 테스트용 |
| `target-ubuntu` | Ubuntu 24.04 | `back_net` | 최신 OS 환경에서의 정보 수집 테스트용 |
| `target-alpine` | Alpine Linux | `back_net` | 경량 컨테이너 OS 환경, 다른 패키지 관리자 구조에서의 수집 테스트용 |
| `target-centos7` | CentOS 7 | `back_net` | EOL된 레거시 구형 OS(sysvinit/구버전 systemd) 환경에서의 수집 테스트용 |
| `target-centos6` | CentOS 6 | `back_net` | 더 오래된 레거시 OS 환경 테스트용 |
| `target-legacy-rsa` | Ubuntu (RSA only) | `back_net` | 구형 RSA 키 알고리즘 제한 환경 테스트용 |

### 3-hop 테스트 서버

| 노드 이름 | OS / 버전 | 네트워크 | 목적 |
|---|---|---|---|
| `middle` | Ubuntu 22.04 | `back_net`, `inner_net` | 2번째 점프 호스트 (bastion → middle 경유) |
| `deep-target` | Ubuntu 24.04 | `inner_net` | 3단계 홉 최종 목적지 (bastion → middle → deep-target) |

### 플러그인 시스템 테스트 서버 (DB/Cache)

| 노드 이름 | 서비스 | 네트워크 | 목적 |
|---|---|---|---|
| `target-mariadb` | MariaDB | `back_net` | `--app mariadb` 플러그인 테스트용 |
| `target-redis` | Redis | `back_net` | `--app redis` 플러그인 테스트용 |
| `target-postgres` | PostgreSQL | `back_net` | `--app postgres` 플러그인 테스트용 |
| `target-mongo` | MongoDB | `back_net` | `--app mongo` 플러그인 테스트용 |

## 초기 설정 및 실행

### 1. 컨테이너 기동

```bash
docker-compose up -d --build
```

> 컨테이너를 중지하려면 `docker-compose down` 명령을 사용합니다.

### 2. alogin 레지스트리 등록

컨테이너가 모두 기동된 후 setup 스크립트를 실행합니다:

```bash
bash testenv/setup_alogin_cluster.sh
```

스크립트가 수행하는 작업:
1. `bastion_host` (localhost:2222) 호스트 등록 및 서버 등록
2. `bastion_gw` 게이트웨이 경로 등록
3. SSH 타겟 서버 5개 등록 (gateway: bastion_gw)
4. DB/Cache 플러그인 테스트 서버 4개 등록 (gateway: bastion_gw)
5. 3-hop 체인 등록: `middle` (GatewayID: bastion_gw) + `deep-target` (GatewayServerID: middle)
6. `test-cluster` (SSH 서버 5개), `db-cluster` (DB 서버 4개) 클러스터 생성
7. `testenv/plugins/*.yaml` → `~/.config/alogin/plugins/` 설치
8. `app-server` 바인딩 4개 등록 (mariadb-test, redis-test, postgres-test, mongo-test)

## 접속 정보

모든 컨테이너에는 아래와 같은 공통 테스트 계정이 미리 설정되어 있습니다.

- **Username:** `testuser`
- **Password:** `testuser`
- 권한: 암호 없는 `sudo` 권한 부여됨

### 로컬 머신 (Host) -> Bastion

`bastion` 서버는 포트 매핑이 되어 있으므로 로컬 컴퓨터에서 바로 접속할 수 있습니다:

```bash
ssh -p 2222 testuser@localhost
```

### Bastion -> 내부망 (Multi-hop)

Bastion 쉘 안에서는 각각의 `target-*` 서버로 바로 ssh 접속이 가능합니다. (`back_net` 네트워크 안의 호스트명 사용)

```bash
# bastion 서버 쉘 안에서
ssh target-ubuntu
ssh target-alpine
ssh target-centos7
ssh target-mariadb
```

## `alogin` 테스트 시나리오 예시

### 멀티홉 SSH

```bash
# Bastion을 경유하여 target-ubuntu 접근 (2홉)
alogin access ssh target-ubuntu --auto-gw

# 레거시 RSA 환경 접근 (2홉)
alogin access ssh target-legacy-rsa --auto-gw
```

### 3-hop SSH

`inner_net`은 `back_net`과도 격리되어 있으므로 `deep-target`은 반드시 bastion → middle을 경유해야 합니다.

```bash
# localhost → bastion → middle → deep-target (3홉)
alogin access ssh deep-target --auto-gw
```

### 클러스터 일괄 접속

```bash
# SSH 타겟 서버 5개 tmux 창 열기
alogin access cluster test-cluster --mode tmux

# DB 서버 4개 tmux 창 열기
alogin access cluster db-cluster --mode tmux
```

### 플러그인 시스템 (app-server)

```bash
# MariaDB 접속 (vault 비밀번호 자동 주입)
alogin app-server connect mariadb-test

# Redis 접속
alogin app-server connect redis-test

# PostgreSQL 접속
alogin app-server connect postgres-test

# MongoDB 접속
alogin app-server connect mongo-test
```

### MCP Agent를 통한 일괄 명령 실행

```bash
# alogin MCP 서버 기동 후 AI 클라이언트(Claude Desktop 등)에서:
# exec_on_cluster tool로 db-cluster 전체에 명령 실행
alogin agent mcp
```
