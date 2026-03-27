ARG BASE_IMAGE=ubuntu:22.04
FROM ${BASE_IMAGE}

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y openssh-server sudo curl dnsutils iproute2 vim tree iputils-ping sudo procps net-tools jq && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir /var/run/sshd

# 사용자 생성 및 비밀번호 설정 (testuser/testuser)
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser:testuser" | chpasswd && \
    usermod -aG sudo testuser

# sudo 패스워드 없이 사용 가능하도록 설정
RUN echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# SSH 기본 설정 수정 (root 로그인 금지, 패스워드 인증 활성화)
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/UsePAM yes/UsePAM no/' /etc/ssh/sshd_config

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
