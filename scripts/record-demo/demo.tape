# Recorded by scripts/record-demo/record.sh, which builds the real writ
# binary and runs this against a freshly seeded throwaway repo. Every
# command below executes for real; nothing here is staged or faked.
#
# Run: vhs scripts/record-demo/demo.tape   (from the repo root, after
# scripts/record-demo/.bin/writ and .demo-repo/ exist - see record.sh,
# which does both before invoking vhs).

Output docs/assets/demo.mp4

Set Shell "zsh"
Set FontFamily "JetBrains Mono"
Set FontSize 20
Set Width 1280
Set Height 480
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 45ms

Hide
Type "export PATH=$PWD/scripts/record-demo/.bin:$PATH"
Enter
Type "cd .demo-repo"
Enter
Type "export PS1='$ '"
Enter
Type "clear"
Enter
Show

Type "cat writ-proposal.toml"
Enter
Sleep 2500ms

Type "writ propose --file writ-proposal.toml"
Enter
Sleep 1500ms

Type "writ approve --yes"
Enter
Sleep 1000ms

Type "git checkout -b writ-1"
Enter
Sleep 800ms

Type "clear"
Enter

Type "echo '--- in-scope change: the file the writ declared ---'"
Enter
Sleep 500ms
Type "cat >> internal/webhook/sender.go << 'EOF'"
Enter
Type ""
Enter
Type "func retryWithBackoff(url string) error {"
Enter
Type "	return Send(url)"
Enter
Type "}"
Enter
Type "EOF"
Enter
Sleep 500ms

Type "git commit -am 'add retry with backoff'"
Enter
Sleep 800ms

Type "writ attest retries-on-5xx --note 'retries Send on failure'"
Enter
Sleep 1200ms

Type "clear"
Enter
Type "writ status"
Enter
Sleep 3500ms

Type "clear"
Enter
Type "echo '--- out-of-scope change: nothing the writ declared ---'"
Enter
Sleep 500ms
Type "cat >> internal/other/thing.go << 'EOF'"
Enter
Type ""
Enter
Type "func unrelated() int { return 2 }"
Enter
Type "EOF"
Enter
Sleep 500ms

Type "git commit -am 'tweak unrelated file'"
Enter
Sleep 800ms

Type "clear"
Enter
Type "writ status"
Enter
Sleep 4500ms
