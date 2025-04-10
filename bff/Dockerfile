FROM golang:1.24-alpine

WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache gcc musl-dev

# 复制go.mod和go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN go build -o main ./bff/cmd

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./main"] 