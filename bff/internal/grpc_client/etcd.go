package grpc_client

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

// EtcdResolverBuilder 实现 gRPC 的 Resolver 接口
type EtcdResolverBuilder struct {
	etcdClient *clientv3.Client
}

// NewEtcdResolverBuilder 创建一个新的EtcdResolverBuilder实例
func NewEtcdResolverBuilder(etcdClient *clientv3.Client) *EtcdResolverBuilder {
	return &EtcdResolverBuilder{
		etcdClient: etcdClient,
	}
}

// Scheme 返回 resolver 的 scheme
func (e *EtcdResolverBuilder) Scheme() string {
	return "etcd"
}

// Build 构建 resolver
func (e *EtcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &etcdResolver{
		etcdClient: e.etcdClient,
		cc:         cc,
		target:     target,
	}
	go r.watch()   // 启动监听 etcd 变化
	r.resolveNow() // 立即解析一次

	// 定期检查服务状态，但不输出日志
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			r.resolveNow() // 定期检查但无输出
		}
	}()

	return r, nil
}

type etcdResolver struct {
	etcdClient *clientv3.Client
	cc         resolver.ClientConn
	target     resolver.Target
}

func (r *etcdResolver) ResolveNow(options resolver.ResolveNowOptions) {
	r.resolveNow()
}

func (r *etcdResolver) resolveNow() {
	// 从 etcd 获取服务地址
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用target.URL.Host作为服务名
	serviceName := r.target.URL.Host
	key := fmt.Sprintf("/services/%s", serviceName)
	resp, err := r.etcdClient.Get(ctx, key)
	if err != nil {
		// 只在出错时打印
		fmt.Printf("获取服务 %s 地址失败: %v\n", serviceName, err)
		// 不立即返回，等待服务上线
	}

	var addrs []resolver.Address
	if resp != nil && len(resp.Kvs) > 0 {
		for _, kv := range resp.Kvs {
			addrs = append(addrs, resolver.Address{Addr: string(kv.Value)})
		}
	}

	// 更新 gRPC 连接地址
	if len(addrs) > 0 {
		r.cc.UpdateState(resolver.State{Addresses: addrs})
	}
}

func (r *etcdResolver) watch() {
	// 使用target.URL.Host作为服务名
	serviceName := r.target.URL.Host
	key := fmt.Sprintf("/services/%s", serviceName)
	fmt.Printf("等待服务 %s 上线...\n", serviceName)

	watchChan := r.etcdClient.Watch(context.Background(), key, clientv3.WithPrefix())
	for wresp := range watchChan {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case clientv3.EventTypePut:
				fmt.Printf("服务 %s 上线: %s\n", serviceName, ev.Kv.Value)
				r.resolveNow()
			case clientv3.EventTypeDelete:
				fmt.Printf("服务 %s 下线\n", serviceName)
				r.resolveNow()
			}
		}
	}
	fmt.Println("监听结束") // 如果退出，打印原因
}

func (r *etcdResolver) Close() {
	// 可选：关闭资源
}

// RegisterEtcdResolver 注册etcd解析器
func RegisterEtcdResolver(rb *EtcdResolverBuilder) {
	resolver.Register(rb)
}
