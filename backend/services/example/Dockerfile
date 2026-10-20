FROM golang:1.27.3-trixie@sha256:1896654cfc5c2b5ad74c07f29cb36f5d2d1cbed42e3054070fbfe444e521e83c AS build

ENV CGO_ENABLED=0
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/example ./services/example/cmd/example

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/example /example

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/example"]
