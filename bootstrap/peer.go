package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/multiformats/go-libp2p/core/protocol"
    "github.com/multiformats/go-multiaddr"
    "github.com/waku-org/go-waku/waku/v2/node"
    wakuprotocol "github.com/waku-org/go-waku/waku/v2/protocol"
    "github.com/waku-org/go-waku/waku/v2/protocol/pb"
    "github.com/waku-org/go-waku/waku/v2/protocol/relay"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: go run peer.go <bootstrap_multiaddr>")
    }

    bootstrapAddr := os.Args[1]

    // 配置节点
    hostAddr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
    if err != nil {
        log.Fatal("Failed to resolve address:", err)
    }

    ctx := context.Background()
    // 创建 Waku 节点
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

    fmt.Printf("Peer node started. PeerID: %s\n", wakuNode.ID())

    // 连接到引导节点
    addr, err := multiaddr.NewMultiaddr(bootstrapAddr)
    if err != nil {
        log.Fatal("Invalid bootstrap address:", err)
    }

    // 解析 peer info
    peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
    if err != nil {
        log.Fatal("Failed to parse peer info:", err)
    }

    // 连接到引导节点
    err = wakuNode.Host().Connect(ctx, *peerInfo)
    if err != nil {
        log.Fatal("Failed to connect to bootstrap:", err)
    }

    fmt.Println("Connected to bootstrap node")
    time.Sleep(2 * time.Second)

    // 订阅主题
    contentTopic := "/my-app/1/chat/proto"
    pubsubTopic := relay.DefaultWakuTopic

    // 创建 ContentTopicSet
    contentTopicSet := make(wakuprotocol.ContentTopicSet)
    contentTopicSet[contentTopic] = struct{}{}

    // 创建内容过滤器
    contentFilter := wakuprotocol.ContentFilter{
        PubsubTopic:   pubsubTopic,
        ContentTopics: contentTopicSet,
    }

    // 订阅消息
    subscriptions, err := wakuNode.Relay().Subscribe(ctx, contentFilter)
    if err != nil {
        log.Fatal("Failed to subscribe:", err)
    }

    if len(subscriptions) == 0 {
        log.Fatal("No subscriptions created")
    }

    subscription := subscriptions[0]
    fmt.Printf("Subscribed with ID: %d\n", subscription.ID)

    // 监听消息
    go func() {
        fmt.Println("Starting message listener...")
        for envelope := range subscription.Ch {
            if envelope.Message() != nil {
                fmt.Printf("📨 Received from %s: %s\n",
                    envelope.PubsubTopic(),
                    string(envelope.Message().Payload))
            }
        }
        fmt.Println("Message listener stopped")
    }()

    // 定期发送消息
    go func() {
        time.Sleep(3 * time.Second) // 等待连接稳定

        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()

        counter := 0
        for range ticker.C {
            counter++

            // 创建消息
            timestamp := time.Now().UnixNano()
            message := &pb.WakuMessage{
                Payload:      []byte(fmt.Sprintf("Hello from %s - message #%d", wakuNode.ID(), counter)),
                ContentTopic: contentTopic,
                Timestamp:    &timestamp,
            }

            // 发布消息
            _, err := wakuNode.Relay().Publish(ctx, message, relay.WithPubSubTopic(pubsubTopic))
            if err != nil {
                log.Printf("❌ Failed to publish: %v", err)
            } else {
                fmt.Printf("📤 Sent message #%d to topic %s\n", counter, pubsubTopic)
            }
        }
    }()

    // 显示连接的节点信息
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            peers := wakuNode.Host().Network().Peers()
            fmt.Printf("🔗 Connected to %d peers\n", len(peers))
            for _, p := range peers {
                fmt.Printf("   - %s\n", p)
            }
        }
    }()

    // 优雅关闭
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    <-c
    fmt.Println("\nShutting down peer node...")

    // 取消订阅
    if subscription.Unsubscribe != nil {
        subscription.Unsubscribe()
    }

    wakuNode.Stop()
}
