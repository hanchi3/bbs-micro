## 使用方法

### 启动中间件服务

``` bash
docker-compose -f docker-compose-middleware.yml up -d
```

### 可以按需启动各个微服务，例如：

``` bash
# 启动所有微服务
docker-compose -f docker-compose-services.yml up -d

# 或者单独启动某个服务
docker-compose -f docker-compose-services.yml up -d user-service
docker-compose -f docker-compose-services.yml up -d post-service
docker-compose -f docker-compose-services.yml up -d comment-service
docker-compose -f docker-compose-services.yml up -d bff-service
```