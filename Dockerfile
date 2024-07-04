ARG _DEB_VERSION="12"

ARG ARCH="arm64/v8"

ARG BASE="debian:${_DEB_VERSION}"
ARG BASE_PKGS="php"
ARG BASE_BINS="php"
ARG BASE_PKG_INSTALL_CMD="apt-get update && apt-get install -y"

ARG BUSYBOX="busybox:latest"

ARG TARGET="gcr.io/distroless/base-nossl-debian${_DEB_VERSION}:latest"

FROM --platform="linux/${ARCH}" ${BUSYBOX} AS busybox

FROM --platform="linux/${ARCH}" ${BASE} AS base

ARG BASE_PKGS
ARG BASE_BINS
ARG BASE_PKG_INSTALL_CMD

COPY --chmod=755 "dependency_resolve" "/usr/local/bin/dependency_resolve"

RUN /bin/sh -c "${BASE_PKG_INSTALL_CMD} ${BASE_PKGS}" \
 && /usr/local/bin/dependency_resolve \
      "$(which "ldd")" \
      $(echo "${BASE_BINS}" | xargs which) \
    | xargs -I {} sh -c 'mkdir -p /root/rootfs/$(dirname "{}") && cp -apP "{}" "/root/rootfs/{}"'

FROM --platform="linux/${ARCH}" ${TARGET} AS target

ARG BASE_BINS

COPY --from=base "/root/rootfs" "/"

COPY --from=busybox "/bin/busybox" "/bin/busybox"
RUN ["/bin/busybox", "--install", "-s"]

USER nonroot

ENTRYPOINT ["/bin/sh"]
