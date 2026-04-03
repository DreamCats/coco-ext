package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internallint "github.com/DreamCats/coco-ext/internal/lint"
)

var pushCmd = &cobra.Command{
	Use:                "push [git-push-args...]",
	Short:              "包装 git push，成功后后台触发 review",
	Long:               "执行 git push；当 push 成功后，后台触发 coco-ext review --async。\n使用 --yes 跳过 force push 确认提示。",
	DisableFlagParsing: true,
	RunE:               runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	return triggerPushFlow(repoRoot, args)
}

// containsForcePushArg 检测参数中是否包含 force push 相关参数
func containsForcePushArg(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--force", "-f", "--force-with-lease", "--force-if-includes":
			return true
		}
		if strings.HasPrefix(arg, "--force-with-lease=") || strings.HasPrefix(arg, "--force-if-includes=") {
			return true
		}
	}
	return false
}

// extractYesFlag 从 args 中提取 --yes 参数，返回清理后的 args 和是否找到 --yes
func extractYesFlag(args []string) ([]string, bool) {
	found := false
	cleaned := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--yes" {
			found = true
			continue
		}
		cleaned = append(cleaned, arg)
	}
	return cleaned, found
}

// confirmForcePush 提示用户确认 force push
func confirmForcePush() bool {
	color.Yellow("⚠ 检测到 force push 参数，这可能会覆盖远程提交历史！")
	fmt.Print("确认执行 force push? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

func triggerPushFlow(repoRoot string, args []string) error {
	// 提取 --yes（coco-ext 自有参数，不传给 git）
	gitPassArgs, skipConfirm := extractYesFlag(args)

	// force push 防护
	if containsForcePushArg(gitPassArgs) {
		if !skipConfirm && os.Getenv("COCO_EXT_PUSH_YES") != "1" {
			if !confirmForcePush() {
				color.Cyan("已取消 force push")
				return nil
			}
		}
	}

	gitArgs := append([]string{"push"}, gitPassArgs...)
	gitPushCmd := exec.Command("git", gitArgs...)
	gitPushCmd.Dir = repoRoot
	gitPushCmd.Stdin = os.Stdin
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr

	if err := gitPushCmd.Run(); err != nil {
		color.Yellow("💡 push 失败，常见原因:")
		color.Yellow("   - 远程有新提交，请先 git pull --rebase")
		color.Yellow("   - 网络问题，请稍后重试")
		return err
	}

	color.Green("Push 成功，正在后台启动 review...")

	prevLowPriority := reviewLowPriority
	reviewLowPriority = true
	defer func() {
		reviewLowPriority = prevLowPriority
	}()

	if err := startReviewAsync(repoRoot, ""); err != nil {
		color.Yellow("💡 可手动触发: coco-ext review --async")
		return fmt.Errorf("push 成功，但启动后台 review 失败: %w", err)
	}

	// golangci-lint 可用时，异步触发 lint
	if internallint.IsGolangciLintAvailable() {
		if err := startLintAsync(repoRoot); err != nil {
			color.Yellow("⚠ 后台 lint 启动失败: %v", err)
		}
	}

	return nil
}
