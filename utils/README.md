# How to build and run the layer1 container

## Build Steps
cd utils/layer1/centos-supervisord
docker build --rm -t kraken/layer1-supervisord:1.0 .

## Run the container
docker run -dt -p 6818:6818 -p 3222:22 -h c1 --security-opt='seccomp=unconfined' --name layer1 kraken/layer1-supervisord:1.0

## Configuration

If you want to use consistent ssh keys, pull those out of the running container and put them in layer1/centos-supervisord/container-files/etc/ssh/

If you want to connect this container to an existing slurm cluster then add the munge.key for that cluster to layer1/centos-supervisord/container-files/etc/munge/

The ``COPY container-files /`` in the Dockerfile will take care of putting any files into the container image. You will be responsbile for setting the correct permissions and ownerships in the following RUN section however.

## Supervisord

If you want to add additional services to run in the layer1 container add the service's package name to the ``ARG RUNNING_SERVICE_PACKAGES=`` variable and they will get installed.

Next you will need to add a supervisor.d file for them to get started up by supervisord. Either copy an existing one or see http://supervisord.org/configuration.html for more information.
