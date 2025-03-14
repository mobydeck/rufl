package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/spf13/cobra"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

var (
	// Flag to disable colored output
	noColor bool
	// Flag to indicate if colors are supported
	colorSupported bool
	// Additional environment variables
	envVars []string
	// Command tags
	tags []string
	// Active commands
	activeCommands sync.Map
	// Force shell usage
	forceShell bool
	// Flag to indicate if we're running in parallel mode
	parallelMode bool
	// Time of the last SIGINT for double Ctrl+C detection
	lastSigIntTime time.Time
	// Currently running command in sequential mode
	currentSequentialCmd *exec.Cmd
	// Mutex to protect currentSequentialCmd
	currentCmdMutex sync.Mutex
)

// CommandInfo holds information about a command to be executed
type CommandInfo struct {
	Command string
	Tag     string
	Index   int
}

// shellSpecialChars contains characters that typically require a shell to interpret
var shellSpecialChars = []string{
	"|", "&", ";", "<", ">", "(", ")", "$", "`", "\\", "\"", "'", "*", "?", "[", "]", "#", "~", "=", "%",
}

func main() {
	// Try to enable color support
	enableVirtualTerminalProcessing()

	// Set up signal handling
	setupSignalHandling()

	var rootCmd = &cobra.Command{
		Use:   "rufl",
		Short: "RunFlow - Run commands in parallel or sequentially",
		Long: `RunFlow (rufl) is a command line tool that allows executing 
other commands either in parallel or sequentially.

Examples:
  rufl p "echo hello world" "cat /etc/hosts" "while true; do echo hello; sleep 1; done"
  rufl s "echo hello world" "cat /etc/hosts" "while true; do echo hello; sleep 1; done"
  
  # Tag commands with names using -t flag
  rufl p -t "greeting:echo hello" -t "hosts:cat /etc/hosts" -t "loop:while true; do echo hello; sleep 1; done"
  
  # Tag commands with names using + syntax
  rufl p "+greeting:echo hello" "+hosts:cat /etc/hosts" "+loop:while true; do echo hello; sleep 1; done"`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringArrayVarP(&envVars, "env", "e", []string{}, "Set additional environment variables (format: KEY=VALUE)")
	rootCmd.PersistentFlags().StringArrayVarP(&tags, "tag", "t", []string{}, "Tag a command with a name (format: NAME:COMMAND)")
	rootCmd.PersistentFlags().BoolVar(&forceShell, "shell", false, "Force the use of a shell for all commands")

	var parallelCmd = &cobra.Command{
		Use:     "=",
		Aliases: []string{"p", "parallel"},
		Short:   "Run commands in parallel",
		Long:    `Run multiple commands in parallel and output the results as they come in.`,
		Args:    cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			commands := processCommands(args)
			runCommands(commands, true)
		},
	}

	var sequentialCmd = &cobra.Command{
		Use:     "+",
		Aliases: []string{"s", "sequential"},
		Short:   "Run commands sequentially",
		Long:    `Run multiple commands one after another and output the results.`,
		Args:    cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			commands := processCommands(args)
			runCommands(commands, false)
		},
	}

	rootCmd.AddCommand(parallelCmd, sequentialCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// setupSignalHandling sets up handlers for various signals
func setupSignalHandling() {
	signalChan := make(chan os.Signal, 1)

	// Register for SIGINT (Ctrl+C), SIGTERM, and SIGHUP
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range signalChan {
			// Handle SIGINT (Ctrl+C) specially for sequential mode
			if sig == syscall.SIGINT && !parallelMode {
				now := time.Now()

				// Check if this is a double Ctrl+C (within 1 second)
				if !lastSigIntTime.IsZero() && now.Sub(lastSigIntTime) < time.Second {
					// Double Ctrl+C detected, exit rufl
					printColoredMessage("Double Ctrl+C detected. Exiting...", colorYellow)
					os.Exit(130) // 128 + SIGINT (2)
				}

				// Single Ctrl+C, just interrupt the current command
				lastSigIntTime = now
				printColoredMessage("Interrupting current command. Press Ctrl+C again within 1 second to exit rufl.", colorYellow)

				// Forward the signal to the current command only
				currentCmdMutex.Lock()
				if currentSequentialCmd != nil && currentSequentialCmd.Process != nil {
					_ = currentSequentialCmd.Process.Signal(sig)
				}
				currentCmdMutex.Unlock()

				// Continue the loop to handle more signals
				continue
			}

			// For other signals or parallel mode, use the original behavior
			printColoredMessage(fmt.Sprintf("Received signal: %v. Forwarding to all child processes...", sig), colorYellow)

			// Forward the signal to all active commands
			activeCommands.Range(func(key, value interface{}) bool {
				cmd := value.(*exec.Cmd)
				if cmd.Process != nil {
					// On Windows, not all signals are supported
					if runtime.GOOS == "windows" && (sig == syscall.SIGHUP) {
						// For unsupported signals on Windows, just kill the process
						_ = cmd.Process.Kill()
					} else {
						_ = cmd.Process.Signal(sig)
					}
				}
				return true
			})

			// For SIGINT and SIGTERM, exit after forwarding
			if (sig == syscall.SIGINT || sig == syscall.SIGTERM) && parallelMode {
				os.Exit(128 + int(sig.(syscall.Signal)))
			}

			// For SIGTERM in sequential mode, also exit
			if sig == syscall.SIGTERM && !parallelMode {
				os.Exit(128 + int(sig.(syscall.Signal)))
			}
		}
	}()
}

// processCommands combines regular command arguments and tagged commands
func processCommands(args []string) []CommandInfo {
	var commands []CommandInfo
	var regularArgs []string
	var taggedCommands []struct {
		Tag     string
		Command string
	}

	// First, separate regular args from +tag:command args
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") && strings.Contains(arg, ":") {
			// This is a +tag:command format
			tagParts := strings.SplitN(arg[1:], ":", 2) // Remove the + prefix
			if len(tagParts) != 2 {
				fmt.Printf("Warning: Invalid tag format '%s', expected '+NAME:COMMAND'\n", arg)
				continue
			}

			taggedCommands = append(taggedCommands, struct {
				Tag     string
				Command string
			}{
				Tag:     tagParts[0],
				Command: tagParts[1],
			})
		} else {
			// This is a regular command
			regularArgs = append(regularArgs, arg)
		}
	}

	// Add any tagged commands from the -t flag
	for _, tag := range tags {
		tagParts := strings.SplitN(tag, ":", 2)
		if len(tagParts) != 2 {
			fmt.Printf("Warning: Invalid tag format '%s', expected 'NAME:COMMAND'\n", tag)
			continue
		}

		taggedCommands = append(taggedCommands, struct {
			Tag     string
			Command string
		}{
			Tag:     tagParts[0],
			Command: tagParts[1],
		})
	}

	// Process regular command arguments first
	for i, cmd := range regularArgs {
		// Check if this command has a tag
		tag := fmt.Sprintf("%d", i+1) // Default tag is the index

		// Look for a matching tagged command
		for j, taggedCmd := range taggedCommands {
			if taggedCmd.Command == cmd {
				tag = taggedCmd.Tag
				// Remove the tagged command to avoid processing it again
				taggedCommands = append(taggedCommands[:j], taggedCommands[j+1:]...)
				break
			}
		}

		commands = append(commands, CommandInfo{
			Command: cmd,
			Tag:     tag,
			Index:   i,
		})
	}

	// Add any remaining tagged commands
	remainingIndex := len(regularArgs)
	for _, taggedCmd := range taggedCommands {
		commands = append(commands, CommandInfo{
			Command: taggedCmd.Command,
			Tag:     taggedCmd.Tag,
			Index:   remainingIndex,
		})
		remainingIndex++
	}

	if len(commands) == 0 {
		fmt.Println("Error: No commands specified. Use positional arguments, +tag:command syntax, or -t/--tag flags.")
		os.Exit(1)
	}

	return commands
}

// runCommands executes the given commands either in parallel or sequentially
func runCommands(commands []CommandInfo, parallel bool) {
	parallelMode = parallel
	if parallel {
		runParallel(commands)
	} else {
		runSequential(commands)
	}
}

// runParallel executes commands in parallel
func runParallel(commands []CommandInfo) {
	var wg sync.WaitGroup
	wg.Add(len(commands))

	// Start commands in order, but let them run concurrently
	for i, cmd := range commands {
		go func(cmdInfo CommandInfo, index int) {
			defer wg.Done()
			executeCommand(cmdInfo)
		}(cmd, i)

		// Wait a small amount of time to ensure commands start in order
		// This is a simple approach that works well in practice
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
}

// runSequential executes commands one after another
func runSequential(commands []CommandInfo) {
	for _, cmd := range commands {
		executeCommand(cmd)
	}
}

// needsShell determines if a command needs a shell to be executed
func needsShell(command string) bool {
	// If shell usage is forced, return true
	if forceShell {
		return true
	}

	// If environment variables are set, always use a shell to ensure proper expansion
	if len(envVars) > 0 {
		return true
	}

	// Check for shell special characters
	for _, char := range shellSpecialChars {
		if strings.Contains(command, char) {
			return true
		}
	}

	// Check for command chaining
	if strings.Contains(command, "&&") || strings.Contains(command, "||") || strings.Contains(command, ";") {
		return true
	}

	// Check for redirections
	if strings.Contains(command, ">") || strings.Contains(command, "<") {
		return true
	}

	// Check for pipes
	if strings.Contains(command, "|") {
		return true
	}

	// Check for glob patterns
	if strings.Contains(command, "*") || strings.Contains(command, "?") || strings.Contains(command, "[") {
		return true
	}

	return false
}

// executeCommand executes a single command
func executeCommand(cmdInfo CommandInfo) {
	var cmd *exec.Cmd

	// Check if the command needs a shell
	if needsShell(cmdInfo.Command) {
		// Determine the shell to use based on the OS
		var shell, shellArg string
		if runtime.GOOS == "windows" {
			shell = "cmd"
			shellArg = "/C"
		} else {
			shell = "sh"
			shellArg = "-c"
		}

		// Create the command using the shell
		cmd = exec.Command(shell, shellArg, cmdInfo.Command)
		printColoredMessage(fmt.Sprintf("[%s] Executing with shell: %s", cmdInfo.Tag, cmdInfo.Command), colorCyan)
	} else {
		// Parse the command using go-shlex
		args, err := shlex.Split(cmdInfo.Command, true)
		if err != nil {
			printColoredMessage(fmt.Sprintf("[%s] Error parsing command: %v", cmdInfo.Tag, err), colorRed)
			return
		}

		if len(args) == 0 {
			printColoredMessage(fmt.Sprintf("[%s] Empty command", cmdInfo.Tag), colorRed)
			return
		}

		// Create the command directly without a shell
		cmd = exec.Command(args[0], args[1:]...)
		printColoredMessage(fmt.Sprintf("[%s] Executing directly: %s", cmdInfo.Tag, cmdInfo.Command), colorCyan)
	}

	// If in sequential mode, set this as the current command
	if !parallelMode {
		currentCmdMutex.Lock()
		currentSequentialCmd = cmd
		currentCmdMutex.Unlock()
	}

	// Inherit environment variables from the parent process
	env := os.Environ()

	// Add any additional environment variables
	if len(envVars) > 0 {
		env = append(env, envVars...)
	}

	cmd.Env = env

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe for command %s: %v\n", cmdInfo.Tag, err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe for command %s: %v\n", cmdInfo.Tag, err)
		return
	}

	// Print environment variables if any were added
	if len(envVars) > 0 {
		printColoredMessage(fmt.Sprintf("[%s] With additional environment: %s", cmdInfo.Tag, strings.Join(envVars, ", ")), colorPurple)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		printColoredMessage(fmt.Sprintf("[%s] Error starting command: %v", cmdInfo.Tag, err), colorRed)
		return
	}

	// Store the command in the active commands map
	cmdID := fmt.Sprintf("%s-%d", cmdInfo.Tag, cmd.Process.Pid)
	activeCommands.Store(cmdID, cmd)

	// Create a wait group for the goroutines that read output
	var outputWg sync.WaitGroup
	outputWg.Add(2)

	// Process stdout
	go func() {
		defer outputWg.Done()
		processOutput(stdout, cmdInfo.Tag, "out", colorGreen)
	}()

	// Process stderr
	go func() {
		defer outputWg.Done()
		processOutput(stderr, cmdInfo.Tag, "err", colorRed)
	}()

	// Wait for all output to be processed
	outputWg.Wait()

	// Wait for the command to complete
	err = cmd.Wait()

	// Remove the command from the active commands map
	activeCommands.Delete(cmdID)

	// If in sequential mode, clear the current command
	if !parallelMode {
		currentCmdMutex.Lock()
		currentSequentialCmd = nil
		currentCmdMutex.Unlock()
	}

	if err != nil {
		// Check if it's an exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			printColoredMessage(fmt.Sprintf("[%s] Command exited with status: %d", cmdInfo.Tag, status.ExitStatus()), colorYellow)
		} else {
			printColoredMessage(fmt.Sprintf("[%s] Error waiting for command: %v", cmdInfo.Tag, err), colorRed)
		}
	} else {
		printColoredMessage(fmt.Sprintf("[%s] Command completed successfully", cmdInfo.Tag), colorGreen)
	}
}

// processOutput reads from a pipe and prints the output with a prefix
func processOutput(pipe io.Reader, tag string, streamType string, color string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()

		// Format the prefix differently based on color settings
		var prefix string
		if noColor || !colorSupported {
			// When color is disabled, include the stream type in the prefix
			prefix = fmt.Sprintf("[%s:%s] ", tag, streamType)
			fmt.Println(prefix + line)
		} else {
			// When color is enabled, omit the stream type as the color indicates it
			prefix = fmt.Sprintf("[%s] ", tag)
			fmt.Print(color + prefix + colorReset + line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		printColoredMessage(fmt.Sprintf("[%s] Error reading %s: %v", tag, streamType, err), colorRed)
	}
}

// printColoredMessage prints a message with the specified color
func printColoredMessage(message string, color string) {
	if noColor || !colorSupported {
		fmt.Println(message)
	} else {
		fmt.Println(color + message + colorReset)
	}
}
