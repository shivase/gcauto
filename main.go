// Package main provides gcauto, a tool that automatically generates git commit messages using AI.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var execCommand = exec.Command

// AIExecutor defines the interface for generating commit messages.
type AIExecutor interface {
	GenerateCommitMessage(diff string) (string, error)
}

// ClaudeExecutor implements AIExecutor for the Claude model.
type ClaudeExecutor struct{}

// GenerateCommitMessage generates a commit message using the Claude model.
func (e *ClaudeExecutor) GenerateCommitMessage(diff string) (string, error) {
	prompt := fmt.Sprintf("以下のgitの差分情報に基づいて、conventional commitsフォーマットで日本語のコミットメッセージを作成してください。\n\n---\n%s\n---\n\n以下の形式で直接出力してください：\n型: 簡潔な変更内容\n\n- 具体的な変更点1\n- 具体的な変更点2\n- 具体的な変更点3\n\n注意事項：\n- 前置きや説明文は一切含めないでください\n- コミットメッセージ本文のみを出力してください\n- 🤖やCo-Authored-Byなどの情報は含めないでください\n- 型は feat/fix/docs/style/refactor/test/chore から適切なものを選択してください", diff)

	// #nosec G204
	cmd := execCommand("claude", "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude execution failed: %w: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude command: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GeminiExecutor implements AIExecutor for the Gemini model.
type GeminiExecutor struct{}

// GenerateCommitMessage generates a commit message using the Gemini model.
func (e *GeminiExecutor) GenerateCommitMessage(diff string) (string, error) {
	prompt := fmt.Sprintf("以下のgitの差分情報に基づいて、conventional commitsフォーマットで日本語のコミットメッセージを作成してください。\n\n---\n%s\n---\n\n以下の形式で直接出力してください：\n型: 簡潔な変更内容\n\n- 具体的な変更点1\n- 具体的な変更点2\n- 具体的な変更点3\n\n注意事項：\n- 前置きや説明文は一切含めないでください\n- コミットメッセージ本文のみを出力してください\n- やCo-Authored-Byなどの情報は含めないでください\n- 型は feat/fix/docs/style/refactor/test/chore から適切なものを選択してください", diff)

	// #nosec G204
	cmd := execCommand("gemini", "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("gemini execution failed: %w: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run gemini command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var filteredLines []string
	for _, line := range lines {
		if !strings.Contains(line, "Loaded cached credentials.") {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(filteredLines, "\n")), nil
}

var newExecutor = func(model string) (AIExecutor, error) {
	switch model {
	case "claude":
		return &ClaudeExecutor{}, nil
	case "gemini":
		return &GeminiExecutor{}, nil
	default:
		return nil, fmt.Errorf("invalid model specified: %s", model)
	}
}

var version = "dev" // Can be set during build

func main() {
	model := flag.String("model", "claude", "AI model to use (claude or gemini)")
	modelShort := flag.String("m", "", "AI model to use (claude or gemini) (shorthand for -model)")
	showHelp := flag.Bool("h", false, "Show help message")
	showHelpLong := flag.Bool("help", false, "Show help message (longhand for -h)")
	showVersion := flag.Bool("version", false, "Show version information")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "gcauto: AI-powered git commit message generator.\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Usage of gcauto:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  gcauto [flags]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *modelShort != "" {
		*model = *modelShort
	}

	if *showHelp || *showHelpLong {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("gcauto version %s\n", version)
		os.Exit(0)
	}

	fmt.Printf("🚀 gcauto: Starting automatic commit process using %s...\n", *model)

	executor, err := newExecutor(*model)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	diff, err := getStagedDiff()
	if err != nil {
		fmt.Printf("❌ Error: Failed to get git diff: %v\n", err)
		os.Exit(1)
	}

	if diff == "" {
		fmt.Println("✅ No changes staged for commit. Nothing to do.")
		os.Exit(0)
	}

	commitMessage, err := executor.GenerateCommitMessage(diff)
	if err != nil {
		fmt.Printf("❌ Error: Failed to generate commit message: %v\n", err)
		os.Exit(1)
	}

	if commitMessage == "" {
		fmt.Println("❌ Error: Commit message is empty")
		os.Exit(1)
	}

	fmt.Println("\n📝 Generated Commit Message:")
	fmt.Println("===================================")
	fmt.Println(commitMessage)
	fmt.Println("===================================")

	fmt.Print("\nDo you want to commit with this message? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("❌ Error: Failed to read input: %v\n", err)
		os.Exit(1)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		if err := gitCommit(commitMessage); err != nil {
			fmt.Printf("\n❌ Commit failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✅ Commit completed successfully!")
	} else {
		fmt.Println("\n⏹️ Commit cancelled.")
		os.Exit(0)
	}
}

func gitCommit(message string) error {
	cmd := execCommand("git", "commit", "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func _getStagedDiff() (string, error) {
	cmd := execCommand("git", "diff", "--staged")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

var getStagedDiff = _getStagedDiff
