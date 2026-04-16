package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	"xyzw_study/web"

	"github.com/fatih/color"
)

// 当前应用版本
const appVersion = "v1.1.9" // 请根据实际情况修改

// Release 结构体用于解析GitHub API返回的release信息
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

// 显示免责声明
func showDisclaimer() {
	// 创建彩色输出对象
	cyan := color.New(color.FgCyan).Add(color.Bold)
	yellow := color.New(color.FgYellow)

	green := color.New(color.FgGreen)

	// 显示标题
	cyan.Println("=================================================================")
	cyan.Println("                      咸鱼之王分析工具")
	cyan.Printf("                         版本: %s\n", appVersion)
	cyan.Println("=================================================================")

	// 显示免责声明
	yellow.Println("免责声明:")
	yellow.Println("1. 本工具仅用于学习和研究网络协议分析技术，禁止用于任何非法用途")
	yellow.Println("2. 使用本工具造成的任何后果由用户自行承担")
	yellow.Println("3. 本工具不会收集任何用户个人信息或游戏账号信息")
	yellow.Println("4. 使用本工具可能违反游戏服务条款，请谨慎使用")
	fmt.Println()

	// 显示项目信息
	green.Println("项目信息:")
	green.Println("- 项目地址: https://github.com/husanpao/xyzw_analyzer")
	green.Println("- 问题反馈: 请在GitHub项目页面提交issue")

	cyan.Println("=================================================================")
	fmt.Println()
}

// 等待用户按键退出
func waitForExit() {
	reader := bufio.NewReader(os.Stdin)
	color.Yellow("按Enter键退出程序...")
	_, _ = reader.ReadString('\n')
}

// 检查GitHub仓库是否有新版本
func checkForUpdates() bool {
	color.Cyan("正在检查更新...")

	// 设置请求超时
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 请求GitHub API获取最新release信息
	resp, err := client.Get("https://api.github.com/repos/husanpao/xyzw_analyzer/releases/latest")
	if err != nil {
		color.Red("检查更新失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		color.Red("读取更新信息失败: %v", err)
		return false
	}

	// 解析JSON响应
	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		color.Red("解析更新信息失败: %v", err)
		return false
	}

	// 简单比较版本
	localVer := strings.TrimPrefix(appVersion, "v")
	remoteVer := strings.TrimPrefix(release.TagName, "v")

	if remoteVer > localVer {
		fmt.Println()

		color.Yellow("发现新版本: %s", release.TagName)

		// 直接打开下载页面
		openBrowser(release.HTMLURL)

		return true
	} else {
		color.Green("当前已是最新版本")
		return false
	}
}

// 打开浏览器访问指定URL
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		color.Red("不支持自动打开浏览器")
		return
	}

	if err != nil {
		color.Red("打开浏览器失败: %v", err)
	}
}

func main() {
	// 显示免责声明
	showDisclaimer()

	// 服务器模式：设置 SERVER_MODE=1 跳过系统代理设置（适用于 Docker/服务器部署）
	serverMode := os.Getenv("SERVER_MODE") == "1"

	if serverMode {
		color.Cyan("服务器模式已启用（SERVER_MODE=1）")
		color.Yellow("请在手机 WiFi 设置中手动配置代理：服务器IP:12311")
	} else {
		// 检查更新，如果有新版本则退出程序
		if checkForUpdates() {
			fmt.Println()
			waitForExit()
			return
		}

		// 设置代理
		err := setGlobalProxy("127.0.0.1:12311", "localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*")
		if err != nil {
			color.Red("设置系统代理失败: %v", err)
			panic(err)
		}
		color.Green("系统代理设置成功")

		// 确保在函数返回时关闭代理
		defer func() {
			color.Yellow("正在关闭系统代理...")
			proxyOff()
			color.Green("系统代理已关闭")
		}()
	}

	// 设置信号处理，捕获中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go web.StartWebServer()
	color.Green("Web服务器已启动，访问 http://localhost:12582")

	// 等待中断信号
	<-c
	color.Yellow("收到退出信号，程序即将关闭...")
}
