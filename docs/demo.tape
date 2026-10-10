# VHS script for the README demo. Render it from the repository root:
#
#   vhs docs/demo.tape
#
# It records scripts/demo.sh, which needs no API key, model or outbound
# network (the first build downloads Go modules if they are not cached).
# The harness is built before recording starts so the take shows the demo,
# not a compile.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 980
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set WaitTimeout 180s

Hide
Type `export PS1='$ ' && go build ./... && (cd labs/synthetic-ops && go build ./...); echo READY-$((1+1))` Enter
Wait+Screen /READY-2/
Type "clear" Enter
Show

Type "./scripts/demo.sh" Enter
Wait+Screen /done in [0-9]+s/
Sleep 10s
