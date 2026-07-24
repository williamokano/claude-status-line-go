_claude_status_line_go()
{
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-h --help -v --version --no-color --completion --project install"

    case "${prev}" in
        --completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
            return 0
            ;;
        *)
            COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
            return 0
            ;;
    esac
}
complete -F _claude_status_line_go claude-status-line-go
