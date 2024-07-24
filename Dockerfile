ARG _DEB_VERSION="12"

ARG PLATFORM="linux/arm64/v8"

ARG BASE="debian:${_DEB_VERSION}"
ARG BASE_PKGS="php"
ARG BASE_BINS="php"
ARG BASE_PKG_INSTALL_CMD="apt-get update && apt-get install -y"

ARG GOLANG="golang:latest"

ARG BUSYBOX="busybox:latest"

ARG TARGET="gcr.io/distroless/base-nossl-debian${_DEB_VERSION}:latest"

FROM --platform="${PLATFORM}" ${GOLANG} AS golang

COPY "./" "/build"

RUN cd "/build" \
 &&   go get \
 &&   go build -o "/usr/local/bin/dependency_resolve" . \
 &&   strip --strip-all "/usr/local/bin/dependency_resolve" \
 && cd -

FROM --platform="${PLATFORM}" ${BUSYBOX} AS busybox

FROM --platform="${PLATFORM}" ${BASE} AS base

ARG BASE_PKGS
ARG BASE_BINS
ARG BASE_PKG_INSTALL_CMD

COPY --from=golang "/usr/local/bin/dependency_resolve" "/usr/local/bin/dependency_resolve"

RUN /bin/sh -c "${BASE_PKG_INSTALL_CMD} ${BASE_PKGS}" \
 && /usr/local/bin/dependency_resolve \
      $(echo "${BASE_BINS}" | xargs which) \
    | xargs -I {} sh -c 'mkdir -p /rootfs/$(dirname "{}") && cp -apP "{}" "/rootfs/{}" && (strip --strip-all "/rootfs/{}" || true)'

FROM --platform="${PLATFORM}" ${TARGET} AS target

ARG BASE_BINS

COPY --from=base "/rootfs" "/"

COPY --from=busybox "/bin/busybox" "/bin/busybox"
RUN ["/bin/busybox", "--install", "-s"]

USER nonroot

ENTRYPOINT ["/bin/sh"]
