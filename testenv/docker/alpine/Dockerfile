FROM alpine:latest

RUN apk update && \
    apk add --no-cache openssh bash sudo curl vim tree bind-tools iproute2 procps jq net-tools

# SSH 호스트 키 생성
RUN ssh-keygen -A

# 사용자 생성 (testuser/testuser)
RUN adduser -D -s /bin/bash testuser && \
    echo "testuser:testuser" | chpasswd

# sudo 설정
RUN echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# SSH 설정 수정
RUN sed -i s/#PermitRootLogin.*/PermitRootLogin\ no/ /etc/ssh/sshd_config && \
    sed -i s/#PasswordAuthentication.*/PasswordAuthentication\ yes/ /etc/ssh/sshd_config

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
