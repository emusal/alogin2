FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y openssh-server sudo vim net-tools jq && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir /var/run/sshd

# Create testuser
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser:testuser" | chpasswd && \
    usermod -aG sudo testuser

RUN echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Remove all non-RSA host keys
RUN rm -f /etc/ssh/ssh_host_ecdsa_key* /etc/ssh/ssh_host_ed25519_key* /etc/ssh/ssh_host_dsa_key*

# Configure SSH to only use RSA
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/UsePAM yes/UsePAM no/' /etc/ssh/sshd_config && \
    echo "HostKey /etc/ssh/ssh_host_rsa_key" >> /etc/ssh/sshd_config && \
    echo "HostKeyAlgorithms ssh-rsa" >> /etc/ssh/sshd_config && \
    echo "PubkeyAcceptedKeyTypes ssh-rsa" >> /etc/ssh/sshd_config

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
