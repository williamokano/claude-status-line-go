#compdef claude-status-line-go
_claude_status_line_go() {
    local -a opts
    opts=(
        '(-h --help)'{-h,--help}'[print help]'
        '(-v --version)'{-v,--version}'[print version]'
        '--no-color[disable ANSI color output]'
        '--completion[print shell completion script]:shell:(bash zsh fish)'
        '--project[with install: register in project settings instead of global]'
        '1:command:(install)'
    )
    _arguments $opts
}
_zsh_debug "using compdef"
compdef _claude_status_line_go claude-status-line-go
