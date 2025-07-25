package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// MockAIExecutor is a mock implementation of AIExecutor for testing.
type MockAIExecutor struct {
	MockResponse string
	MockError    error
}

// Execute returns the mock response or error.
func (m *MockAIExecutor) Execute(prompt string) (string, error) {
	if m.MockError != nil {
		return "", m.MockError
	}
	return m.MockResponse, nil
}

func TestGenerateCommitMessage(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockError    error
		wantError    bool
		wantEmpty    bool
	}{
		{
			name:         "valid conventional commit message",
			mockResponse: "feat: 新しい機能を追加\n\n- 機能1を追加\n- 機能2を追加",
			wantError:    false,
			wantEmpty:    false,
		},
		{
			name:         "fix type commit message",
			mockResponse: "fix: バグを修正",
			wantError:    false,
			wantEmpty:    false,
		},
		{
			name:      "error from ai",
			mockError: os.ErrNotExist,
			wantError: true,
		},
		{
			name:         "empty response",
			mockResponse: "",
			wantError:    false,
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &MockAIExecutor{
				MockResponse: tt.mockResponse,
				MockError:    tt.mockError,
			}

			message, err := generateCommitMessage(executor, "fake diff")

			if tt.wantError {
				if err == nil {
					t.Error("generateCommitMessage() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("generateCommitMessage() unexpected error = %v", err)
				return
			}

			if tt.wantEmpty {
				if message != "" {
					t.Errorf("generateCommitMessage() expected empty message but got %s", message)
				}
				return
			}

			if message == "" {
				t.Error("generateCommitMessage() returned empty message")
			}

			conventionalTypes := []string{"feat:", "fix:", "docs:", "style:", "refactor:", "test:", "chore:"}
			hasValidType := false
			for _, typ := range conventionalTypes {
				if strings.HasPrefix(message, typ) {
					hasValidType = true
					break
				}
			}

			if !hasValidType && !tt.wantEmpty {
				t.Errorf("generateCommitMessage() returned message without valid conventional commit type: %s", message)
			}
		})
	}
}

func TestGitCommit(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git user.email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git user.name: %v", err)
	}

	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "add", testFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	err = gitCommit("test: テストコミット")
	if err != nil {
		t.Errorf("gitCommit() error = %v", err)
	}

	cmd = exec.Command("git", "log", "--oneline", "-1")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git log: %v", err)
	}

	if !strings.Contains(string(output), "test: テストコミット") {
		t.Errorf("Commit message not found in git log: %s", output)
	}
}

func TestMainUserInput(t *testing.T) {
	originalGetStagedDiff := getStagedDiff
	getStagedDiff = func() (string, error) {
		return "fake diff for main user input test", nil
	}
	defer func() {
		getStagedDiff = originalGetStagedDiff
	}()

	originalNewExecutor := newExecutor
	newExecutor = func(model string) (AIExecutor, error) {
		return &MockAIExecutor{
			MockResponse: "test: テスト用のコミットメッセージ",
		}, nil
	}
	defer func() {
		newExecutor = originalNewExecutor
	}()

	tests := []struct {
		name     string
		input    string
		wantExit int
	}{
		{
			name:     "User cancels with 'n'",
			input:    "n\n",
			wantExit: 0,
		},
		{
			name:     "User cancels with 'N'",
			input:    "N\n",
			wantExit: 0,
		},
		{
			name:     "User cancels with empty input",
			input:    "\n",
			wantExit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if os.Getenv("BE_CRASHER") == "1" {
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				os.Stdin = r

				go func() {
					_, _ = io.WriteString(w, tt.input)
					_ = w.Close()
				}()

				main()
				os.Stdin = oldStdin
				return
			}

			cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
			cmd.Env = append(os.Environ(), "BE_CRASHER=1")

			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			if e, ok := err.(*exec.ExitError); ok {
				if e.ExitCode() != tt.wantExit {
					t.Errorf("Process exited with code %d, want %d", e.ExitCode(), tt.wantExit)
				}
			} else if err != nil {
				t.Errorf("Process exited with unexpected error: %v", err)
			} else if tt.wantExit != 0 {
				t.Errorf("Process did not exit as expected")
			}
		})
	}
}

func TestMain_InvalidModel(t *testing.T) {
	originalNewExecutor := newExecutor
	newExecutor = func(model string) (AIExecutor, error) {
		return nil, fmt.Errorf("invalid model specified: %s", model)
	}
	defer func() {
		newExecutor = originalNewExecutor
	}()

	if os.Getenv("BE_CRASHER") == "1" {
		// This part of the test runs in a separate process.
		// When the test is re-run with BE_CRASHER, os.Args contains flags for the
		// test runner, followed by "--", followed by flags for our main function.
		// We need to strip out the test runner flags.
		args := os.Args
		for i, arg := range args {
			if arg == "--" {
				os.Args = append([]string{args[0]}, args[i+1:]...)
				break
			}
		}
		main()
		return
	}

	// This is the main test process.
	cmd := exec.Command(os.Args[0], "-test.run=TestMain_InvalidModel", "--", "-model=invalid")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")

	output, err := cmd.CombinedOutput()
	if e, ok := err.(*exec.ExitError); ok {
		if e.ExitCode() != 1 {
			t.Errorf("Process exited with code %d, want 1", e.ExitCode())
		}
	} else if err != nil {
		t.Errorf("Process exited with unexpected error: %v", err)
	} else {
		t.Errorf("Process did not exit as expected")
	}

	expectedError := "invalid model specified: invalid"
	if !strings.Contains(string(output), expectedError) {
		t.Errorf("Expected output to contain '%s', but got '%s'", expectedError, string(output))
	}
}
