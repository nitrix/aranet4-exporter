FROM golang:alpine AS builder

WORKDIR /opt
COPY . /opt

RUN apk --no-cache add ca-certificates
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o aranet4-exporter

FROM scratch AS release

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /opt /opt

WORKDIR /opt
CMD ["/opt/aranet4-exporter"]