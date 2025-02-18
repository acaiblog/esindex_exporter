# 使用官方 Golang 镜像作为构建环境
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 文件（如果有）
COPY go.mod go.sum ./

# 下载依赖项
RUN go mod download

# 复制项目文件
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -o esindex_exporter .

# 使用最小的 alpine 镜像作为运行时环境
FROM alpine:latest

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/esindex_exporter .

# 定义环境变量
ENV ES_URI="http://elastic:password@elasticsearch:9200"
ENV ES_INDEX_PREFIX="llmstudio-"
ENV QUERY_INTERVAL=10
ENV LISTEN_PORT=9184

# 暴露监听端口
EXPOSE ${LISTEN_PORT}

# 启动应用并从环境变量中读取参数
CMD ["./esindex_exporter"]