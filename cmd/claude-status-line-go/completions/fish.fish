function __fish_claude_status_line_go_complete
    set -lx COMP_LINE (commandline -pc)
    set -lx COMP_POINT (string length (commandline))
    claude-status-line-go 2>/dev/null
end

complete -c claude-status-line-go -f
complete -c claude-status-line-go -s h -l help -d "print help"
complete -c claude-status-line-go -s v -l version -d "print version"
complete -c claude-status-line-go -l no-color -d "disable ANSI color output"
complete -c claude-status-line-go -l completion -d "print shell completion script" -xa "bash zsh fish"
complete -c claude-status-line-go -l project -d "with install: register in project settings instead of global"
complete -c claude-status-line-go -n "__fish_use_subcommand" -a "install" -d "register as the Claude Code status line"
