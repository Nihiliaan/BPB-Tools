package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/kv"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/google/uuid"
)

type DeployType int

const (
	DTWorker DeployType = iota
	DTPage
)

// 自动模式标志（用于懒人一键部署）
var autoMode = false

var DeployTypeNames = map[DeployType]string{
	DTWorker: "worker",
	DTPage:   "page",
}

func (dt DeployType) String() string {
	return DeployTypeNames[dt]
}

type Panel struct {
	Name string
	Type string
}

const (
	CharsetAlphaNumeric      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	CharsetSpecialCharacters = "!@#$%^&*()_+[]{}|;:',.<>?"
	CharsetTrojanPassword    = CharsetAlphaNumeric + CharsetSpecialCharacters
	CharsetSubDomain         = "abcdefghijklmnopqrstuvwxyz0123456789-"
	CharsetURIPath           = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@$&*_-+;:,."
	DomainRegex              = `^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`
)

// worker.js 下载地址（多个备用源）
var workerURLs = []string{
	"https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js",
	"https://ghp.ci/https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js",
	"https://ghproxy.net/https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js",
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 worker.js 失败: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	finalContent := append(content, []byte(generateJunkCode())...)
	if err := os.WriteFile(dest, finalContent, 0644); err != nil {
		return err
	}

	return nil
}

func downloadWorker() error {
	fmt.Printf("\n%s 下载 %s...\n", title, fmtStr("worker.js", GREEN, true))

	for {
		if _, err := os.Stat(workerPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("检查 worker.js 失败：%w", err)
			}
		} else {
			successMessage("worker.js 已存在，跳过下载。")
			return nil
		}

		// 尝试所有下载源
		var lastErr error
		for i, url := range workerURLs {
			if autoMode && i > 0 {
				fmt.Printf("%s 主源下载失败，尝试备用源 %d...\n", info, i)
			}

			if err := downloadFile(url, workerPath); err != nil {
				lastErr = err
				log.Printf("下载源 %d 失败：%v\n", i+1, err)
				continue
			}

			successMessage("worker.js 下载成功！")
			return nil
		}

		failMessage("所有下载源均失败。\n")
		log.Printf("%v\n", lastErr)
		if !autoMode && promptUser("- 是否重试？(y/n): ", []string{"y", "n"}) == "n" {
			os.Exit(0)
		}
		if autoMode {
			time.Sleep(2 * time.Second)
		}
	}
}

func generateJunkCode() string {
	var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	minVars, maxVars := 50, 500
	minFuncs, maxFuncs := 50, 500

	varCount := rng.Intn(maxVars-minVars+1) + minVars
	funcCount := rng.Intn(maxFuncs-minFuncs+1) + minFuncs

	var sb strings.Builder

	for i := range varCount {
		varName := fmt.Sprintf("__var_%s_%d", generateRandomString(CharsetAlphaNumeric, 8, false), i)
		value := rng.Intn(100000)
		sb.WriteString(fmt.Sprintf("let %s = %d; ", varName, value))
	}

	for i := range funcCount {
		funcName := fmt.Sprintf("__Func_%s_%d", generateRandomString(CharsetAlphaNumeric, 8, false), i)
		ret := rng.Intn(1000)
		sb.WriteString(fmt.Sprintf("function %s() { return %d; } ", funcName, ret))
	}

	return sb.String()
}

func generateRandomString(charSet string, length int, isDomain bool) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomBytes := make([]byte, length)

	for i := range randomBytes {
		for {
			char := charSet[r.Intn(len(charSet))]
			if isDomain && (i == 0 || i == length-1) && char == byte('-') {
				continue
			}
			randomBytes[i] = char
			break
		}
	}

	return string(randomBytes)
}

func generateRandomSubDomain(subDomainLength int) string {
	return generateRandomString(CharsetSubDomain, subDomainLength, true)
}

func isValidSubDomain(subDomain string) error {
	if strings.Contains(subDomain, "bpb") {
		message := fmt.Sprintf("名称不能包含 %s。请重试。\n", fmtStr("bpb", RED, true))
		return fmt.Errorf("%s", message)
	}

	subdomainRegex := regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	isValid := subdomainRegex.MatchString(subDomain)
	if !isValid {
		message := fmt.Sprintf("子域名不能以 %s 开头，且只能包含 %s 和 %s。请重试。\n", fmtStr("-", RED, true), fmtStr("A-Z", GREEN, true), fmtStr("0-9", GREEN, true))
		return fmt.Errorf("%s", message)
	}
	return nil
}

func isValidIpDomain(value string) bool {
	if net.ParseIP(value) != nil && !strings.Contains(value, ":") {
		return true
	}

	if isValidIPv6(value) {
		return true
	}

	domainRegex := regexp.MustCompile(DomainRegex)
	return domainRegex.MatchString(value)
}

func isValidIPv6(value string) bool {
	regex := regexp.MustCompile(`^\[(.+)\]$`)
	matches := regex.FindStringSubmatch(value)
	return matches != nil && net.ParseIP(matches[1]) != nil
}

func isValidHost(value string) bool {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return false
	}

	if !isValidIpDomain(host) {
		return false
	}

	intPort, err := strconv.Atoi(port)
	if err != nil || intPort < 1 || intPort > 65535 {
		return false
	}

	return true
}

func generateTrPassword(passwordLength int) string {
	return generateRandomString(CharsetTrojanPassword, passwordLength, false)
}

func isValidTrPassword(trojanPassword string) bool {
	for _, c := range trojanPassword {
		if !strings.ContainsRune(CharsetTrojanPassword, c) {
			return false
		}
	}

	return true
}

func generateSubURIPath(uriLength int) string {
	return generateRandomString(CharsetURIPath, uriLength, false)
}

func isValidSubURIPath(uri string) bool {
	for _, c := range uri {
		if !strings.ContainsRune(CharsetURIPath, c) {
			return false
		}
	}

	return true
}

func promptUser(prompt string, answers []string) string {
	// 自动模式下直接返回默认值
	if autoMode {
		if answers != nil && len(answers) > 0 {
			// 对于有选项的问题，返回第一个选项（如 "y" 或 "1"）
			return answers[0]
		}
		// 对于输入问题，返回空字符串
		return ""
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n%s", prompt)
		input, err := reader.ReadString('\n')

		if err != nil {
			fmt.Printf("\n%s 正在退出...\n", title)
			if err == io.EOF {
				os.Exit(0)
			}
			os.Exit(1)
		}

		input = strings.TrimSpace(input)

		if answers == nil {
			return input
		} else {
			for _, ans := range answers {
				if strings.EqualFold(input, ans) {
					return input
				}
			}

			failMessage("无效回答，请重试。")
		}
	}
}

func failMessage(message string) {
	errMark := fmtStr("✗", RED, true)
	fmt.Printf("%s %s\n", errMark, message)
}

func successMessage(message string) {
	succMark := fmtStr("✓", GREEN, true)
	fmt.Printf("%s %s\n", succMark, message)
}

func openURL(url string) error {
	var cmd string
	var args = []string{url}

	switch runtime.GOOS {
	case "darwin": // MacOS
		cmd = "open"
	case "windows": // Windows
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // Linux, BSD, Android, etc.
		if isAndroid {
			termuxBin := os.Getenv("PATH")
			cmd = filepath.Join(termuxBin, "termux-open-url")
		} else {
			cmd = "xdg-open"
		}
	}

	err := exec.Command(cmd, args...).Start()
	if err != nil {
		return err
	}

	return nil
}

func checkBPBPanel(url string) error {
	message := fmt.Sprintf("BPB 面板已就绪 -> %s", fmtStr(url, BLUE, true))
	successMessage(message)

	// 自动模式下直接打开浏览器
	if autoMode {
		if err := openURL(url); err != nil {
			return err
		}
		return nil
	}

	prompt := fmt.Sprintf("- 是否在浏览器中打开 %s？(y/n): ", fmtStr("BPB 面板", BLUE, true))

	if response := promptUser(prompt, []string{"y", "n"}); strings.ToLower(response) == "n" {
		return nil
	}

	if err := openURL(url); err != nil {
		return err
	}

	return nil
	// }

	// return nil
}

func runWizard() {
	renderHeader()
	fmt.Printf("\n%s 欢迎使用 %s！\n", title, fmtStr("BPB 向导", GREEN, true))
	fmt.Printf("%s 本向导将帮助您在 Cloudflare 上部署或修改 %s。\n", info, fmtStr("BPB 面板", BLUE, true))
	fmt.Printf("%s 请确保您拥有经过验证的 %s 账户。\n", info, fmtStr("Cloudflare", ORANGE, true))

	// 自动模式下直接创建面板
	if autoMode {
		createPanel()
		return
	}

	for {
		message := fmt.Sprintf("1- %s 新面板。\n2- %s 现有面板。\n\n- 选择：", fmtStr("创建", GREEN, true), fmtStr("修改", RED, true))
		response := promptUser(message, []string{"1", "2"})
		switch response {
		case "1":
			createPanel()
		case "2":
			modifyPanel()
		}

		res := promptUser("- 是否再次运行向导？(y/n): ", []string{"y", "n"})
		if strings.ToLower(res) == "n" {
			fmt.Printf("\n%s 正在退出...\n", title)
			return
		}
	}
}

func createPanel() {
	ctx := context.Background()
	var err error
	if cfClient == nil || cfAccount == nil {
		go login()
		token := <-obtainedToken
		cfClient = NewClient(token)

		cfAccount, err = getAccount(ctx)
		if err != nil {
			failMessage("获取 Cloudflare 账户失败。")
			log.Fatalln(err)
		}
	}

	fmt.Printf("\n%s 获取设置...\n", title)
	fmt.Printf("\n%s 您可以使用 %s 或 %s 方式部署。\n", info, fmtStr("Workers", ORANGE, true), fmtStr("Pages", ORANGE, true))
	fmt.Printf("%s %s：如果您选择 %s，可能需要等待最多 5 分钟才能访问面板，请耐心等待！\n", info, warning, fmtStr("Pages", ORANGE, true))
	var deployType DeployType

	if autoMode {
		// 自动模式默认选择 Workers
		deployType = DTWorker
	} else {
		response := promptUser("1- Workers 方式。\n2- Pages 方式。\n\n- 选择：", []string{"1", "2"})
		switch response {
		case "1":
			deployType = DTWorker
		case "2":
			deployType = DTPage
		}
	}

	var projectName string
	for {
		projectName = generateRandomSubDomain(32)
		fmt.Printf("\n%s 随机生成的名称（%s）为：%s", info, fmtStr("子域名", GREEN, true), fmtStr(projectName, ORANGE, true))

		if !autoMode {
			if response := promptUser("- 请输入自定义名称或按回车键使用生成的名称：", nil); response != "" {
				if err := isValidSubDomain(response); err != nil {
					failMessage(err.Error())
					continue
				}

				projectName = response
			}
		}

		var isAvailable bool
		fmt.Printf("\n%s 检查域名可用性...\n", title)

		if deployType == DTWorker {
			isAvailable = isWorkerAvailable(ctx, projectName)
		} else {
			isAvailable = isPagesProjectAvailable(ctx, projectName)
		}

		if !isAvailable {
			prompt := fmt.Sprintf("- 该名称已存在！这将 %s 所有面板设置，是否继续覆盖？(y/n): ", fmtStr("重置", RED, true))
			if autoMode || strings.ToLower(promptUser(prompt, []string{"y", "n"})) == "n" {
				continue
			}
		}

		successMessage("可用！")
		break
	}

	uid := uuid.NewString()
	fmt.Printf("\n%s 随机生成的 %s 为：%s", info, fmtStr("UUID", GREEN, true), fmtStr(uid, ORANGE, true))
	if !autoMode {
		for {
			if response := promptUser("- 请输入自定义 uid 或按回车键使用生成的 uid：", nil); response != "" {
				if _, err := uuid.Parse(response); err != nil {
					failMessage("UUID 不符合标准，请重试。")
					continue
				}

				uid = response
			}

			break
		}
	}

	trPass := generateTrPassword(12)
	fmt.Printf("\n%s 随机生成的 %s 为：%s", info, fmtStr("Trojan 密码", GREEN, true), fmtStr(trPass, ORANGE, true))
	if !autoMode {
		for {
			if response := promptUser("- 请输入自定义 Trojan 密码或按回车键使用生成的密码：", nil); response != "" {
				if !isValidTrPassword(response) {
					failMessage("Trojan 密码不能包含非标准字符！请重试。")
					continue
				}

				trPass = response
			}

			break
		}
	}

	proxyIP := ""
	fmt.Printf("\n%s 默认 %s 为：%s", info, fmtStr("代理 IP", GREEN, true), fmtStr("bpb.yousef.isegaro.com", ORANGE, true))
	if !autoMode {
		for {
			if response := promptUser("- 请输入自定义代理 IP/域名或按回车键使用默认值：", nil); response != "" {
				areValid := true
				values := strings.SplitSeq(response, ",")
				for v := range values {
					trimmedValue := strings.TrimSpace(v)
					if !isValidIpDomain(trimmedValue) && !isValidHost(trimmedValue) {
						areValid = false
						message := fmt.Sprintf("%s 不是有效的 IP 或域名。请重试。", trimmedValue)
						failMessage(message)
					}
				}

				if !areValid {
					continue
				}

				proxyIP = response
			}

			break
		}
	}

	nat64Prefix := ""
	fmt.Printf("\n%s 默认 %s 在此处查看：%s", info, fmtStr("Nat64 前缀", GREEN, true), fmtStr("https://github.com/bia-pain-bache/BPB-Worker-Panel/blob/main/NAT64Prefixes.md", ORANGE, true))
	if !autoMode {
		for {
			if response := promptUser("- 请输入自定义 NAT64 前缀或按回车键使用默认值：", nil); response != "" {
				areValid := true
				values := strings.SplitSeq(response, ",")
				for v := range values {
					trimmedValue := strings.TrimSpace(v)
					if !isValidIPv6(trimmedValue) {
						areValid = false
						message := fmt.Sprintf("%s 不是有效的 IPv6 地址。请重试。", trimmedValue)
						failMessage(message)
					}
				}

				if !areValid {
					continue
				}

				nat64Prefix = response
			}

			break
		}
	}

	fallback := ""
	fmt.Printf("\n%s 默认 %s 为：%s", info, fmtStr("回退域名", GREEN, true), fmtStr("speed.cloudflare.com", ORANGE, true))
	if !autoMode {
		if response := promptUser("- 请输入自定义回退域名或按回车键使用默认值：", nil); response != "" {
			fallback = response
		}
	}

	subPath := generateSubURIPath(16)
	fmt.Printf("\n%s 随机生成的 %s 为：%s", info, fmtStr("订阅路径", GREEN, true), fmtStr(subPath, ORANGE, true))
	if !autoMode {
		for {
			if response := promptUser("- 请输入自定义订阅路径或按回车键使用生成的路径：", nil); response != "" {
				if !isValidSubURIPath(response) {
					failMessage("URI 不能包含非标准字符！请重试。")
					continue
				}

				subPath = response
			}

			break
		}
	}

	var customDomain string
	fmt.Printf("\n%s 仅当您在 Cloudflare 账户注册了域名时，才可以设置 %s。", info, fmtStr("自定义域名", GREEN, true))
	if !autoMode {
		if response := promptUser("- 请输入自定义域名（如有）或按回车键跳过：", nil); response != "" {
			customDomain = response
		}
	}

	fmt.Printf("\n%s 创建 KV 命名空间...\n", title)
	var kvNamespace *kv.Namespace

	for {
		now := time.Now().Format("2006-01-02_15-04-05")
		kvName := fmt.Sprintf("kv-%s", now)
		kvNamespace, err = createKVNamespace(ctx, kvName)
		if err != nil {
			failMessage("创建 KV 失败。")
			log.Printf("%v\n\n", err)
			if autoMode || strings.ToLower(promptUser("- 是否重试？(y/n): ", []string{"y", "n"})) == "n" {
				return
			}
			continue
		}

		successMessage("KV 创建成功！")
		break
	}

	var panel string
	if err := downloadWorker(); err != nil {
		failMessage("下载 worker.js 失败")
		log.Fatalln(err)
	}

	switch deployType {
	case DTWorker:
		panel, err = deployWorker(ctx, projectName, uid, trPass, proxyIP, nat64Prefix, fallback, subPath, kvNamespace, customDomain)
	case DTPage:
		panel, err = deployPagesProject(ctx, projectName, uid, trPass, proxyIP, nat64Prefix, fallback, subPath, kvNamespace, customDomain)
	}

	if err != nil {
		failMessage("获取面板 URL 失败。")
		log.Fatalln(err)
	}

	if err := checkBPBPanel(panel); err != nil {
		failMessage("访问 BPB 面板失败。")
		log.Fatalln(err)
	}

	// 自动模式下，写入 Clean IPs 到 KV
	if autoMode {
		if err := writeCleanIPsToKV(ctx, kvNamespace); err != nil {
			failMessage(fmt.Sprintf("写入 Clean IPs 失败：%v", err))
			log.Printf("写入 KV 失败：%v\n", err)
		}
	}
}

func modifyPanel() {
	ctx := context.Background()
	var err error
	if cfClient == nil || cfAccount == nil {
		go login()
		token := <-obtainedToken
		cfClient = NewClient(token)

		cfAccount, err = getAccount(ctx)
		if err != nil {
			failMessage("获取 Cloudflare 账户失败。")
			log.Fatalln(err)
		}
	}

	for {
		var panels []Panel
		var message string

		fmt.Printf("\n%s 获取面板列表...\n", title)
		workersList, err := listWorkers(ctx)
		if err != nil {
			failMessage("获取 Workers 列表失败。")
			log.Println(err)
		} else {
			for _, worker := range workersList {
				panels = append(panels, Panel{
					Name: worker,
					Type: "workers",
				})
			}
		}

		pagesList, err := listPages(ctx)
		if err != nil {
			failMessage("获取 Pages 列表失败。")
			log.Println(err)
		} else {
			for _, pages := range pagesList {
				panels = append(panels, Panel{
					Name: pages,
					Type: "pages",
				})
			}
		}

		if len(panels) == 0 {
			failMessage("未找到 Workers 或 Pages，正在退出...")
			return
		}

		message = fmt.Sprintf("找到 %d 个 Workers 和 Pages 项目：\n", len(panels))
		successMessage(message)
		for i, panel := range panels {
			fmt.Printf(" %s %s - %s\n", fmtStr(strconv.Itoa(i+1)+".", BLUE, true), panel.Name, fmtStr(panel.Type, ORANGE, true))
		}

		var index int
		for {
			response := promptUser("- 请选择要修改的编号：", nil)
			index, err = strconv.Atoi(response)
			if err != nil || index < 1 || index > len(panels) {
				failMessage("选择无效，请重试。")
				continue
			}

			break
		}

		panelName := panels[index-1].Name
		panelType := panels[index-1].Type

		message = fmt.Sprintf("1- %s 面板。\n2- %s 面板。\n\n- 选择：", fmtStr("更新", GREEN, true), fmtStr("删除", RED, true))
		response := promptUser(message, []string{"1", "2"})
		for {
			switch response {
			case "1":

				if err := downloadWorker(); err != nil {
					failMessage("下载 worker.js 失败")
					log.Fatalln(err)
				}

				if panelType == "workers" {
					if err := updateWorker(ctx, panelName); err != nil {
						failMessage("更新面板失败。")
						log.Fatalln(err)
					}

					successMessage("面板更新成功！")
					break
				}

				if err := updatePagesProject(ctx, panelName); err != nil {
					failMessage("更新面板失败。")
					log.Fatalln(err)
				}

				successMessage("面板更新成功！")

			case "2":

				if panelType == "workers" {
					if err := deleteWorker(ctx, panelName); err != nil {
						failMessage("删除面板失败。")
						log.Fatalln(err)
					}

					successMessage("面板删除成功！")
					break
				}

				if err := deletePagesProject(ctx, panelName); err != nil {
					failMessage("删除面板失败。")
					log.Fatalln(err)
				}

				successMessage("面板删除成功！")

			default:
				failMessage("选择错误，请仅选择 1 或 2！")
				continue
			}

			break
		}

		if response := promptUser("- 是否修改另一个面板？(y/n): ", []string{"y", "n"}); strings.ToLower(response) == "n" {
			break
		}
	}
}

// writeCleanIPsToKV 读取 result.csv 前十组 IP 地址并写入 KV
func writeCleanIPsToKV(ctx context.Context, kvNamespace *kv.Namespace) error {
	fmt.Printf("\n%s 读取测速结果并更新 %s...\n", title, fmtStr("Clean IPs", GREEN, true))

	// 打开 CSV 文件
	csvFile, err := os.Open("result.csv")
	if err != nil {
		return fmt.Errorf("打开 result.csv 失败：%w", err)
	}
	defer csvFile.Close()

	// 解析 CSV
	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("读取 CSV 失败：%w", err)
	}

	// 收集前 10 个 IP 地址（跳过表头）
	var cleanIPs []string
	maxCount := 10
	if len(records)-1 < maxCount {
		maxCount = len(records) - 1
	}

	for i := 1; i <= maxCount; i++ {
		if len(records[i]) > 0 {
			cleanIPs = append(cleanIPs, records[i][0])
		}
	}

	if len(cleanIPs) == 0 {
		return fmt.Errorf("未找到有效的 IP 地址")
	}

	// 构建 JSON
	proxySettings := map[string]interface{}{
		"cleanIPs":  cleanIPs, // 从 result.csv 读取的前 10 个 IP
		"TRConfigs": false,
		"ports":     []int{443, 80, 8080, 8880, 2052, 2082, 2086, 2095},
	}

	jsonData, err := json.Marshal(proxySettings)
	if err != nil {
		return fmt.Errorf("JSON 序列化失败：%w", err)
	}

	fmt.Printf("KV Namespace ID: %s\n", kvNamespace.ID)
	fmt.Printf("Account ID: %s\n", cfAccount.ID)
	_, err = cfClient.KV.Namespaces.Values.Update(
		ctx,
		kvNamespace.ID,
		"proxySettings",
		kv.NamespaceValueUpdateParams{
			AccountID: cloudflare.F(cfAccount.ID),
		},
		option.WithRequestBody("text/plain", jsonData),
	)
	if err != nil {
		return fmt.Errorf("写入 KV 失败：%w", err)
	}

	successMessage(fmt.Sprintf("已写入 %d 个 Clean IPs 到 KV。", len(cleanIPs)))
	return nil
}
