FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
RUN apk --no-cache add \
  bash

WORKDIR /src
COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH VERSION=$VERSION ./scripts/build.sh

# Alpine, not distroless as upstream Saremox uses. This fork gives the Redis
# pods an init container that runs "cp" from this image to put the
# redis-instance binary on a shared volume. A distroless image has no cp, so
# the Redis pods would not start.
FROM alpine:latest
RUN apk --no-cache add \
  ca-certificates
COPY --from=build /src/bin/redis-operator /usr/local/bin
COPY --from=build /src/bin/redis-instance /usr/local/bin
RUN addgroup -g 1000 rf && \
  adduser -D -u 1000 -G rf rf && \
  chown rf:rf /usr/local/bin/redis-operator /usr/local/bin/redis-instance
USER rf

ENTRYPOINT ["/usr/local/bin/redis-operator"]
