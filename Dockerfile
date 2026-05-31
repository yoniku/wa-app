FROM docker.m.daocloud.io/library/node:22-bookworm-slim AS dashboard_remote_builder

WORKDIR /wa-app/webui
COPY common-lib/ui /common-lib/ui
COPY wa-app/webui ./
RUN npm ci && SOURCE_ROOT=/ npm run build

FROM docker.m.daocloud.io/library/golang:1.26-alpine AS builder

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates

COPY common-lib /common-lib
COPY wa-app/go.mod wa-app/go.sum ./
RUN go mod edit -replace github.com/byte-v-forge/common-lib=/common-lib \
    && go mod download

COPY wa-app .
RUN CGO_ENABLED=0 GOOS=linux go build -o wa-app-service ./cmd/wa-app-service

FROM docker.m.daocloud.io/library/alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/wa-app-service .
COPY --from=dashboard_remote_builder /wa-app/webui/dist /app/dashboard/wa
EXPOSE 50051 8080
CMD ["./wa-app-service"]
