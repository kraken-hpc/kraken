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
ARG GOVER="1.15"
ARG ALPINE="3.13"

################################################################################
### Container #1:  Alpine-based Go dev env with Kraken built & installed
################################################################################
FROM docker.io/library/golang:${GOVER}-alpine AS kraken-build
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

ARG GOARCH="${GOARCH:-amd64}"
ARG GOOS="${GOOS:-linux}"

WORKDIR "${GOPATH}/src/kraken"
COPY . .

RUN export GOARCH="${GOARCH}" GOOS="${GOOS}" \
        && env \
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
FROM docker.io/library/alpine:${ALPINE} AS kraken-alpine
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY --from=kraken-build /sbin/kraken /sbin/kraken

ENTRYPOINT [ "/sbin/kraken" ]
CMD [ "--help" ]



################################################################################
### Container #3:  Nothing but the Kraken executable (composable/sidecar)
################################################################################
FROM scratch AS kraken
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY --from=kraken-build /sbin/kraken /sbin/kraken

ENTRYPOINT [ "/sbin/kraken" ]
CMD [ "--help" ]



################################################################################
### Container #4:  Above "kraken-build" container plus configs
################################################################################
FROM kraken-build AS kraken-build-cfg
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY config.yaml state.json /etc/kraken/

ENTRYPOINT [ "/sbin/kraken", "-config", "/etc/kraken/config.yaml", "-state", "/etc/kraken/state.json" ]
CMD [ "--help" ]



################################################################################
### Container #5:  Above "kraken-alpine" container plus configs
################################################################################
FROM kraken-alpine AS kraken-alpine-cfg
LABEL maintainer="Michael Jennings <mej@lanl.gov>"

COPY config.yaml state.json /etc/kraken/

ENTRYPOINT [ "/sbin/kraken", "-config", "/etc/kraken/config.yaml", "-state", "/etc/kraken/state.json" ]
CMD [ "--help" ]



################################################################################
