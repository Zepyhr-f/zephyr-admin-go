FROM docker.1ms.run/library/golang:alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED 0
ENV GOPROXY https://goproxy.cn,direct

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod edit -go=1.24.0 && go mod download

COPY . .

WORKDIR /build/app/auth
RUN go build -ldflags="-s -w" -o /app/auth auth.go

FROM docker.1ms.run/library/alpine:latest

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata
ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=builder /app/auth /app/auth
COPY --from=builder /build/app/auth/etc /app/etc

CMD ["./auth", "-f", "etc/auth-docker.yaml"]
