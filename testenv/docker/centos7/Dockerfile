FROM centos:7

# CentOS 7 EOL 이슈 해결을 위해 마러/볼트 아카이브 사용
RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*

RUN yum -y install openssh-server openssh-clients sudo curl vim tree bind-utils iproute net-tools which jq epel-release procps-ng && \
    yum clean all

# SSH 키 생성
RUN ssh-keygen -A

# 사용자 생성 및 비밀번호 설정 (testuser/testuser)
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser" | passwd --stdin testuser && \
    usermod -aG wheel testuser

# sudo 패스워드 없이 사용 가능하도록 설정
RUN echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# SSH 기본 설정 수정
RUN sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config && \
    sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config || true

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
