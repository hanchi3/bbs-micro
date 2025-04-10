FROM golang:1.24-alpine

WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache gcc musl-dev

# 复制整个项目
COPY . .

# 下载依赖
RUN go mod download

# ARG用于在构建时传入参数
ARG SERVICE_NAME
ARG SERVICE_PORT

# 编译指定服务
RUN go build -o main ./${SERVICE_NAME}/cmd/server

# 暴露端口
EXPOSE ${SERVICE_PORT}

# 运行应用
CMD ["./main"] 