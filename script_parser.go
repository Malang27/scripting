package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var functions = make(map[string]string)

func executeForLoop(line string) error {
	if !strings.HasPrefix(line, "for ((") || !strings.Contains(line, ")); do") || !strings.HasSuffix(line, "done") {
		return fmt.Errorf("invalid for loop syntax, expected 'for ((i=0; i<5; i++)); do ...; done'")
	}

	parts := strings.SplitN(line, ")); do", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid for loop structure")
	}
	conditionPart := strings.TrimPrefix(parts[0], "for ((")
	conditionPart = strings.TrimSpace(conditionPart)
	commandPart := strings.TrimSpace(strings.TrimSuffix(parts[1], "done"))

	conditions := strings.Split(conditionPart, ";")
	if len(conditions) != 3 {
		return fmt.Errorf("for loop requires init, condition, and increment")
	}

	init := strings.TrimSpace(conditions[0])
	if !strings.Contains(init, "=") {
		return fmt.Errorf("invalid initialization in for loop")
	}
	varParts := strings.SplitN(init, "=", 2)
	varName := strings.TrimSpace(varParts[0])
	initValue, err := strconv.Atoi(strings.TrimSpace(varParts[1]))
	if err != nil {
		return fmt.Errorf("invalid initial value: %v", err)
	}

	cond := strings.TrimSpace(conditions[1])
	if !strings.Contains(cond, "<") {
		return fmt.Errorf("only '<' condition supported in for loop")
	}
	condParts := strings.SplitN(cond, "<", 2)
	if strings.TrimSpace(condParts[0]) != varName {
		return fmt.Errorf("condition must use the same variable as init")
	}
	limit, err := strconv.Atoi(strings.TrimSpace(condParts[1]))
	if err != nil {
		return fmt.Errorf("invalid limit value: %v", err)
	}

	inc := strings.TrimSpace(conditions[2])
	if inc != varName+"++" {
		return fmt.Errorf("only 'var++' increment supported")
	}

	for i := initValue; i < limit; i++ {
		command := strings.ReplaceAll(commandPart, "$"+varName, fmt.Sprintf("%d", i))
		cmdLines := strings.Split(command, ";")
		for _, cmd := range cmdLines {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			args := strings.Fields(expandVariables(cmd))
			if len(args) > 0 {
				if err := executeCommand(args); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func registerFunction(line string) error {
	parts := strings.SplitN(line, "() {", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid function syntax")
	}
	funcName := strings.TrimSpace(parts[0])
	body := strings.TrimSpace(strings.TrimSuffix(parts[1], "}"))

	functions[funcName] = body
	fmt.Printf("%sFunction '%s' registered%s\n", Green, funcName, Reset)
	return nil
}

func executeCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	if body, exists := functions[args[0]]; exists {
		expandedBody := body
		for i, arg := range args[1:] {
			expandedBody = strings.ReplaceAll(expandedBody, fmt.Sprintf("$%d", i+1), arg)
		}
		return executeInput(expandedBody)
	}

	cleanArgs, stdin, stdout, filesToClose, err := parseRedirections(args)
	if err != nil {
		return err
	}
	defer func() {
		for _, f := range filesToClose {
			f.Close()
		}
	}()

	switch cleanArgs[0] {
	case "exit":
		fmt.Println("Exiting shell.")
		os.Exit(0)
	case "echo":
		return echoCommand(cleanArgs, stdout)
	case "mkdir":
		return makeDirectory(cleanArgs)
	case "cat":
		return catCommand(cleanArgs, stdin, stdout)
	case "edit":
		return simpleEditor(cleanArgs, stdout)
	case "pwd":
		return pwdCommand(stdout)
	case "read":
		return readCommand(cleanArgs)
	default:
		return fmt.Errorf("command not found: %s", cleanArgs[0])
	}
	return nil
}

func echoCommand(args []string, stdout *os.File) error {
	if len(args) > 1 {
		output := strings.Join(args[1:], " ")
		if strings.HasPrefix(output, "\"") && strings.HasSuffix(output, "\"") {
			output = strings.TrimPrefix(output, "\"")
			output = strings.TrimSuffix(output, "\"")
		}
		if strings.HasPrefix(output, "'") && strings.HasSuffix(output, "'") {
			output = strings.TrimPrefix(output, "'")
			output = strings.TrimSuffix(output, "'")
		}
		fmt.Fprintln(stdout, output)
	}
	lastExitCode = 0
	return nil
}

func makeDirectory(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("mkdir: missing operand")
	}
	dir := args[1]
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0755); err != nil {
			lastExitCode = 1
			return fmt.Errorf("mkdir: %v", err)
		}
		lastExitCode = 0
	} else {
		lastExitCode = 1
	}
	return nil
}

func catCommand(args []string, stdin, stdout *os.File) error {
	if len(args) < 2 {
		_, err := io.Copy(stdout, stdin)
		if err != nil {
			lastExitCode = 1
		} else {
			lastExitCode = 0
		}
		return err
	}

	for _, file := range args[1:] {
		f, err := os.Open(file)
		if err != nil {
			lastExitCode = 1
			return fmt.Errorf("cat: %v", err)
		}
		_, err = io.Copy(stdout, f)
		f.Close()
		if err != nil {
			lastExitCode = 1
			return fmt.Errorf("cat: %v", err)
		}
	}
	lastExitCode = 0
	return nil
}

func simpleEditor(args []string, stdout *os.File) error {
	if len(args) < 2 {
		return fmt.Errorf("edit: missing file operand")
	}

	fileName := args[1]
	var content []string
	if file, err := os.Open(fileName); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			content = append(content, scanner.Text())
		}
		file.Close()
	}

	if len(content) == 0 {
		content = append(content, "")
	}

	fmt.Fprint(stdout, "\033[H\033[2J")
	fmt.Fprintln(stdout, "Éditeur de texte simple - Tapez 'SAVE' pour sauvegarder, 'EXIT' pour quitter")
	fmt.Fprintln(stdout, "----------------------------------------------------------------------------")
	for i, line := range content {
		fmt.Fprintf(stdout, "%d: %s\n", i+1, line)
	}
	fmt.Fprintln(stdout, "----------------------------------------------------------------------------")
	fmt.Fprintln(stdout, "Mode édition: entrez le numéro de ligne à modifier, ou 'NEW' pour ajouter une ligne:")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
			break
		}

		command := scanner.Text()

		if command == "EXIT" {
			return nil
		}

		if command == "SAVE" {
			file, err := os.Create(fileName)
			if err != nil {
				return fmt.Errorf("edit: %v", err)
			}
			for _, line := range content {
				fmt.Fprintln(file, line)
			}
			file.Close()
			fmt.Fprintln(stdout, "Fichier sauvegardé.")
			continue
		}

		if command == "NEW" {
			fmt.Fprintln(stdout, "Entrez le contenu de la nouvelle ligne:")
			if scanner.Scan() {
				content = append(content, scanner.Text())
			}
			continue
		}

		var lineNum int
		if _, err := fmt.Sscanf(command, "%d", &lineNum); err == nil && lineNum > 0 && lineNum <= len(content) {
			fmt.Fprintf(stdout, "Ligne %d: %s\n", lineNum, content[lineNum-1])
			fmt.Fprintln(stdout, "Entrez le nouveau contenu (ou vide pour ne pas modifier):")
			if scanner.Scan() {
				newContent := scanner.Text()
				if newContent != "" {
					content[lineNum-1] = newContent
				}
			}
			continue
		}

		fmt.Fprintf(stdout, "%sCommande non reconnue. Utilisez un numéro de ligne, 'NEW', 'SAVE' ou 'EXIT'.%s\n", Red, Reset)
	}

	return nil
}

func pwdCommand(stdout *os.File) error {
	cwd, err := os.Getwd()
	if err != nil {
		lastExitCode = 1
		return fmt.Errorf("pwd: %v", err)
	}
	fmt.Fprintln(stdout, cwd)
	lastExitCode = 0
	return nil
}

func readCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("read: missing variable name")
	}
	varName := args[1]
	
	// Get a scanner on stdin to read user input
	scanner := bufio.NewScanner(os.Stdin)
	
	// Explicitly flush any pending output to ensure prompt is displayed
	os.Stdout.Sync()
	
	// Wait for the user to enter a line
	if scanner.Scan() {
		// Set the environment variable to the user's input
		os.Setenv(varName, scanner.Text())
		lastExitCode = 0
		return nil
	} else if err := scanner.Err(); err != nil {
		lastExitCode = 1
		return fmt.Errorf("read: %v", err)
	}
	
	return nil
}

func parseRedirections(args []string) ([]string, *os.File, *os.File, []*os.File, error) {
	var stdin, stdout *os.File = os.Stdin, os.Stdout
	var filesToClose []*os.File
	var cleanArgs []string
	var err error

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case ">":
			if i+1 >= len(args) {
				return nil, nil, nil, filesToClose, fmt.Errorf("missing file for output redirection")
			}
			if stdout, err = os.Create(args[i+1]); err != nil {
				return nil, nil, nil, filesToClose, err
			}
			filesToClose = append(filesToClose, stdout)
			i++
		case ">>":
			if i+1 >= len(args) {
				return nil, nil, nil, filesToClose, fmt.Errorf("missing file for append redirection")
			}
			if stdout, err = os.OpenFile(args[i+1], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
				return nil, nil, nil, filesToClose, err
			}
			filesToClose = append(filesToClose, stdout)
			i++
		case "<":
			if i+1 >= len(args) {
				return nil, nil, nil, filesToClose, fmt.Errorf("missing file for input redirection")
			}
			if stdin, err = os.Open(args[i+1]); err != nil {
				return nil, nil, nil, filesToClose, err
			}
			filesToClose = append(filesToClose, stdin)
			i++
		default:
			cleanArgs = append(cleanArgs, args[i])
		}
	}

	return cleanArgs, stdin, stdout, filesToClose, nil
}
