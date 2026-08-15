Output assets/demo.gif
Require Go

Set FontSize 14
Set Width 1120
Set Height 560
Set Padding 16

Type "# build survmap"
Enter
Type "go build -o /tmp/survmap . && echo ok"
Enter
Sleep 3s

Type "# prepare a tiny demo repo and commit it at HEAD"
Enter
Type "rm -rf /tmp/demo && mkdir -p /tmp/demo && cp examples/repo/main.go examples/sample.jsonl /tmp/demo/ && cd /tmp/demo && git init -q && git add . && git -c user.name=demo -c user.email=demo@x commit -qm init && echo 'demo repo ready'"
Enter
Sleep 2s

Type "# score a finished Claude Code session against HEAD survival"
Enter
Type "/tmp/survmap score sample.jsonl --repo ."
Enter
Sleep 2s

Type "# red/green survival heatmap"
Enter
Type "/tmp/survmap heatmap sample.jsonl --repo ."
Enter
Sleep 3s
