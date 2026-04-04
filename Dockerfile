FROM golang:1.25-alpine AS builder

# 设置环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 设置工作目录
WORKDIR /build

# 先拷贝 go.mod 和 go.sum 能够利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源代码
COPY . .

# 编译（假设你的 main.go 在根目录，生成的程序叫 app）
RUN go build -ldflags="-s -w" -o app .

# --- 第二阶段：运行 ---
FROM alpine:latest

WORKDIR /root/

# 从编译阶段拷贝二进制文件
COPY --from=builder /build/app .
# 如果有配置文件，也要拷贝过去
# COPY --from=builder /build/config.json ./config.json

# 赋予执行权限
RUN chmod +x app

# 运行程序
ENTRYPOINT ["./app"]