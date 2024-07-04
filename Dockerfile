ARG ARCH="arm64/v8"
ARG PKGS="bash curl php"
ARG BINS="bash curl php"

ARG BASE_IMAGE="debian"
ARG BASE_TAG="12"
ARG BASE_PKG_INSTALL_CMD="apt-get update && apt-get install -y"

ARG TARGET_IMAGE="gcr.io/distroless/base-nossl-debian${BASE_TAG}"
ARG TARGET_TAG="latest"

FROM --platform="linux/${ARCH}" ${BASE_IMAGE}:${BASE_TAG} AS base

ARG PKGS
ARG BINS
ARG BASE_PKG_INSTALL_CMD

COPY --chmod=755 "dependency_resolve" "/usr/local/bin/dependency_resolve"

RUN /bin/sh -c "${BASE_PKG_INSTALL_CMD} ${PKGS}" \
 && /usr/local/bin/dependency_resolve \
      "$(which "ldd")" \
      $(echo "${BINS}" | xargs which) \
    | xargs -I {} sh -c 'mkdir -p /root/rootfs/$(dirname "{}") && cp -apP "{}" "/root/rootfs/{}"' \
&& for BINARY in ${BINS}; do \
     "${BINARY}" --version >> "/root/rootfs/expect.txt"; \
   done

FROM --platform="linux/${ARCH}" busybox:latest as busybox

FROM --platform="linux/${ARCH}" ${TARGET_IMAGE}:${TARGET_TAG} as target

ARG PKGS
ARG BINS

COPY --from=base "/root/rootfs" "/"

COPY --from=busybox "/bin/busybox" "/bin/busybox"
RUN ["/bin/busybox", "ln", "-s", "/bin/busybox", "/bin/sh"]

ENTRYPOINT ["/bin/sh"]
