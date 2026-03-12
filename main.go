package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"BPB-Tools/task"
	"BPB-Tools/utils"
)

const version = "1.0.0"

// BPB-Wizard 版本（供 init.go 使用）
var VERSION = "dev"

// CloudflareSpeedTest 版本（测速工具版本）
var cfVersion = "2.4.0"

// BPB-Wizard 全局变量
var (
	srcPath    string
	workerPath string
	cachePath  string
	isAndroid  = false
	workerURL  = "https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js"
)

// Speedtest 更新检查（测速工具版本检查）
var versionNew string

// 嵌入资源文件
//
//go:embed ip.txt
var defaultIP []byte

//go:embed ipv6.txt
var defaultIPv6 []byte

// 初始化资源文件，如果不存在则释放
func initResources() {
	files := map[string][]byte{
		"ip.txt":   defaultIP,
		"ipv6.txt": defaultIPv6,
	}

	for name, data := range files {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			fmt.Printf("检测到资源文件 %s 丢失，正在自动恢复...\n", name)
			err := os.WriteFile(name, data, 0644)
			if err != nil {
				log.Printf("恢复文件 %s 失败: %v\n", name, err)
			} else {
				fmt.Printf("资源文件 %s 已成功恢复。\n", name)
			}
		}
	}
}

func main() {
	// 初始化资源文件
	initResources()

	// 如果有命令行参数，使用子命令模式
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "wizard":
			runWizardCmd(os.Args[2:])
		case "speedtest":
			runSpeedtestCmd(os.Args[2:])
		case "version", "-v", "--version":
			fmt.Printf("BPB-Tools 版本 %s\n", version)
			fmt.Println("包含工具:")
			fmt.Println("  - wizard: BPB 配置向导（Web 界面）")
			fmt.Println("  - speedtest: Cloudflare IP 测速工具")
		case "help", "-h", "--help":
			printUsage()
		default:
			fmt.Printf("未知命令：%s\n\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
		return
	}

	// 没有参数时，显示交互式菜单
	showInteractiveMenu()
}

func showInteractiveMenu() {
	for {
		fmt.Println()
		fmt.Println("===============================")
		fmt.Println("    BPB-Tools v" + version + "    ")
		fmt.Println("===============================")
		fmt.Println("1. 运行 CloudflareSpeedTest (测速)")
		fmt.Println("2. 运行 BPB-Wizard (部署代理面板)")
		fmt.Println("3. 懒人一键部署")
		fmt.Println("q. 退出程序")
		fmt.Println("===============================")
		fmt.Print("请选择功能 [1/2/3/q]: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			fmt.Println("\n启动 CloudflareSpeedTest...")
			runSpeedtestCmd([]string{})
		case "2":
			fmt.Println("\n启动 BPB-Wizard...")
			runWizardCmd([]string{})
		case "3":
			runLazyDeploy()
		case "q", "Q":
			fmt.Println("再见！")
			return
		default:
			fmt.Println("无效选择，请重新输入。")
		}
	}
}

func printUsage() {
	fmt.Printf("BPB-Tools v%s - 多功能工具箱\n\n", version)
	fmt.Println("使用方法:")
	fmt.Println("  bpb-tools <command> [options]")
	fmt.Println("")
	fmt.Println("可用命令:")
	fmt.Println("  wizard      启动 BPB 配置向导（Web 界面）")
	fmt.Println("  speedtest   启动 Cloudflare IP 测速")
	fmt.Println("  version     显示版本信息")
	fmt.Println("  help        显示帮助信息")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  bpb-tools wizard              # 启动配置向导")
	fmt.Println("  bpb-tools speedtest -n 200    # 延迟测速，200 线程")
	fmt.Println("  bpb-tools version             # 查看版本")
}

// ==================== Wizard 命令 ====================

func runWizardCmd(args []string) {
	wizardCmd := flag.NewFlagSet("wizard", flag.ExitOnError)
	showVersion := wizardCmd.Bool("version", false, "显示版本")
	wizardCmd.Parse(args)

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	initPaths()
	setDNS()
	checkAndroid()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		runWizard()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", callback)
	server := &http.Server{
		Addr:    ":8976",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 不要直接使用 log.Fatalln，以免杀掉主进程
			log.Printf("本地服务启动警告: %v\n", err)
		}
	}()

	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭异常：%v", err)
	}

	fmt.Println("\nBPB-Wizard 已退出，返回主菜单。")
}

// ==================== Speedtest 命令 ====================

func runSpeedtestCmd(args []string) {
	speedtestCmd := flag.NewFlagSet("speedtest", flag.ExitOnError)

	var (
		printVersion                     bool
		minDelay, maxDelay, downloadTime int
		maxLossRate                      float64
		help                             = `
CloudflareSpeedTest ` + cfVersion + `
测试各个 CDN 或网站所有 IP 的延迟和速度，获取最快 IP (IPv4+IPv6)！
https://github.com/XIU2/CloudflareSpeedTest

参数：
    -n 200
        延迟测速线程；越多延迟测速越快，性能弱的设备 (如路由器) 请勿太高；(默认 200 最多 1000)
    -t 4
        延迟测速次数；单个 IP 延迟测速的次数；(默认 4 次)
    -dn 10
        下载测速数量；延迟测速并排序后，从最低延迟起下载测速的数量；(默认 10 个)
    -dt 10
        下载测速时间；单个 IP 下载测速最长时间，不能太短；(默认 10 秒)
    -tp 443
        指定测速端口；延迟测速/下载测速时使用的端口；(默认 443 端口)
    -url https://cf.xiu2.xyz/url
        指定测速地址；延迟测速 (HTTPing)/下载测速时使用的地址，默认地址不保证可用性，建议自建；

    -httping
        切换测速模式；延迟测速模式改为 HTTP 协议，所用测试地址为 [-url] 参数；(默认 TCPing)
    -httping-code 200
        有效状态代码；HTTPing 延迟测速时网页返回的有效 HTTP 状态码，仅限一个；(默认 200 301 302)
    -cfcolo HKG,KHH,NRT,LAX,SEA,SJC,FRA,MAD
        匹配指定地区；IATA 机场地区码或国家/城市码，英文逗号分隔，仅 HTTPing 模式可用；(默认 所有地区)

    -tl 200
        平均延迟上限；只输出低于指定平均延迟的 IP，各上下限条件可搭配使用；(默认 9999 ms)
    -tll 40
        平均延迟下限；只输出高于指定平均延迟的 IP；(默认 0 ms)
    -tlr 0.2
        丢包几率上限；只输出低于/等于指定丢包率的 IP，范围 0.00~1.00，0 过滤掉任何丢包的 IP；(默认 1.00)
    -sl 5
        下载速度下限；只输出高于指定下载速度的 IP，凑够指定数量 [-dn] 才会停止测速；(默认 0.00 MB/s)

    -p 10
        显示结果数量；测速后直接显示指定数量的结果，为 0 时不显示结果直接退出；(默认 10 个)
    -f ip.txt
        IP 段数据文件；如路径含有空格请加上引号；支持其他 CDN IP 段；(默认 ip.txt)
    -ip 1.1.1.1,2.2.2.2/24,2606:4700::/32
        指定 IP 段数据；直接通过参数指定要测速的 IP 段数据，英文逗号分隔；(默认 空)
    -o result.csv
        写入结果文件；如路径含有空格请加上引号；值为空时不写入文件 [-o ""]；(默认 result.csv)

    -dd
        禁用下载测速；禁用后测速结果会按延迟排序 (默认按下载速度排序)；(默认 启用)
    -allip
        测速全部的 IP；对 IP 段中的每个 IP (仅支持 IPv4) 进行测速；(默认 每个 /24 段随机测速一个 IP)

    -debug
        调试输出模式；会在一些非预期情况下输出更多日志以便判断原因；(默认 关闭)

    -v
        打印程序版本 + 检查版本更新
    -h
        打印帮助说明
`
	)

	speedtestCmd.IntVar(&task.Routines, "n", 200, "延迟测速线程")
	speedtestCmd.IntVar(&task.PingTimes, "t", 4, "延迟测速次数")
	speedtestCmd.IntVar(&task.TestCount, "dn", 10, "下载测速数量")
	speedtestCmd.IntVar(&downloadTime, "dt", 10, "下载测速时间")
	speedtestCmd.IntVar(&task.TCPPort, "tp", 443, "指定测速端口")
	speedtestCmd.StringVar(&task.URL, "url", "https://cf.xiu2.xyz/url", "指定测速地址")

	speedtestCmd.BoolVar(&task.Httping, "httping", false, "切换测速模式")
	speedtestCmd.IntVar(&task.HttpingStatusCode, "httping-code", 0, "有效状态代码")
	speedtestCmd.StringVar(&task.HttpingCFColo, "cfcolo", "", "匹配指定地区")

	speedtestCmd.IntVar(&maxDelay, "tl", 9999, "平均延迟上限")
	speedtestCmd.IntVar(&minDelay, "tll", 0, "平均延迟下限")
	speedtestCmd.Float64Var(&maxLossRate, "tlr", 1, "丢包几率上限")
	speedtestCmd.Float64Var(&task.MinSpeed, "sl", 0, "下载速度下限")

	speedtestCmd.IntVar(&utils.PrintNum, "p", 10, "显示结果数量")
	speedtestCmd.StringVar(&task.IPFile, "f", "ip.txt", "IP 段数据文件")
	speedtestCmd.StringVar(&task.IPText, "ip", "", "指定 IP 段数据")
	speedtestCmd.StringVar(&utils.Output, "o", "result.csv", "输出结果文件")

	speedtestCmd.BoolVar(&task.Disable, "dd", false, "禁用下载测速")
	speedtestCmd.BoolVar(&task.TestAll, "allip", false, "测速全部 IP")

	speedtestCmd.BoolVar(&utils.Debug, "debug", false, "调试输出模式")
	speedtestCmd.BoolVar(&printVersion, "v", false, "打印程序版本")
	speedtestCmd.Usage = func() { fmt.Print(help) }
	speedtestCmd.Parse(args)

	if printVersion {
		println(cfVersion)
		fmt.Println("检查版本更新中...")
		checkUpdate()
		if versionNew != "" {
			utils.Yellow.Printf("*** 发现新版本 [%s]！请前往 [https://github.com/XIU2/CloudflareSpeedTest] 更新！ ***", versionNew)
		} else {
			utils.Green.Println("当前为最新版本 [" + cfVersion + "]！")
		}
		os.Exit(0)
	}

	if task.MinSpeed > 0 && time.Duration(maxDelay)*time.Millisecond == utils.InputMaxDelay {
		utils.Yellow.Println("[提示] 在使用 [-sl] 参数时，建议搭配 [-tl] 参数，以避免因凑不够 [-dn] 数量而一直测速...")
	}
	utils.InputMaxDelay = time.Duration(maxDelay) * time.Millisecond
	utils.InputMinDelay = time.Duration(minDelay) * time.Millisecond
	utils.InputMaxLossRate = float32(maxLossRate)
	task.Timeout = time.Duration(downloadTime) * time.Second
	task.HttpingCFColomap = task.MapColoMap()

	// 开始测速
	task.InitRandSeed()

	fmt.Printf("# XIU2/CloudflareSpeedTest %s \n\n", cfVersion)

	// 开始延迟测速 + 过滤延迟/丢包
	pingData := task.NewPing().Run().FilterDelay().FilterLossRate()
	// 开始下载测速
	speedData := task.TestDownloadSpeed(pingData)
	utils.ExportCsv(speedData) // 输出文件
	speedData.Print()          // 打印结果
	endPrint()                 // 根据情况选择退出方式（针对 Windows）
}

// 根据情况选择退出方式（针对 Windows）
func endPrint() {
	if utils.NoPrintResult() {
		return
	}
	// 自动模式下跳过回车等待
	if autoMode {
		return
	}
	if runtime.GOOS == "windows" {
		fmt.Printf("按下 回车键 或 Ctrl+C 退出。")
		fmt.Scanln()
	}
}

// 检查更新
func checkUpdate() {
	timeout := 10 * time.Second
	client := http.Client{Timeout: timeout}
	res, err := client.Get("https://api.xiu2.xyz/ver/cloudflarespeedtest.txt")
	if err != nil {
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if string(body) != cfVersion {
		versionNew = string(body)
	}
}

// ==================== 懒人一键部署 ====================

func runLazyDeploy() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("runLazyDeploy panic: %v", r)
		}
	}()

	fmt.Println("\n===============================")
	fmt.Println("    懒人一键部署")
	fmt.Println("===============================")
	fmt.Println("\n为了更准确地测量延迟和网速，请在开始之前：")
	fmt.Println("  1. 关闭占用网络的程序（如视频、下载工具等）")
	fmt.Println("  2. 关闭代理程序")
	fmt.Println("\n按回车键开始执行...")
	fmt.Scanln()

	// 设置自动模式标志
	autoMode = true

	// 步骤 1: 运行 CloudflareSpeedTest
	fmt.Println("\n[1/2] 开始执行 CloudflareSpeedTest 测速...")
	fmt.Println("提示：测速过程可能需要几分钟，请耐心等待。")
	runSpeedtestCmd([]string{})

	// 步骤 2: 运行 BPB-Wizard（自动模式）
	fmt.Println("\n[2/2] 开始自动部署 BPB 面板...")
	fmt.Println("提示：自动模式下所有选项将使用默认值。")

	// 初始化必要的环境
	initPaths()
	setDNS()
	checkAndroid()

	// 启动 HTTP 服务器
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		runWizard()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", callback)
	server := &http.Server{
		Addr:    ":8976",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("本地服务启动警告: %v\n", err)
		}
	}()

	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭异常：%v", err)
	}

	fmt.Println("\n懒人一键部署完成！")
}
