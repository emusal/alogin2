FROM centos:6

# CentOS 6 EOL Archive (Vault) setting
RUN sed -i 's/^mirrorlist=/#mirrorlist=/g' /etc/yum.repos.d/CentOS-Base.repo && \
    sed -i 's/^#baseurl=http:\/\/mirror.centos.org\/centos\/\$releasever/baseurl=http:\/\/vault.centos.org\/6.10/g' /etc/yum.repos.d/CentOS-Base.repo && \
    sed -i 's/enabled=1/enabled=0/g' /etc/yum.repos.d/CentOS-fasttrack.repo || true

# epel for extra packages if needed, but might fail. Using only base.
RUN yum -y install openssh-server openssh-clients sudo curl vim tree bind-utils iproute net-tools which procps passwd && \
    yum clean all

# SSH Key generation
RUN ssh-keygen -q -t rsa -f /etc/ssh/ssh_host_rsa_key -C '' -N '' && \
    ssh-keygen -q -t dsa -f /etc/ssh/ssh_host_dsa_key -C '' -N ''

# testuser setup
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser" | passwd --stdin testuser && \
    usermod -aG wheel testuser

RUN echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# SSH config
RUN sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config && \
    sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config || true

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
