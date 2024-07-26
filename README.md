# dependency_resolve - distroless packaging support

Binary packaging support tool for distroless / alpine.

## Usage

```Dockerfile
FROM debian:12 AS builder
FROM --platform="${PLATFORM}" ${GOLANG} AS golang
COPY "./" "/build"
RUN cd "/build" \
 &&   go build -o "/usr/local/bin/dependency_resolve" . \
 && cd -
COPY --from=golang "/usr/local/bin/dependency_resolve" "/usr/local/bin/dependency_resolve"
RUN apt-get update && apt-get install -y "binutils" "php"
RUN /usr/local/bin/dependency_resolve "$(which "php")" | xargs -I {} sh -c 'mkdir -p /rootfs/$(dirname "{}") && cp -apP "{}" "/rootfs/{}" && (strip --strip-all "/rootfs/{}" || true)'

FROM gcr.io/distroless/base-nossl-debian12:latest
COPY --from=builder "/rootfs" "/"

ENTRYPOINT ["/usr/bin/php"]
```

See `Dockerfile` for more details.
