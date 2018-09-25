#
# Supervisor Dockerfile
#

# Pull base image.
FROM centos/systemd

LABEL org.label-schema.vcs-url="https://github.com/lanl/kraken" \
      org.label-schema.docker.cmd="docker run -dt -v /sys/fs/cgroup:/sys/fs/cgroup:ro --cap-add=SYS_ADMIN -e "container=docker" -p 3222:22 -h c1 kraken/layer1-systemd:1.0" \
      org.label-schema.name="layer1-systemd" \
      org.label-schema.description="Layer1 running CentOS 7 with systemd" \
      maintainer="Paul Peltz Jr."

# Variables to use for SLURM install
ARG SLURM_VERSION=slurm-17.11.9-2
ARG SLURM_DOWNLOAD_MD5=f0d0fbc730a0f7d1fa02081e2c514ee8
ARG SLURM_DOWNLOAD_URL=https://download.schedmd.com/slurm/slurm-17.11.9-2.tar.bz2
ARG SLURM_BUILD_PACKAGES="rpm-build bzip2 gcc make munge-devel wget readline-devel openssl openssl-devel pam-devel perl-ExtUtils-MakeMaker mysql-devel perl"
ARG SLURMD_PACKAGES="slurm-17.11* slurm-slurmd-* slurm-example-configs* slurm-pam_slurm*"

# Variables for services to run persistently within the layer1
ARG RUNNING_SERVICE_PACKAGES="munge openssh-server ntp rsyslog"

# Use local repos
COPY repos/*.repo /etc/yum.repos.d/

# Install OHPC and tools for Supervisor.
RUN set -x \
    && yum -y install epel-release \
    && yum update -y \
    && yum install -y ${RUNNING_SERVICE_PACKAGES}

RUN set -x \
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
    && mkdir /etc/sysconfig/slurm \
        /var/spool/slurm \
        /var/run/slurmd \
        /var/lib/slurmd \
        /var/log/slurm \
        /data \
    && touch /var/lib/slurmd/node_state \
        /var/lib/slurmd/front_end_state \
        /var/lib/slurmd/job_state \
        /var/lib/slurmd/resv_state \
        /var/lib/slurmd/trigger_state \
        /var/lib/slurmd/assoc_mgr_state \
        /var/lib/slurmd/assoc_usage \
        /var/lib/slurmd/qos_usage \
        /var/lib/slurmd/fed_mgr_state \
    && groupadd -r slurm --gid=995 && useradd -r -g slurm --uid=995 slurm \
    && chown -R slurm:slurm /var/*/slurm* \
    && echo "clean_requirements_on_remove=1" >> /etc/yum.conf \
    && yum -y autoremove $SLURM_BUILD_PACKAGES epel-release \
    && yum clean all \
    && rm -rf /var/cache/yum

COPY container-files /

# Modify permissions and enable services
RUN set -x \
    && chmod 0700 /root/.ssh \
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
    && systemctl enable munge slurmd sshd rsyslog ntpd

# Expose slurmd sshd port
EXPOSE 6818:6818 3222:22

# Launch the bootstrap
CMD ["/usr/sbin/init"]
