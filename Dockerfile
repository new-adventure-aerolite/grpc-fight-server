FROM golang:1.16.2 AS builder

WORKDIR /go/src
COPY . .
RUN go env -w GOPROXY=https://goproxy.cn,direct
# RUN go mod tidy && go mod vendor
RUN CGO_ENABLED=0 GOOS=linux go build -o fight-server

FROM alpine:latest
COPY --from=builder /go/src/config/config.json /etc/config/config.json
COPY --from=builder /go/src/fight-server /usr/local/bin/fight-server
ENTRYPOINT ["/usr/local/bin/fight-server","-config", "/etc/config/config.json"]
