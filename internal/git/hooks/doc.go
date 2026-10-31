// Package hooks implements Anvil's server-side git hooks with the loop-back pattern
// (docs/reference/04-git-hosting.md §3.1).
//
// Install writes tiny sh scripts into a repository's hooks/ directory. When a push
// arrives, git receive-pack runs them; they exec `anvil hook <name>`, whose body is
// RunClient. RunClient forwards the hook input to the running server's internal API
// (loopback only, authenticated by a per-process token passed in the environment), and
// InternalAPI answers by calling repos.Service.CheckPush / AfterPush. The push is
// rejected when pre-receive exits non-zero; the server's message is shown as "remote: …".
//
// Git for Windows runs the scripts with its bundled sh.exe, so the same scripts work on
// both operating systems (docs/reference/12 §2.3).
package hooks
