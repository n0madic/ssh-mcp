FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y openssh-server sudo && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /run/sshd

# Create test user with password and sudo access.
RUN useradd -m -d /home/testuser -s /bin/bash testuser && \
    echo "testuser:password" | chpasswd && \
    usermod -aG sudo testuser && \
    echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Allow password authentication.
RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config

# Create test fixtures.
RUN echo "This is a test file" > /home/testuser/test-file.txt && \
    mkdir -p /home/testuser/test-dir && \
    echo "sub-file content" > /home/testuser/test-dir/sub-file.txt && \
    chown -R testuser:testuser /home/testuser

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
