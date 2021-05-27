FROM golang:alpine AS builder
WORKDIR /go/src/github.com/go-k8s-metadata
COPY . .
RUN cd cmd \
    && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o  go-k8s-metadata .

FROM alpine
COPY --from=builder /go/src/github.com/go-k8s-metadata/cmd/go-k8s-metadata /bin/go-k8s-metadata

ENTRYPOINT ["/bin/go-k8s-metadata"]
