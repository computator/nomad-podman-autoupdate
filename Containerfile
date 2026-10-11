ARG GOVER=1.26
FROM docker.io/library/golang:${GOVER} AS build
WORKDIR /build

COPY go.* ./
RUN go mod download

COPY --from=ghcr.io/casey/just /just /usr/local/bin/
COPY . ./
RUN just build

FROM gcr.io/distroless/static-debian13

LABEL org.opencontainers.image.source=https://github.com/computator/nomad-podman-autoupdate

COPY --from=build /build/nomad-podman-autoupdate /usr/local/bin/
CMD ["nomad-podman-autoupdate"]
