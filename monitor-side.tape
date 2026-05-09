Output assets/monitor-side.gif

Set Shell "bash"
Set FontSize 16
Set Width 900
Set Height 700
Set Theme "Dracula"
Set Padding 20
Set PlaybackSpeed 1
Set FontFamily "JetBrains Mono"
Env PS1 "\[\e[1;37m\]zalshy/tkt \[\e[38;5;205m\]❯\[\e[0m\] "

Sleep 1s

Type "tkt monitor side --dir .screenshots/fake-project"
Enter
Sleep 4s

# Trigger a live transition mid-recording to capture the animation
Exec "go run . advance 2 --to IN_PROGRESS --note 'planning done' --dir .screenshots/fake-project 2>/dev/null || true"
Sleep 6s

Type "q"
