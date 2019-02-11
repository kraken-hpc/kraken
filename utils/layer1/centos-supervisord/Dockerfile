#
# Supervisor Dockerfile
#

# Pull base image.
FROM centos:latest

LABEL org.label-schema.vcs-url="https://github.com/hpc/kraken" \
  org.label-schema.docker.cmd="docker run -dt -p 6818:6818 -p 3222:22 -h c1 --name layer1 --security-opt='seccomp=unconfined' kraken/layer1-supervisord:1.0" \
  org.label-schema.name="layer1-supervisord" \
  org.label-schema.description="Layer1 running CentOS 7 with supervisord" \
  maintainer="Paul Peltz Jr."

# Variables to use for Slurm install
#ARG SLURM_VERSION=slurm-18.08.3
#ARG SLURM_DOWNLOAD_MD5=a52d8f857ec5a58b2605b643a99dcc71
#ARG SLURM_DOWNLOAD_URL=https://download.schedmd.com/slurm/slurm-18.08.3.tar.bz2
ARG SLURM_VERSION=slurm-17.11.9-2
ARG SLURM_DOWNLOAD_MD5=f0d0fbc730a0f7d1fa02081e2c514ee8
ARG SLURM_DOWNLOAD_URL=https://download.schedmd.com/slurm/slurm-17.11.9-2.tar.bz2
ARG SLURM_BUILD_PACKAGES="rpm-build bzip2 gcc make munge-devel wget readline-devel openssl openssl-devel pam-devel perl-ExtUtils-MakeMaker mysql-devel perl git"
ARG SLURMD_PACKAGES="slurm-${SLURM_VERSION}* slurm-slurmd-* slurm-example-configs* slurm-pam_slurm*"

# Variables for services to run persistently within the layer1
ARG RUNNING_SERVICE_PACKAGES="munge openssh-server ntp rsyslog"

# Use local repos
COPY repos/*.repo /etc/yum.repos.d/

# Install epel-release and tools for Supervisor.
RUN set -e \
  && yum -y install epel-release \
  && yum update -y \
  && yum install -y ${RUNNING_SERVICE_PACKAGES} \
  iproute \
  python-setuptools \
  inotify-tools \
  yum-utils \
  which \
  jq \
  rsync \
  && yum clean all \
  && rm -rf /var/cache/yum

# Install both Slurm and Charliecloud
RUN set -e \
  && yum install -y $SLURM_BUILD_PACKAGES \
  && mkdir -p /root/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS} \
  && cd /root/rpmbuild/SOURCES/ \
  && wget "$SLURM_DOWNLOAD_URL" \
  && echo "$SLURM_DOWNLOAD_MD5" "$SLURM_VERSION".tar.bz2 | md5sum -c - \
  && rpmbuild -tb "$SLURM_VERSION".tar.bz2 \
  && cd /root/rpmbuild/RPMS/x86_64 \
  && yum -y install $SLURMD_PACKAGES \
  && cd \
  && rm -rf /root/rpmbuild \
  && useradd -r -U --uid=101 slurm \
  && cd /usr/local/src \
  && git clone --recursive https://github.com/hpc/charliecloud.git \
  && cd charliecloud \
  && make \
  && make install PREFIX=/usr/local \
  && ch-run --version \
  #   && printf "export CH_TEST_TARDIR=/var/tmp/tarballs\nexport CH_TEST_IMGDIR=/var/tmp/images\nexport CH_TEST_PERMDIRS=skip\n" > /etc/profile.d/charliecloud.sh \
  && echo "clean_requirements_on_remove=1" >> /etc/yum.conf \
  && yum -y autoremove $SLURM_BUILD_PACKAGES epel-release \
  && yum clean all \
  && rm -rf /var/cache/yum /usr/local/src/charliecloud

COPY container-files /

# Create needed files if they weren't in container-files
RUN if [ ! -e /etc/munge/munge.key ]; then \
  /bin/dd if=/dev/urandom bs=1 count=1024 >/etc/munge/munge.key 2>/dev/null; \
  fi \
  && if [ ! -e /etc/ssh/ssh_host_ecdsa_key -a ! -e /etc/ssh/ssh_host_ecdsa_key.pub ]; then \
  ssh-keygen -q -t ecdsa -f /etc/ssh/ssh_host_ecdsa_key -C '' -N ''; \
  fi \
  && if [ ! -e /etc/ssh/ssh_host_ed25519_key -a ! -e /etc/ssh/ssh_host_ed25519_key.pub ]; then \
  ssh-keygen -q -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -C '' -N ''; \
  fi \
  && if [ ! -e /etc/ssh/ssh_host_rsa_key -a ! -e /etc/ssh/ssh_host_rsa_key.pub ]; then \
  ssh-keygen -q -t rsa -f /etc/ssh/ssh_host_rsa_key -C '' -N ''; \
  fi

# Modify permissions and enable services
RUN set -e \
  && chmod 0700 /root/ /root/.ssh \
  && chmod 0600 /root/.ssh/authorized_keys \
  && chmod 0644 /etc/ssh/*.pub /etc/ssh/moduli \
  && chmod 0640 /etc/ssh/ssh_*key \
  && chgrp ssh_keys /etc/ssh/ssh_*key \
  && chmod 0600 /etc/ssh/sshd_config \
  && chown munge:munge -R /run/munge \
  /etc/munge \
  /var/lib/munge \
  /var/log/munge \
  && chmod 0755 /etc \
  && chmod 0700 /etc/munge \
  && chmod 0600 /etc/munge/munge.key \
  && rm /etc/localtime \
  && ln -s /usr/share/zoneinfo/America/Denver /etc/localtime \
  && echo "test" | passwd --stdin root \
  && useradd -u 5611 -U -m peltz

RUN set -e \
  && easy_install supervisor \
  && mkdir /etc/supervisor \
  && chmod +x /config/bootstrap.sh

# Expose slurmd sshd port
EXPOSE 6818:6818 3222:22

# Define default command.
ENTRYPOINT ["/config/bootstrap.sh"]
