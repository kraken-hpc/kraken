### -*- Mode: Dockerfile; fill-column: 80; comment-auto-fill-only-comments: t; tab-width: 4 -*-
################################################################################
#
# Kraken Dockerfile
#
# Michael Jennings <mej@lanl.gov>
# 05 Feb 2021
#
################################################################################

### Build Arguments (override via "--build-arg")
ARG GOARCH="${GOARCH:-amd64}"
ARG GOOS="${GOOS:-linux}"
ARG GOPATH="${GOPATH:-/go}"
ARG GOVER="1.15"

################################################################################
### Container #1:  Alpine-based Go dev env with Kraken built & installed
################################################################################
FROM registry.lanl.gov:5000/golang:${GOVER}-alpine AS kraken-build
MAINTAINER Michael Jennings <mej@lanl.gov>
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

ARG GOOS
ARG GOARCH
ARG GOPATH
ARG GOVER

WORKDIR "${GOPATH}/src/kraken"
COPY . .

#RUN go get -d -v ...

RUN export GOARCH="${GOARCH:-amd64}" GOOS="${GOOS:-linux}" GOPATH="${GOPATH:-/go}" \
        && go build -v -o "${GOPATH}/bin/kraken-${GOOS}-${GOARCH}" \
        && go install -v \
        && cp -a "${GOPATH}/bin/kraken-${GOOS}-${GOARCH}" /sbin/ \
        && ln -s "kraken-${GOOS}-${GOARCH}" /sbin/kraken \
        && rm -rf "${GOPATH}/pkg"

ENTRYPOINT [ "/sbin/kraken" ]
CMD [ "--help" ]



################################################################################
### Container #2:  Pure Alpine container (no Go) with Kraken copied in
################################################################################
FROM registry.lanl.gov:5000/alpine AS kraken-alpine
MAINTAINER Michael Jennings <mej@lanl.gov>
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY --from=kraken-build /sbin/kraken /sbin/kraken

ENTRYPOINT [ "/sbin/kraken" ]
CMD [ "--help" ]



################################################################################
### Container #3:  Nothing but the Kraken executable (composable/sidecar)
################################################################################
FROM scratch AS kraken-exe
MAINTAINER Michael Jennings <mej@lanl.gov>
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY --from=kraken-build /sbin/kraken /sbin/kraken

ENTRYPOINT [ "/sbin/kraken" ]
CMD [ "--help" ]



################################################################################
### Container #4:  Above "kraken-build" container plus configs
################################################################################
FROM kraken-build AS kraken-build-cfg
MAINTAINER Michael Jennings <mej@lanl.gov>
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY config.yaml state.json /etc/kraken/

ENTRYPOINT [ "/sbin/kraken", "-config", "/etc/kraken/config.yaml", "-state", "/etc/kraken/state.json" ]
CMD [ "--help" ]



################################################################################
### Container #5:  Above "kraken-alpine" container plus configs
################################################################################
FROM kraken-alpine AS kraken-alpine-cfg
MAINTAINER Michael Jennings <mej@lanl.gov>
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY config.yaml state.json /etc/kraken/

ENTRYPOINT [ "/sbin/kraken", "-config", "/etc/kraken/config.yaml", "-state", "/etc/kraken/state.json" ]
CMD [ "--help" ]



################################################################################
