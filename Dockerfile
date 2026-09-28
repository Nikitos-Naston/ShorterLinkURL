ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/shortener ./cmd/linkShrinks

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/shortener /shortener
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/shortener"]
