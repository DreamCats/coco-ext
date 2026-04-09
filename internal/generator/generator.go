package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DreamCats/coco-acp-sdk/daemon"
	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/daemonutil"
)

// CodePrompter 是代码生成所需的最小 prompt 接口。
// Generator（daemon 模式）和 RawGenerator（直连模式）均实现此接口。
type CodePrompter interface {
	PromptWithTimeout(prompt string, timeout time.Duration, onChunk func(string)) (string, error)
	Close()
}

// Generator 知识文件生成器
type Generator struct {
	conn      *daemon.Conn
	sessionID string
	modelID   string
}

// New 创建生成器，连接 coco daemon
// 每次调用都会创建新的 session，由上游 agent 决策是否复用
func New(repoPath string) (*Generator, error) {
	logPath, err := ensureDaemonStartWithLog(repoPath)
	if err != nil {
		return nil, fmt.Errorf("预启动 coco daemon 失败: %w", err)
	}

	conn, err := daemon.Dial(repoPath, &daemon.DialOption{
		ConfigDir: config.DefaultConfigDir(),
	})
	if err != nil {
		if logPath != "" {
			return nil, fmt.Errorf("连接 coco daemon 失败: %w\n建议：先执行 `coco-ext doctor --fix` 或 `coco-ext daemon start -d --cwd .`\ndaemon 启动日志：%s", err, logPath)
		}
		return nil, fmt.Errorf("连接 coco daemon 失败: %w\n建议：先执行 `coco-ext doctor --fix` 或 `coco-ext daemon start -d --cwd .`", err)
	}

	// 创建新 session；这一层当前在 coco-acp-sdk 中没有超时保护，必须由上层兜住。
	sess, err := newSessionWithTimeout(conn, repoPath, config.ContextPromptTimeout)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建 session 失败: %w\n建议：执行 `coco-ext daemon stop` 清理卡住的 daemon，再重试。", err)
	}

	// 设置当前使用的 session
	conn.UseSession(sess.SessionID)

	return &Generator{
		conn:      conn,
		sessionID: sess.SessionID,
		modelID:   config.DefaultModel,
	}, nil
}

func newSessionWithTimeout(conn *daemon.Conn, repoPath string, timeout time.Duration) (*daemon.SessionResponse, error) {
	type sessionResult struct {
		session *daemon.SessionResponse
		err     error
	}

	done := make(chan sessionResult, 1)
	go func() {
		session, err := conn.NewSession(repoPath)
		done <- sessionResult{session: session, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-done:
		if result.err != nil {
			return nil, result.err
		}
		return result.session, nil
	case <-timer.C:
		return nil, fmt.Errorf("session 创建超时（%s）", timeout)
	}
}

func ensureDaemonStartWithLog(repoPath string) (string, error) {
	configDir := config.DefaultConfigDir()
	if daemon.IsRunningAt(configDir) {
		return "", nil
	}

	if repaired, err := daemonutil.RepairBrokenState(configDir); err != nil {
		return "", fmt.Errorf("清理异常 daemon 状态失败: %w", err)
	} else if repaired {
		time.Sleep(300 * time.Millisecond)
		if daemon.IsRunningAt(configDir) {
			return "", nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	logDir := filepath.Join(configDir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", fmt.Errorf("创建 daemon 日志目录失败: %w", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("daemon-start-%s.log", time.Now().Format("20060102150405")))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return "", fmt.Errorf("创建 daemon 启动日志失败: %w", err)
	}
	defer logFile.Close()

	startCmd := exec.Command(exe, "daemon", "start", "--cwd", repoPath)
	startCmd.Args = append(startCmd.Args, "--idle-timeout", config.DaemonIdleTimeout().String())
	startCmd.Dir = repoPath
	startCmd.Stdin = nil
	startCmd.Stdout = logFile
	startCmd.Stderr = logFile
	startCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := startCmd.Start(); err != nil {
		return logPath, fmt.Errorf("启动 daemon 进程失败: %w", err)
	}
	_ = startCmd.Process.Release()

	return logPath, nil
}

// Info 获取 daemon 状态信息（PID、SessionID、ModelID、Uptime）
func (g *Generator) Info() (pid int, sessionID, modelID, uptime string, err error) {
	resp, err := g.conn.Status()
	if err != nil {
		return 0, "", "", "", err
	}
	return resp.PID, g.sessionID, g.modelID, resp.Uptime, nil
}

// Close 关闭连接
func (g *Generator) Close() {
	if g.conn != nil {
		g.conn.Close()
	}
}

// Generate 生成单个知识文件内容
func (g *Generator) Generate(name, scanSummary string, onChunk func(string)) (string, error) {
	prompt := GetPrompt(name, scanSummary)
	if prompt == "" {
		return "", fmt.Errorf("未知的知识文件: %s", name)
	}

	result, err := g.executePromptWithTimeout(prompt, config.ContextPromptTimeout, onChunk)
	if err != nil {
		return "", fmt.Errorf("生成 %s 失败: %w", name, err)
	}

	return result, nil
}

// Update 增量更新知识文件
func (g *Generator) Update(name, existingContent, diffContent string, onChunk func(string)) (string, error) {
	prompt := GetUpdatePrompt(name, existingContent, diffContent)

	result, err := g.executePromptWithTimeout(prompt, config.ContextPromptTimeout, onChunk)
	if err != nil {
		return "", fmt.Errorf("更新 %s 失败: %w", name, err)
	}

	if strings.TrimSpace(result) == "NO_UPDATE" {
		return "", nil // 无需更新
	}

	return result, nil
}

// Prompt 直接发送 prompt，返回完整响应
func (g *Generator) Prompt(prompt string, onChunk func(string)) (string, error) {
	result, err := g.executePromptWithTimeout(prompt, config.DefaultPromptTimeout, onChunk)
	if err != nil {
		return "", err
	}
	return result, nil
}

// PromptWithTimeout 直接发送 prompt，并使用指定超时
func (g *Generator) PromptWithTimeout(prompt string, timeout time.Duration, onChunk func(string)) (string, error) {
	result, err := g.executePromptWithTimeout(prompt, timeout, onChunk)
	if err != nil {
		return "", err
	}
	return result, nil
}

// PromptWithEarlyStop 发送 prompt，支持通过 stop channel 提前终止。
// 当 stop 收到信号时，立即返回已累积的内容（不视为错误）。
func (g *Generator) PromptWithEarlyStop(prompt string, timeout time.Duration, onChunk func(string), stop <-chan struct{}) (string, error) {
	if g == nil || g.conn == nil {
		return "", fmt.Errorf("daemon 连接不可用，请重新创建 generator 或重试命令")
	}

	type promptResult struct {
		content string
		err     error
	}

	var result strings.Builder
	conn := g.conn

	done := make(chan promptResult, 1)
	go func() {
		_, err := conn.Prompt(
			prompt,
			g.modelID,
			"",
			func(text string) {
				result.WriteString(text)
				if onChunk != nil {
					onChunk(text)
				}
			},
			func(kind, title, status string) {},
		)
		if err != nil {
			done <- promptResult{err: fmt.Errorf("prompt 失败: %w", err)}
			return
		}
		done <- promptResult{content: result.String()}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case promptResp := <-done:
		if promptResp.err != nil {
			return "", promptResp.err
		}
		return promptResp.content, nil
	case <-stop:
		// 提前终止：关闭连接，返回已累积内容
		if g.conn != nil {
			_ = g.conn.Close()
			g.conn = nil
		}
		return result.String(), nil
	case <-timer.C:
		if g.conn != nil {
			_ = g.conn.Close()
			g.conn = nil
		}
		return "", fmt.Errorf("prompt 超时（%s）", timeout)
	}
}

// PromptWithIdleTimeout 发送 prompt，同时支持总超时和空闲超时。
// totalTimeout 为绝对截止时间；idleTimeout 为连续无 chunk 输出的最大等待时间。
// idle timer 在收到第一个 chunk 后才启动，避免 time-to-first-token 被误判为超时。
func (g *Generator) PromptWithIdleTimeout(prompt string, totalTimeout, idleTimeout time.Duration, onChunk func(string)) (string, error) {
	if g == nil || g.conn == nil {
		return "", fmt.Errorf("daemon 连接不可用，请重新创建 generator 或重试命令")
	}

	type promptResult struct {
		content string
		err     error
	}

	var result strings.Builder
	conn := g.conn

	// 每收到一个 chunk 就向此 channel 发信号
	chunkSignal := make(chan struct{}, 1)

	done := make(chan promptResult, 1)
	go func() {
		_, err := conn.Prompt(
			prompt,
			g.modelID,
			"",
			func(text string) {
				result.WriteString(text)
				if onChunk != nil {
					onChunk(text)
				}
				// 非阻塞发送信号
				select {
				case chunkSignal <- struct{}{}:
				default:
				}
			},
			func(kind, title, status string) {},
		)
		if err != nil {
			done <- promptResult{err: fmt.Errorf("prompt 失败: %w", err)}
			return
		}
		done <- promptResult{content: result.String()}
	}()

	totalTimer := time.NewTimer(totalTimeout)
	defer totalTimer.Stop()

	// 阶段一：等待第一个 chunk，此阶段只受总超时限制
	firstChunkReceived := false
	for !firstChunkReceived {
		select {
		case promptResp := <-done:
			if promptResp.err != nil {
				return "", promptResp.err
			}
			return promptResp.content, nil
		case <-chunkSignal:
			firstChunkReceived = true
		case <-totalTimer.C:
			if g.conn != nil {
				_ = g.conn.Close()
				g.conn = nil
			}
			return result.String(), fmt.Errorf("prompt 超时（%s），等待首次输出", totalTimeout)
		}
	}

	// 阶段二：已收到第一个 chunk，启动 idle timer
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case promptResp := <-done:
			if promptResp.err != nil {
				return "", promptResp.err
			}
			return promptResp.content, nil
		case <-chunkSignal:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idleTimeout)
		case <-idleTimer.C:
			if g.conn != nil {
				_ = g.conn.Close()
				g.conn = nil
			}
			return result.String(), fmt.Errorf("prompt 空闲超时（%s 无输出）", idleTimeout)
		case <-totalTimer.C:
			if g.conn != nil {
				_ = g.conn.Close()
				g.conn = nil
			}
			return result.String(), fmt.Errorf("prompt 超时（%s）", totalTimeout)
		}
	}
}

func (g *Generator) executePromptWithTimeout(prompt string, timeout time.Duration, onChunk func(string)) (string, error) {
	if g == nil || g.conn == nil {
		return "", fmt.Errorf("daemon 连接不可用，请重新创建 generator 或重试命令")
	}

	type promptResult struct {
		content string
		err     error
	}

	var result strings.Builder
	conn := g.conn

	done := make(chan promptResult, 1)
	go func() {
		_, err := conn.Prompt(
			prompt,
			g.modelID,
			"",
			func(text string) {
				result.WriteString(text)
				if onChunk != nil {
					onChunk(text)
				}
			},
			func(kind, title, status string) {},
		)
		if err != nil {
			done <- promptResult{err: fmt.Errorf("prompt 失败: %w", err)}
			return
		}
		done <- promptResult{content: result.String()}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case promptResp := <-done:
		if promptResp.err != nil {
			return "", promptResp.err
		}
		return promptResp.content, nil
	case <-timer.C:
		if g.conn != nil {
			_ = g.conn.Close()
			g.conn = nil
		}
		return "", fmt.Errorf("prompt 超时（%s）", timeout)
	}
}
