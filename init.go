package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	RED    = "1"
	GREEN  = "2"
	ORANGE = "208"
	BLUE   = "39"
)

var (
	title   = fmtStr("●", BLUE, true)
	ask     = fmtStr("-", "", true)
	info    = fmtStr("+", "", true)
	warning = fmtStr("警告", RED, true)
)

func checkAndroid() {
	path := os.Getenv("PATH")
	if runtime.GOOS == "android" || strings.Contains(path, "com.termux") {
		prefix := os.Getenv("PREFIX")
		certPath := filepath.Join(prefix, "etc/tls/cert.pem")
		if err := os.Setenv("SSL_CERT_FILE", certPath); err != nil {
			failMessage("设置 Termux 证书文件失败。")
			log.Fatalln(err)
		}
		isAndroid = true
	}
}

func setDNS() {
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					conn, err := net.Dial("udp", "8.8.8.8:53")
					if err != nil {
						failMessage("DNS 连接失败。请断开 VPN 连接后重试...")
						log.Fatal(err)
					}
					return conn, nil
				},
			},
		}
		conn, err := d.DialContext(ctx, network, addr)
		if err != nil {
			failMessage("DNS 解析失败。请断开 VPN 连接后重试...")
			log.Fatal(err)
		}
		return conn, nil
	}

}

func renderHeader() {
	fmt.Printf(`
■■■■■■■  ■■■■■■■  ■■■■■■■ 
■■   ■■  ■■   ■■  ■■   ■■
■■■■■■■  ■■■■■■■  ■■■■■■■ 
■■   ■■  ■■       ■■   ■■
■■■■■■■  ■■       ■■■■■■■  %s %s
`,
		fmtStr("向导", GREEN, true),
		fmtStr(VERSION, GREEN, false),
	)
}

func initPaths() {
	var err error
	srcPath, err = os.MkdirTemp("", ".bpb-wizard")
	if err != nil {
		failMessage("创建临时目录失败。")
		log.Fatalln(err)
	}

	workerPath = filepath.Join(srcPath, "worker.js")
	cachePath = filepath.Join(srcPath, "tld.cache")
}

func fmtStr(str string, color string, isBold bool) string {
	style := lipgloss.NewStyle().Bold(isBold)

	if color != "" {
		style = style.Foreground(lipgloss.Color(color))
	}

	return style.Render(str)
}
