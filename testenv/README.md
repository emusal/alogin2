# alogin2 Test Environment

이 환경은 `alogin2` 에이전트의 다양한 기능(인프라 정보 수집, 멀티홉 동작, 레거시 OS 접근성 등)을 테스트하기 위한 **Docker Compose** 기반 가상화 환경입니다.

## 아키텍처 및 구성

이 환경은 외부망(`front_net`)과 내부망(`back_net`)으로 분리된 네트워크 구조를 가집니다.

| 노드 이름 | OS / 버전 | 네트워크 | 목적 |
|---|---|---|---|
| `bastion` | Ubuntu 22.04 | `front_net`, `back_net` | 외부에서 접근 가능한 유일한 서버, 멀티홉(Jump 호스트) 기반 접속 테스트용 |
| `target-ubuntu` | Ubuntu 24.04 | `back_net` | 최신 OS 환경에서의 정보 수집 테스트용 |
| `target-alpine` | Alpine Linux | `back_net` | 경량 컨테이너 OS 환경, 다른 패키지 관리자 구조에서의 수집 테스트용 |
| `target-centos7` | CentOS 7 | `back_net` | EOL된 레거시 구형 OS(sysvinit/구버전 systemd) 환경에서의 수집 테스트용 |

## 초기 설정 및 실행

테스트 환경을 실행하려면 다음 명령어를 사용하세요:

```bash
docker-compose up -d --build
```
> 컨테이너를 중지하려면 `docker-compose down` 명령을 사용합니다.

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
```

## `alogin2` 테스트 시나리오 예시

로컬 머신에서 바로 내부망(`target-ubuntu`)에 대해 alogin의 Agent나 정보수집 커맨드를 실행하려고 하는 경우, `bastion`을 Jump host로 지정하여 접근이 되는지(`ProxyJump` 등 멀티홉 확인) 테스트할 수 있습니다.

```bash
# 예시: 로컬 터미널에서 Bastion을 경유하여 target-ubuntu 접근
ssh -p 2222 -J testuser@localhost target-ubuntu
```
