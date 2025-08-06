package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    "github.com/waku-org/go-waku/waku/v2/node"
)

func main() {
    // 配置引导节点监听地址
    hostAddr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:60000")
    if err != nil {
        log.Fatal("Failed to resolve address:", err)
    }

    ctx := context.Background()

    // 创建 Waku 节点 - 不指定私钥，让它自动生成
    wakuNode, err := node.New(
        node.WithHostAddress(hostAddr),
        node.WithWakuRelay(),
    )

    if err != nil {
        log.Fatal("Failed to create node:", err)
    }

    // 启动节点
    if err := wakuNode.Start(ctx); err != nil {
        log.Fatal("Failed to start node:", err)
    }

    // 打印节点信息
    fmt.Println("=== Bootstrap Node Started ===")
    fmt.Printf("PeerID: %s\n", wakuNode.ID())
    fmt.Printf("Listen addresses:\n")
    for _, addr := range wakuNode.ListenAddresses() {
        fmt.Printf("  Full address: %s\n", addr)
        // 打印完整的 multiaddr 供其他节点连接使用
        fmt.Printf("  node id: %s\n", wakuNode.ID())
    }
    fmt.Println("===============================")

    // 优雅关闭
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    <-c
    fmt.Println("\nShutting down bootstrap node...")
    wakuNode.Stop()
}

