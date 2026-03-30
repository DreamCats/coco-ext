package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/changelog"
	"github.com/DreamCats/coco-ext/internal/gcmsg"
)

var gcmsgAmend bool
var gcmsgChangelog bool
var gcmsgOriginal string
var gcmsgPush bool

var gcmsgCmd = &cobra.Command{
	Use:   "gcmsg",
	Short: "生成 commit message",
	Long:  "根据代码变更生成符合规范的 commit message，支持 --amend 覆盖上一个 commit",
	RunE:  runGcmsg,
}

func init() {
	rootCmd.AddCommand(gcmsgCmd)
	gcmsgCmd.Flags().BoolVarP(&gcmsgAmend, "amend", "", false, "自动 amend 到上一个 commit")
	gcmsgCmd.Flags().BoolVarP(&gcmsgChangelog, "changelog", "", false, "写入 changelog")
	gcmsgCmd.Flags().StringVarP(&gcmsgOriginal, "original", "", "", "原始 commit message")
	gcmsgCmd.Flags().BoolVarP(&gcmsgPush, "push", "", false, "amend 成功后执行 push")
}

func runGcmsg(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 获取当前分支
	branch, err := getCurrentBranch(repoRoot)
	if err != nil {
		branch = "unknown"
	}

	// 获取当前 commit 的 message（原始烂 message）
	originalMsg, err := getCurrentCommitMessage(repoRoot)
	if err != nil {
		originalMsg = ""
	}

	// 获取当前 commit 的 diff
	color.Cyan("正在获取代码变更...")
	diff, err := getCurrentCommitDiff(repoRoot)
	if err != nil {
		return fmt.Errorf("获取 diff 失败: %w", err)
	}

	var newMsg string
	var usedCache bool

	// 检查 changelog 是否有当前 message 的记录
	if gcmsgChangelog && originalMsg != "" {
		if optimizedMsg, ok := changelog.GetOptimizedMessageByMessage(repoRoot, branch, originalMsg); ok {
			color.Cyan("找到已记录的优化 message，复用...")
			newMsg = optimizedMsg
			usedCache = true
		}
	}

	// 如果没有缓存，调用 acp 生成
	if !usedCache {
		color.Cyan("正在生成 commit message...")
		newMsg, err = gcmsg.GenerateCommitMsg(repoRoot, diff)
		if err != nil {
			color.Red("生成失败: %v", err)
			return err
		}
	}

	// 打印生成的消息
	fmt.Println("\n生成的 commit message:")
	color.Green("---")
	fmt.Println(newMsg)
	color.Green("---\n")

	if gcmsgAmend {
		color.Cyan("正在执行 amend...")
		if err := amendCommit(repoRoot, newMsg); err != nil {
			color.Red("Amend 失败: %v", err)
			if gcmsgChangelog {
				writeChangelogError(repoRoot, branch, changelog.MessageHash(originalMsg), originalMsg, newMsg, err.Error())
			}
			return err
		}

		// 获取 amend 后的 commit ID
		newCommitID, err := getShortCommitID(repoRoot)
		if err != nil {
			newCommitID = "unknown"
		}

		color.Green("✓ commit message 已更新")

		// 执行 push
		if gcmsgPush {
			color.Cyan("正在执行 push...")
			if err := pushGit(repoRoot); err != nil {
				color.Red("✗ push 失败: %v", err)
				if gcmsgChangelog {
					writeChangelogError(repoRoot, branch, changelog.MessageHash(originalMsg), originalMsg, newMsg, "push failed: "+err.Error())
				}
				return err
			}
			color.Green("✓ push 成功")
		}

		// 写入 changelog
		if gcmsgChangelog {
			if err := writeChangelogSuccess(repoRoot, branch, changelog.MessageHash(originalMsg), originalMsg, newMsg, newCommitID); err != nil {
				color.Yellow("⚠ 写入 changelog 失败: %v", err)
			}
		}
	}

	return nil
}

// getCurrentCommitDiff 获取当前分支最新 commit 的 diff
func getCurrentCommitDiff(repoRoot string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD~1", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// amendCommit 用新 message 覆盖上一个 commit
func amendCommit(repoRoot, newMsg string) error {
	// 使用 git commit --amend
	cmd := exec.Command("git", "commit", "--amend", "-m", newMsg)
	cmd.Dir = repoRoot
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// pushGit 执行 git push
func pushGit(repoRoot string) error {
	cmd := exec.Command("git", "push")
	cmd.Dir = repoRoot
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// cleanCommitMsg 清理 commit message（去掉多余空白）
func cleanCommitMsg(msg string) string {
	lines := strings.Split(msg, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n")
}

// getCurrentBranch 获取当前分支名
func getCurrentBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repoRoot
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(string(output))
	}
	return branch, nil
}

// getShortCommitID 获取当前 commit 的短 ID
func getShortCommitID(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getCurrentCommitMessage 获取当前 commit 的 message（第一行）
func getCurrentCommitMessage(repoRoot string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// writeChangelogSuccess 写入成功的 changelog
func writeChangelogSuccess(repoRoot, branch, msgHash, original, optimized, newCommitID string) error {
	entry := &changelog.Entry{
		Original:    original,
		Optimized:   optimized,
		PushResult:  "success",
		CommitID:    msgHash,
		NewCommitID: newCommitID,
	}
	return changelog.WriteByMessageHash(repoRoot, branch, msgHash, entry)
}

// writeChangelogError 写入失败的 changelog
func writeChangelogError(repoRoot, branch, msgHash, original, optimized, errMsg string) error {
	entry := &changelog.Entry{
		Original:   original,
		Optimized:  optimized,
		PushResult: "error: " + errMsg,
		CommitID:   msgHash,
	}
	return changelog.WriteByMessageHash(repoRoot, branch, msgHash, entry)
}