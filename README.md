# dependency_resolve - distroless packaging support

Binary packaging support tool for distroless / alpine.

## Usage

```Dockerfile
FROM debian:12 AS builder
COPY --chmod=755 "dependency_resolve" "/usr/local/bin/dependency_resolve"
RUN apt-get update && apt-get install -y "php"
RUN dependency_resolve "$(which "ldd")" "$(which "php")" | xargs -I {} sh -c 'mkdir -p /root/rootfs/$(dirname "{}") && cp -apP "{}" "/root/rootfs/{}"'

FROM gcr.io/distroless/base-nossl-debian12:latest
COPY --from=builder "/root/rootfs" "/"

ENTRYPOINT ["/usr/bin/php"]
```

See `Dockerfile` for more details.
