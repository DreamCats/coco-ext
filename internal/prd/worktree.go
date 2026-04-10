package prd

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CodeWorkspace 描述 prd code 使用的隔离工作区。
type CodeWorkspace struct {
	RepoRoot        string
	BranchName      string
	MainTaskDir     string
	WorktreeDir     string
	WorktreeTaskDir string
}

// PrepareCodeWorkspace 创建或复用 prd code 专用 worktree，并同步 task/context 产物。
func PrepareCodeWorkspace(repoRoot, taskID, branchName string) (*CodeWorkspace, error) {
	mainTaskDir := filepath.Join(repoRoot, ".livecoding", "tasks", taskID)
	if _, err := os.Stat(mainTaskDir); err != nil {
		return nil, fmt.Errorf("task 目录不存在: %w", err)
	}

	worktreeDir, err := BuildCodeWorktreePath(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	if err := ensureCodeWorktree(repoRoot, worktreeDir, branchName); err != nil {
		return nil, err
	}

	worktreeTaskDir := filepath.Join(worktreeDir, ".livecoding", "tasks", taskID)
	if err := syncCodeWorkspaceArtifacts(repoRoot, worktreeDir, taskID); err != nil {
		return nil, err
	}

	return &CodeWorkspace{
		RepoRoot:        repoRoot,
		BranchName:      branchName,
		MainTaskDir:     mainTaskDir,
		WorktreeDir:     worktreeDir,
		WorktreeTaskDir: worktreeTaskDir,
	}, nil
}

// BuildCodeWorktreePath 返回指定 task 的 worktree 目录。
func BuildCodeWorktreePath(repoRoot, taskID string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("解析仓库绝对路径失败: %w", err)
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(filepath.Clean(root)))

	parentDir := filepath.Dir(root)
	repoName := sanitizeRepoName(filepath.Base(root))
	return filepath.Join(parentDir, ".coco-ext-worktree", fmt.Sprintf("%s-%08x", repoName, hasher.Sum32()), taskID), nil
}

// CleanupCodeWorktree 删除已创建的 worktree。
func CleanupCodeWorktree(repoRoot, worktreeDir string) error {
	if strings.TrimSpace(worktreeDir) == "" {
		return nil
	}

	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		return pruneCodeWorktrees(repoRoot)
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", worktreeDir)
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("删除 worktree 失败: %s\n%s", err, string(output))
	}

	return pruneCodeWorktrees(repoRoot)
}

func ensureCodeWorktree(repoRoot, worktreeDir, branchName string) error {
	if stat, err := os.Stat(worktreeDir); err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("worktree 路径已存在但不是目录: %s", worktreeDir)
		}
		if !isGitWorktree(worktreeDir) {
			return fmt.Errorf("worktree 路径已存在但不是有效 git worktree: %s", worktreeDir)
		}
		currentBranch, err := currentWorktreeBranch(worktreeDir)
		if err != nil {
			return err
		}
		if currentBranch != branchName {
			return fmt.Errorf("worktree 已存在，但当前分支为 %s，期望 %s，请先执行 `coco-ext prd reset --task %s`", currentBranch, branchName, filepath.Base(worktreeDir))
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 worktree 目录失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0755); err != nil {
		return fmt.Errorf("创建 worktree 父目录失败: %w", err)
	}

	args := []string{"worktree", "add"}
	if gitBranchExists(repoRoot, branchName) {
		args = append(args, worktreeDir, branchName)
	} else {
		args = append(args, "-b", branchName, worktreeDir)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建 worktree 失败: %s\n%s", err, string(output))
	}

	return nil
}

func syncCodeWorkspaceArtifacts(repoRoot, worktreeDir, taskID string) error {
	type syncPair struct {
		src string
		dst string
	}

	pairs := []syncPair{
		{
			src: filepath.Join(repoRoot, ".livecoding", "tasks", taskID),
			dst: filepath.Join(worktreeDir, ".livecoding", "tasks", taskID),
		},
		{
			src: filepath.Join(repoRoot, ".livecoding", "context"),
			dst: filepath.Join(worktreeDir, ".livecoding", "context"),
		},
	}

	for _, pair := range pairs {
		if _, err := os.Stat(pair.src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("检查同步源目录失败: %w", err)
		}

		_ = os.RemoveAll(pair.dst)
		if err := copyDir(pair.src, pair.dst); err != nil {
			return err
		}
	}

	return nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取目录失败 %s: %w", src, err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", dst, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开文件失败 %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("读取文件信息失败 %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("创建目标文件失败 %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("复制文件失败 %s -> %s: %w", src, dst, err)
	}

	return nil
}

func gitBranchExists(repoRoot, branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

func isGitWorktree(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

func currentWorktreeBranch(dir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 worktree 分支失败: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func pruneCodeWorktrees(repoRoot string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("清理 worktree 元信息失败: %s\n%s", err, string(output))
	}
	return nil
}

func sanitizeRepoName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "repo"
	}

	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "repo"
	}
	return result
}
