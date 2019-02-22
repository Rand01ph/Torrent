ARG GO_VERSION=1.11

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add git
ENV GO111MODULE=on

# WORKDIR指令用于设置Dockerfile中的RUN、CMD和ENTRYPOINT指令执行命令的工作目录(默认为/目录)
# 该指令在Dockerfile文件中可以出现多次，如果使用相对路径则为相对于WORKDIR上一次的值
WORKDIR /src

# Fetch dependencies first; they are less susceptible to change on every build
# and will therefore be cached for speeding up the next build
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build \
    -o /app .

FROM scratch AS final
COPY --from=builder /app /app
COPY ./filebeat-input-log.tpl /filebeat-input-log.tpl
ENTRYPOINT ["/app"]
