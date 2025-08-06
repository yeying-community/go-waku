package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/waku-org/go-waku/waku/v2/node"
)

const privKeyFile = "wakunode.priv"

// 生成或加载 *ecdsa.PrivateKey
func loadOrCreateECDSAPrivKey() (*ecdsa.PrivateKey, error) {
	// 如果存在则加载
	if _, err := os.Stat(privKeyFile); err == nil {
		data, err := os.ReadFile(privKeyFile)
		if err != nil {
			return nil, err
		}
		libp2pPriv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, err
		}
		stdKey, err := crypto.PrivKeyToStdKey(libp2pPriv)
		if err != nil {
			return nil, err
		}
		ecdsaPriv, ok := stdKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an ECDSA private key")
		}
		return ecdsaPriv, nil
	}

	// 否则生成新私钥
	libp2pPriv, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		return nil, err
	}
	data, err := crypto.MarshalPrivateKey(libp2pPriv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(privKeyFile, data, 0600); err != nil {
		return nil, err
	}
	stdKey, err := crypto.PrivKeyToStdKey(libp2pPriv)
	if err != nil {
		return nil, err
	}
	ecdsaPriv, ok := stdKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA private key")
	}
	return ecdsaPriv, nil
}

// 获取公网IP
func getPublicIP() string {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "UNKNOWN"
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	return string(ip)
}

func main() {
	// 1. 加载或生成 *ecdsa.PrivateKey
	ecdsaPrivKey, err := loadOrCreateECDSAPrivKey()
	if err != nil {
		log.Fatal("Failed to load or create private key:", err)
	}

	// 2. 配置监听地址
	port := 60000
	hostAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatal("Failed to resolve address:", err)
	}

	ctx := context.Background()

	// 3. 创建 Waku 节点，带 *ecdsa.PrivateKey
	wakuNode, err := node.New(
		node.WithHostAddress(hostAddr),
		node.WithWakuRelay(),
		node.WithPrivateKey(ecdsaPrivKey),
	)
	if err != nil {
		log.Fatal("Failed to create node:", err)
	}

	// 4. 启动节点
	if err := wakuNode.Start(ctx); err != nil {
		log.Fatal("Failed to start node:", err)
	}

	// 5. 打印节点信息
	publicIP := getPublicIP()
	fmt.Println("=== Bootstrap Node Started ===")
	fmt.Printf("PeerID: %s\n", wakuNode.ID())
	fmt.Printf("公网 multiaddr:\n")
	fmt.Printf("/ip4/%s/tcp/%d/p2p/%s\n", publicIP, port, wakuNode.ID())
	fmt.Println("===============================")

	// 6. 优雅关闭
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down bootstrap node...")
	wakuNode.Stop()
}
