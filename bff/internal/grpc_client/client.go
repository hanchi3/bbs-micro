package grpc_client

import (
	// 根据你的 proto 文件调整包路径

	"bluebell_microservices/proto/comment"
	"bluebell_microservices/proto/post"
	"bluebell_microservices/proto/user"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Clients 结构体可以扩展以支持其他服务（例如 Post、Vote 等）
type Clients struct {
	// 接口，由 Protobuf 文件（user.proto）生成
	User                            user.UserServiceClient
	Post                            post.PostServiceClient
	Comment                         comment.CommentServiceClient
	userConn, postConn, commentConn *grpc.ClientConn // 保存连接以便关闭
}

// NewClients 初始化 gRPC 客户端
func NewClients(etcdEndpoints []string) (*Clients, error) {
	// 初始化 etcd 客户端
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndpoints, // 例如 []string{"localhost:2379"}
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 etcd 客户端失败: %v", err)
	}

	// 注册 etcd resolver
	rb := NewEtcdResolverBuilder(etcdClient)
	RegisterEtcdResolver(rb)

	// 服务名称 - 确保没有尾部斜杠
	userServiceName := "etcd://user"
	postServiceName := "etcd://post"
	commentServiceName := "etcd://comment"

	// 连接用户服务（非阻塞）
	fmt.Println("正在初始化微服务连接...")
	userConn, err := grpc.Dial(
		userServiceName,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("连接初始化失败: %v\n", err)
	}

	// 连接帖子服务（非阻塞）
	postConn, err := grpc.Dial(
		postServiceName,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("连接初始化失败: %v\n", err)
	}

	// 连接评论服务（非阻塞）
	commentConn, err := grpc.Dial(
		commentServiceName,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("连接初始化失败: %v\n", err)
	}

	fmt.Println("微服务客户端初始化完成")

	// 返回客户端，即使某些服务未连接
	clients := &Clients{
		User:        user.NewUserServiceClient(userConn),
		Post:        post.NewPostServiceClient(postConn),
		Comment:     comment.NewCommentServiceClient(commentConn),
		userConn:    userConn,
		postConn:    postConn,
		commentConn: commentConn,
	}
	return clients, nil
}

// Close 关闭所有连接
func (c *Clients) Close() {
	if c.userConn != nil {
		c.userConn.Close()
	}
	if c.postConn != nil {
		c.postConn.Close()
	}
	if c.commentConn != nil {
		c.commentConn.Close()
	}
}

// func NewClients() (*Clients, error) {
// 	fmt.Printf("正在连接用户服务(localhost:8081)...\n")
// 	userConn, err := grpc.Dial("localhost:8081",
// 		grpc.WithInsecure(),
// 		grpc.WithBlock(),                // 添加阻塞选项，确保连接建立
// 		grpc.WithTimeout(5*time.Second), // 添加超时控制
// 	)
// 	if err == nil {
// 		fmt.Printf("用户服务连接成功\n")
// 	}

// 	// 连接帖子服务（8082端口）
// 	postConn, err := grpc.Dial("localhost:8082",
// 		grpc.WithInsecure(),
// 		grpc.WithBlock(),
// 		grpc.WithTimeout(5*time.Second),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("连接帖子服务失败: %v", err)
// 	}

// 	// 连接评论服务（8083端口）
// 	commentConn, err := grpc.Dial("localhost:8083",
// 		grpc.WithInsecure(),
// 		grpc.WithBlock(),
// 		grpc.WithTimeout(5*time.Second),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("连接评论服务失败: %v", err)
// 	}

// 	return &Clients{
// 		User:    user.NewUserServiceClient(userConn),
// 		Post:    post.NewPostServiceClient(postConn),
// 		Comment: comment.NewCommentServiceClient(commentConn),
// 	}, nil
// }
