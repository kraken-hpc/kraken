# How to build and run the layer1 container

## Build Steps

```bash
cd utils/layer1/centos-supervisord

docker build --rm -t kraken/layer1-supervisord:1.0 .
```

## Run the container

```bash
docker run -dt -p 6818:6818 -p 3222:22 -h c1 --security-opt='seccomp=unconfined' --name layer1 kraken/layer1-supervisord:1.0
```

The ``-p 6818:6818`` forwards the slurmd port, and ``-p 3222:22`` forwards the ssh port into the container. The ``--security-opt='seccomp=unconfined`` is to allow a nested user namespace container for charliecloud to run.

## Configuration

If you want to use consistent ssh keys, pull those out of the running container and put them in ``layer1/centos-supervisord/container-files/etc/ssh/``

If you want to connect this container to an existing slurm cluster then add the ``munge.key`` for that cluster to ``layer1/centos-supervisord/container-files/etc/munge/``

The ``COPY container-files /`` in the Dockerfile will take care of putting any files into the container image. You will be responsbile for setting the correct permissions and ownerships in the following RUN section however.

## Supervisord

If you want to add additional services to run in the layer1 container add the service's package name to the ``ARG RUNNING_SERVICE_PACKAGES=`` variable and they will get installed.

Next you will need to add a supervisor.d file for them to get started up by supervisord. Either copy an existing one or see <http://supervisord.org/configuration.html> for more information.

## Running a charliecloud job

Make sure you have a consistent uid/gid inside your container. I do this by the following in the Dockerfile:

```bash
useradd -u 5611 -U -m peltz
```

You will also need to mount your home area in the container runtime. You can modify the docker run to achieve this.

```bash
docker run -dt -p 6818:6818 -p 3222:22 -h c1 --mount type=bind,source="/home/peltz",target=/home/peltz --security-opt='seccomp=unconfined' --name layer1 kraken/layer1-supervisord:1.0
```

In order to build a mpihello container for charliecloud you can follow the instructions on their website. <https://hpc.github.io/charliecloud/test.html>

Once you have created the mpihello.tar.gz file and it is in a place that both the cluster and container have mounted, i.e. /home/peltz in my example. Now you can use this simple sbatch script to launch the container:

```bash
#!/bin/bash
#SBATCH --time=0:10:00
#SBATCH -p container

# Arguments: Path to tarball, path to image parent directory.

set -e

TAR="$1"
IMGDIR="$2"
IMG="$2/$(basename "${TAR%.tar.gz}")"

if [[ -z $TAR ]]; then
    echo 'no tarball specified' 1>&2
    exit 1
fi
printf 'tarball:   %s\n' "$TAR"

if [[ -z $IMGDIR ]]; then
    echo 'no image directory specified' 1>&2
    exit 1
fi
printf 'image:     %s\n' "$IMG"

# Unpack image.
srun -N $SLURM_NNODES ch-tar2dir "$TAR" "$IMGDIR"

# MPI version in container.
printf 'container: '
ch-run "$IMG" -- mpirun --version | grep -E '^mpirun'
```

## Run the app.

```shell
srun --cpus-per-task=1 ch-run "$IMG" -- /hello/hello
```

Submit the job:

```shell
sbatch -N 1 mpihello.sbatch ~/mpihello.tar.gz /var/tmp
```

Check the output:

```shell
[peltz@c1 ~]$ cat slurm-1230.out
tarball:   /home/peltz/mpihello.tar.gz
image:     /var/tmp/mpihello
replacing existing image /var/tmp/mpihello
/var/tmp/mpihello unpacked ok
container: mpirun (Open MPI) 2.1.5
0: init ok c1, 1 ranks, userns 4026533176
0: send/receive ok
0: finalize ok
```
