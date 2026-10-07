# VHS script for the README demo. The lead records it:
#
#   go build -o assay ./cmd/assay && vhs docs/demo.tape
#
# Nothing here reaches a network. The audit log comes from the demo helper
# under scripts/demo/auditlog; the key is generated on the spot and lives
# only in this shell.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1280
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

Hide
Type `export PATH="$PWD:$PATH" PS1='$ ' && clear` Enter
Show

Type "assay version" Enter
Sleep 1.5s

Type "assay tools verify --manifests internal/tools/manifests --trust trust/keys" Enter
Sleep 3s

Type "clear" Enter
Type "# an allowed request: recon may read" Enter
Type "assay policy eval --policy policy/default.yaml --manifests internal/tools/manifests --trust trust/keys --agent recon --tool http_get" Enter
Sleep 3s

Type "# a denied request: recon may not write (exit 1)" Enter
Type "assay policy eval --policy policy/default.yaml --manifests internal/tools/manifests --trust trust/keys --agent recon --tool http_post" Enter
Sleep 3s

Type "clear" Enter
Type "# write a small encrypted audit log with a throwaway key, then verify its hash chain" Enter
Type `export ASSAY_AUDIT_KEY="$(assay audit keygen)"` Enter
Sleep 500ms
Type "go run ./scripts/demo/auditlog -out /tmp/demo-audit.log" Enter
Sleep 3s
Type "assay audit verify --log /tmp/demo-audit.log --key-env ASSAY_AUDIT_KEY" Enter
Sleep 4s

Hide
Type "rm -f /tmp/demo-audit.log; unset ASSAY_AUDIT_KEY" Enter
Show
Sleep 1s
