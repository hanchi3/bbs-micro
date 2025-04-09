// user-service/cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"bluebell_microservices/common/config"
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/common/pkg/snowflake"
	pb "bluebell_microservices/proto/user"
	"bluebell_microservices/user-service/internal/controller"
	"bluebell_microservices/user-service/internal/dao/mysql"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	// 添加这行
	"google.golang.org/grpc/reflection"
)

type UserServer struct {
	pb.UnimplementedUserServiceServer
}

func main() {
	flag.Parse()

	// 初始化日志
	if err := logger.Init("info", "user-service.log"); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer logger.Logger.Sync() // 确保日志在程序退出时写入

	// 初始化配置
	config.InitConfig()

	// 初始化雪花算法
	if err := snowflake.Init(1); err != nil {
		log.Fatalf("init snowflake failed, err:%v\n", err)
	}

	// 初始化数据库连接
	if err := mysql.Init(config.Conf.MySQL); err != nil {
		log.Fatalf("init mysql failed, err:%v\n", err)
	}
	defer mysql.Close()

	// 初始化 etcd 客户端
	etcdEndpoints := []string{"localhost:2379"}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("连接 etcd 失败: %v", err)
	}
	defer cli.Close() // 服务退出时关闭

	// 服务注册
	if err := registerService(cli, "user", "localhost:8081"); err != nil {
		logger.Error("Failed to register service", zap.Error(err))
		log.Fatalf("failed to register service: %v", err)
	}

	// 监听端口
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		logger.Error("Failed to listen", zap.Error(err))
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// 创建 gRPC 服务器
	s := grpc.NewServer()

	// 注册微服务
	pb.RegisterUserServiceServer(s, controller.NewUserController()) // 在这里，实现proto文件中定义的接口 UnimplementedUserServiceServer

	// 注册反射服务
	reflection.Register(s) // 添加这行

	logger.Info("User service running", zap.String("addr", ":8081"))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func registerService(cli *clientv3.Client, serviceName, address string) error {
	// 创建租约
	leaseResp, err := cli.Grant(context.Background(), 10)
	if err != nil {
		return fmt.Errorf("创建租约失败: %v", err)
	}

	// 注册服务
	key := fmt.Sprintf("/services/%s", serviceName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = cli.Put(ctx, key, address, clientv3.WithLease(leaseResp.ID))
	cancel()
	if err != nil {
		return fmt.Errorf("注册服务失败: %v", err)
	}
	fmt.Printf("服务 %s 注册成功，地址: %s\n", serviceName, address)

	// 续约
	keepAliveChan, err := cli.KeepAlive(context.Background(), leaseResp.ID)
	if err != nil {
		return fmt.Errorf("续约失败: %v", err)
	}
	go func() {
		for range keepAliveChan {
		}
		fmt.Println("续约结束，租约已失效")
	}()

	return nil
}
