FROM golang:1.22 AS builder
ADD . /src
WORKDIR /src
ENV CGO_ENABLED 0
RUN go build -o main .
RUN go install github.com/bitnami/wait-for-port@v1.0.8

FROM busybox
WORKDIR /
COPY --from=builder /src/main /
COPY --from=builder /go/bin/wait-for-port /
ENV PORT 8080
CMD /wait-for-port --host rabbit --state inuse 5672 && \
    /main
