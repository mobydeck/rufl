# RunFlow (rufl)

RunFlow is a command line tool that allows executing other commands either in parallel or sequentially.

## Installation

The easiest way to install RunFlow is to download a pre-built binary from the [Releases](https://github.com/mobydeck/rufl/releases) page. Choose the appropriate version for your operating system and architecture:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Linux and macOS

1. Download the archive for your system
2. Extract it: `tar xzf rufl_*_*.tar.gz`
3. Move the binary to a directory in your PATH:
   ```bash
   sudo mv rufl /usr/local/bin/
   ```

### Windows

1. Download the ZIP archive for Windows
2. Extract the contents
3. Add the directory containing `rufl.exe` to your system's PATH, or move the executable to a directory that's already in your PATH


### Cross-Platform Builds

RunFlow can be built for multiple platforms and architectures using the included justfile. You'll
need [just](https://github.com/casey/just) installed to use these commands.

Build for all platforms and architectures:

```bash
just build-all
```

Build for a specific platform and architecture:

```bash
just build-linux-amd64
just build-macos-arm64
just build-windows-amd64
```

Create release archives:

```bash
just package
```

Available build targets:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

The compiled binaries will be placed in the `dist/` directory. All builds are optimized for size using
`-ldflags='-s -w'` and `-trimpath` flags, resulting in significantly smaller executables.

## Usage

RunFlow provides two main commands with multiple aliases:

- `=`, `p` or `parallel`: Run commands in parallel
- `+`, `s` or `sequential`: Run commands sequentially

### Examples

Run commands in parallel:

```bash
rufl = "echo hello world" "cat /etc/hosts" "while true; do echo hello; sleep 1; done"
```

Run commands sequentially:

```bash
rufl + "echo hello world" "cat /etc/hosts" "while true; do echo hello; sleep 1; done"
```

### Parallel Execution Order

When running commands in parallel mode, RunFlow ensures that commands start in the order they are provided, even though they run concurrently. This means that the first command will start first, followed by the second command, and so on. However, the commands will run concurrently, so they may finish in a different order depending on their execution time.

For example:

```bash
rufl = "sleep 2 && echo first" "sleep 1 && echo second" "echo third"
```

In this example, the commands will start in the order they are provided (first, second, third), but they will finish in a different order (third, second, first) because they have different execution times.

### Command Tagging

You can tag commands with custom names to make the output more descriptive. This is especially useful when running
multiple commands, and you want to easily identify which command produced which output.

There are two ways to tag commands:

#### Using the `-t` flag

Use the `-t` or `--tag` flag with the format `NAME:COMMAND`:

```bash
rufl = -t "greeting:echo hello" -t "hosts:cat /etc/hosts" -t "loop:while true; do echo hello; sleep 1; done"
```

#### Using the `+tagname:command` syntax

Alternatively, you can use the more concise `+tagname:command` syntax directly in your command arguments:

```bash
rufl = "+greeting:echo hello" "+hosts:cat /etc/hosts" "+loop:while true; do echo hello; sleep 1; done"
```

Both methods produce the same result, with the output using the tag names instead of numbers:

```
[greeting] hello
[hosts] 127.0.0.1 localhost
[loop] hello
```

You can mix tagged and untagged commands. Untagged commands will use numbers as identifiers:

```bash
rufl = "echo untagged command" "+tagged:echo tagged command"
```

Output:

```
[1] untagged command
[tagged] tagged command
```

#### Command Ordering

Commands are executed in the order they are specified in the command line. When mixing positional arguments and tagged
commands:

1. Positional arguments are executed in the order they appear
2. If a tagged command matches a positional argument, the tag is applied to that command
3. Any tagged commands that don't match positional arguments are executed after all positional arguments

For example:

```bash
rufl + "echo first" "echo second" "+third:echo third"
```

Will execute the commands in the order: "echo first", "echo second", "echo third".

If you tag a command that also appears as a positional argument:

```bash
rufl + "echo first" "echo second" "+second:echo second"
```

The command will be executed in its original position, but with the tag applied.

### Direct Command Execution

RunFlow intelligently determines whether a command needs a shell to execute:

- Simple commands like `echo hello` or `ls -la` are executed directly without a shell
- Commands with shell features like pipes (`|`), redirections (`>`, `<`), environment variables (`$VAR`), or glob
  patterns (`*.txt`) are executed using a shell

This provides better performance and security for simple commands while maintaining full shell functionality when
needed.

You can force all commands to use a shell with the `--shell` flag:

```bash
rufl = --shell "echo hello" "ls -la"
```

### Signal Handling

RunFlow handles signals differently depending on the execution mode:

#### Parallel Mode

In parallel mode, when signals like SIGINT (Ctrl+C), SIGTERM, or SIGHUP are received, they are forwarded to all running child processes. This ensures that when you press Ctrl+C or the terminal session is closed, all running commands are properly terminated.

```bash
rufl = "while true; do echo hello; sleep 1; done" "while true; do echo world; sleep 1; done"
# Press Ctrl+C to terminate all commands and exit
```

#### Sequential Mode

In sequential mode, RunFlow provides a more nuanced signal handling approach:

- **Single Ctrl+C**: Interrupts only the currently running command, then continues with the next command in the sequence
- **Double Ctrl+C** (within 1 second): Interrupts the current command and exits RunFlow completely

This allows you to skip a long-running command without terminating the entire sequence:

```bash
rufl + "sleep 10" "echo This will still run after Ctrl+C on the sleep command"
# Press Ctrl+C once during the sleep to skip to the next command
# Press Ctrl+C twice quickly to exit RunFlow entirely
```

When you press Ctrl+C once, you'll see a message indicating that you can press it again to exit:

```
Interrupting current command. Press Ctrl+C again within 1 second to exit rufl.
```

### Environment Variables

Commands executed by RunFlow inherit all environment variables from the parent process. This allows you to use
environment variables in your commands:

```bash
export MY_VAR="some value"
rufl = "echo $MY_VAR" "env | grep MY_VAR"
```

You can also set additional environment variables using the `-e` or `--env` flag:

```bash
rufl = -e "VAR1=value1" -e "VAR2=value2" "echo $VAR1 $VAR2"
```

These additional environment variables will be available to all commands being executed.

### Output Format

RunFlow formats command output differently based on whether color is enabled:

#### With Color Enabled (Default)

When color is enabled, the output is formatted with colored tags that indicate the command and stream type:

- Standard output is displayed in green with just the command tag: `[tag]`
- Standard error is displayed in red with just the command tag: `[tag]`

Example:
```
[greeting] hello world
[hosts] 127.0.0.1 localhost
```

The color itself indicates whether the output is from stdout (green) or stderr (red).

#### With Color Disabled

When color is disabled (using `--no-color` flag or in environments without color support), the output includes both the command tag and the stream type:

```
[greeting:out] hello world
[hosts:out] 127.0.0.1 localhost
[error:err] some error message
```

### Color Support

RunFlow uses colored output to make it easier to distinguish between different commands and output types:

- Command execution messages are displayed in cyan
- Standard output is displayed in green
- Standard error is displayed in red
- Command completion messages are displayed in green
- Command error messages are displayed in yellow or red
- Environment variable information is displayed in blue

You can disable colored output using the `--no-color` flag:

```bash
rufl = --no-color "echo hello" "echo world"
```

RunFlow also preserves ANSI color codes in command output. This means that if a command produces colored output (like `ls --color=always` or scripts that use color codes), those colors will be displayed correctly in RunFlow's output:

```bash
rufl = "ls --color=always" "grep --color=always pattern file.txt"
```

#### Windows Color Support

On Windows, ANSI color support is automatically enabled for Windows 10 version 1511 (November 2015) and later. For older
Windows versions, colors may not be displayed correctly.

## Features

- Execute multiple commands in parallel or sequentially
- Real-time output streaming (doesn't wait for commands to finish)
- Intelligent shell detection (only uses a shell when necessary)
- Proper shell command parsing using go-shlex
- Clear output formatting with command number and stream type indicators
- Colored output with automatic Windows support
- Environment variable inheritance from the parent process
- Setting additional environment variables with the `-e` flag
- Command tagging for descriptive output with the `-t` flag or `+tagname:command` syntax
- Advanced signal handling (double Ctrl+C detection in sequential mode)
- Cross-platform support (Linux, macOS, Windows) on multiple architectures (amd64, arm64)

## Dependencies

- [github.com/spf13/cobra](https://github.com/spf13/cobra) - Command line interface framework
- [github.com/anmitsu/go-shlex](https://github.com/anmitsu/go-shlex) - Shell-style lexical analyzer
- [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows) - Windows system calls (for Windows color
  support)

## License

MIT 