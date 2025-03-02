package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
)

var lastExitCode = 0

func main() {
	fmt.Printf("%sBienvenue dans 0-shell ! Tapez 'exit' pour quitter.%s\n", Green, Reset)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		cwd, _ := os.Getwd()
		fmt.Printf("%s%s%s $ ", Purple, cwd, Reset)

		if !scanner.Scan() {
			fmt.Println("\nExiting shell.")
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "bash ") {
			scriptFile := strings.TrimPrefix(input, "bash ")
			scriptFile = strings.TrimSpace(scriptFile)
			if err := executeScript(scriptFile); err != nil {
				fmt.Printf("%sError executing script: %v%s\n", Red, err, Reset)
			}
			continue
		}

		if err := executeInput(input); err != nil {
			fmt.Printf("%sError: %v%s\n", Red, err, Reset)
			lastExitCode = 1
		} else {
			lastExitCode = 0
		}
	}
}

func executeInput(input string) error {
	lines := strings.Split(input, "\n")
	var inIfBlock, inElseBlock bool
	var ifLines, elseLines []string
	var conditionResult bool

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "if ") {
			parts := strings.SplitN(line, "then", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid if syntax, missing 'then'")
			}
			condition := strings.TrimSpace(strings.TrimPrefix(parts[0], "if"))
			conditionResult = evaluateCondition(condition)
			inIfBlock = conditionResult
			inElseBlock = false // Reset else block
			continue
		} else if strings.HasPrefix(line, "else") {
			if !conditionResult && !inIfBlock && !inElseBlock {
				return fmt.Errorf("else without preceding if")
			}
			inIfBlock = false
			inElseBlock = !conditionResult
			continue
		} else if strings.HasPrefix(line, "fi") {
			if inIfBlock {
				for _, cmd := range ifLines {
					if err := executeCommand(strings.Fields(expandVariables(cmd))); err != nil {
						return err
					}
				}
			} else if inElseBlock {
				for _, cmd := range elseLines {
					if err := executeCommand(strings.Fields(expandVariables(cmd))); err != nil {
						return err
					}
				}
			}
			inIfBlock, inElseBlock = false, false
			ifLines, elseLines = nil, nil
			continue
		} else if inIfBlock {
			ifLines = append(ifLines, line)
			continue
		} else if inElseBlock {
			elseLines = append(elseLines, line)
			continue
		}

		if strings.Contains(line, "=") && !strings.Contains(line, " ") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := parts[1]
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = strings.TrimPrefix(value, "\"")
					value = strings.TrimSuffix(value, "\"")
				}
				if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
					value = strings.TrimPrefix(value, "'")
					value = strings.TrimSuffix(value, "'")
				}
				os.Setenv(parts[0], value)
				continue
			}
		}

		if strings.HasPrefix(line, "for ((") {
			return executeForLoop(line)
		} else if strings.HasSuffix(line, "}") && strings.Contains(line, "() {") {
			return registerFunction(line)
		} else {
			expandedLine := expandVariables(line)
			args := strings.Fields(expandedLine)
			if len(args) > 0 {
				return executeCommand(args)
			}
		}
	}
	return nil
}

func executeScript(scriptFile string) error {
	file, err := os.Open(scriptFile)
	if err != nil {
		return fmt.Errorf("cannot open script file: %v", err)
	}
	defer file.Close()

	// Read the entire script content at once
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading script: %v", err)
	}

	// Skip the shebang line if present
	scriptContent := string(content)
	lines := strings.Split(scriptContent, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		// Remove the shebang line
		scriptContent = strings.Join(lines[1:], "\n")
	}

	// Make sure to flush stdout before executing the script
	// This ensures any pending output is displayed
	os.Stdout.Sync()

	// Execute the script as a single input string
	return executeInput(scriptContent)
}

func expandVariables(input string) string {
	for strings.Contains(input, "$(") {
		start := strings.Index(input, "$(")
		end := strings.Index(input[start:], ")") + start
		if end < start {
			break
		}
		cmd := input[start+2 : end]
		args := strings.Fields(cmd)
		if len(args) > 0 {
			output := captureCommandOutput(args)
			input = input[:start] + output + input[end+1:]
		}
	}

	result := ""
	inVar := false
	varName := ""

	for i := 0; i < len(input); i++ {
		if input[i] == '$' && i+1 < len(input) && (isAlphaNum(input[i+1]) || input[i+1] == '_' || input[i+1] == '?') {
			inVar = true
			varName = ""
			continue
		}

		if inVar {
			if isAlphaNum(input[i]) || input[i] == '_' {
				varName += string(input[i])
			} else {
				inVar = false
				if varName == "?" {
					result += fmt.Sprintf("%d", lastExitCode)
				} else if value := os.Getenv(varName); value != "" {
					result += value
				} else {
					result += "$" + varName
				}
				result += string(input[i])
			}
		} else {
			result += string(input[i])
		}
	}

	if inVar {
		if varName == "?" {
			result += fmt.Sprintf("%d", lastExitCode)
		} else if value := os.Getenv(varName); value != "" {
			result += value
		} else {
			result += "$" + varName
		}
	}

	return result
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func captureCommandOutput(args []string) string {
	pipeR, pipeW, _ := os.Pipe()
	defer pipeR.Close()
	defer pipeW.Close()

	savedStdout := os.Stdout
	os.Stdout = pipeW

	executeCommand(args)
	os.Stdout = savedStdout
	pipeW.Close()

	var output strings.Builder
	scanner := bufio.NewScanner(pipeR)
	for scanner.Scan() {
		output.WriteString(scanner.Text() + "\n")
	}
	return strings.TrimSpace(output.String())
}

func evaluateCondition(condition string) bool {
	condition = strings.TrimSpace(condition)
	if !strings.HasPrefix(condition, "[") || !strings.HasSuffix(condition, "]") {
		return false
	}
	condition = strings.TrimPrefix(condition, "[")
	condition = strings.TrimSuffix(condition, "]")
	condition = strings.TrimSpace(condition)

	parts := strings.Fields(condition)
	if len(parts) != 3 {
		return false
	}

	if parts[0] == "$?" {
		parts[0] = fmt.Sprintf("%d", lastExitCode)
	}
	if parts[1] == "-eq" {
		num1, err1 := strconv.Atoi(parts[0])
		num2, err2 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil {
			return num1 == num2
		}
	}
	return false
}
