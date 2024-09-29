FROM golang:1.23 AS builder
ADD . /src
WORKDIR /src
ENV CGO_ENABLED 0
RUN go build -o main .

FROM scratch
WORKDIR /
COPY --from=builder /src/main /
COPY --from=builder /src/videos /videos
ENV PORT 8080
ENTRYPOINT ["/main"]
