// Command devtool runs the project's development tasks the same way on Windows and Linux,
// replacing a Makefile or shell scripts (docs/reference/14-code-structure.md §1).
//
//	go run ./tools/devtool build   # frontend (if Node is available) + anvil binary into bin/
//	go run ./tools/devtool test    # go test ./... (add -race on Linux with: test -race)
//	go run ./tools/devtool lint    # gofmt check + go vet for linux and windows
//	go run ./tools/devtool dist    # CGO_ENABLED=0 binaries for every release target
//	go run ./tools/devtool dev     # build, then init (once) and serve with ./.dev/anvil.toml
//	go run ./tools/devtool docs    # rebuild the documentation website in site/
//
// Every task prints the commands it runs.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var tasks = map[string]func(args []string) error{
	"build": build,
	"test":  test,
	"lint":  lint,
	"dist":  dist,
	"dev":   dev,
	"docs":  docs,
}

func main() {
	if len(os.Args) < 2 || tasks[os.Args[1]] == nil {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/devtool <build|test|lint|dist|dev|docs> [args]")
		os.Exit(2)
	}
	if err := tasks[os.Args[1]](os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "devtool:", err)
		os.Exit(1)
	}
}

// run executes name with args in the repository root, streaming output, with extra env.
func run(env []string, name string, args ...string) error {
	fmt.Printf("$ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

func output(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ldflags stamps version information into internal/version.
func ldflags() string {
	v := os.Getenv("ANVIL_VERSION")
	if v == "" {
		v = "dev"
	}
	commit := output("git", "rev-parse", "HEAD")
	date := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("-s -w -X anvil/internal/version.Version=%s -X anvil/internal/version.Commit=%s -X anvil/internal/version.BuildDate=%s", v, commit, date)
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// buildWeb runs the Vite build when web/package.json exists and npm is installed.
// Without them the binary embeds web/dist/placeholder.html.
func buildWeb() error {
	if _, err := os.Stat(filepath.Join("web", "package.json")); err != nil {
		fmt.Println("(web/package.json not found: embedding the placeholder page)")
		return nil
	}
	npm := "npm"
	if _, err := exec.LookPath(npm); err != nil {
		fmt.Println("(npm not found: embedding whatever is in web/dist)")
		return nil
	}
	if _, err := os.Stat(filepath.Join("web", "node_modules")); err != nil {
		if err := run(nil, npm, "--prefix", "web", "ci"); err != nil {
			return err
		}
	}
	return run(nil, npm, "--prefix", "web", "run", "build")
}

func build(args []string) error {
	if err := buildWeb(); err != nil {
		return err
	}
	return run([]string{"CGO_ENABLED=0"}, "go", "build", "-trimpath", "-ldflags", ldflags(), "-o", filepath.Join("bin", exe("anvil")), "./cmd/anvil")
}

func test(args []string) error {
	if err := run(nil, "go", append([]string{"test", "-count=1"}, append(args, "./...")...)...); err != nil {
		return err
	}
	// Frontend unit tests, when the web dependencies are installed (devtool build installs them).
	if _, err := os.Stat(filepath.Join("web", "node_modules")); err == nil {
		return run(nil, "npm", "--prefix", "web", "test")
	}
	return nil
}

func lint(args []string) error {
	fmt.Println("$ gofmt -l .")
	if out := output("gofmt", "-l", "."); out != "" {
		return fmt.Errorf("files need gofmt -w:\n%s", out)
	}
	for _, goos := range []string{"linux", "windows"} {
		if err := run([]string{"GOOS=" + goos, "CGO_ENABLED=0"}, "go", "vet", "./..."); err != nil {
			return fmt.Errorf("go vet (GOOS=%s): %w", goos, err)
		}
	}
	return nil
}

// releaseTargets are the platforms from docs/reference/12-cross-platform.md §3.
var releaseTargets = []string{"windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64"}

func dist(args []string) error {
	if err := buildWeb(); err != nil {
		return err
	}
	for _, t := range releaseTargets {
		goos, goarch, _ := strings.Cut(t, "/")
		name := "anvil-" + goos + "-" + goarch
		if goos == "windows" {
			name += ".exe"
		}
		env := []string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=0"} // NFR-PORT-02: no cgo
		if err := run(env, "go", "build", "-trimpath", "-ldflags", ldflags(), "-o", filepath.Join("dist", name), "./cmd/anvil"); err != nil {
			return err
		}
	}
	return nil
}

func dev(args []string) error {
	if err := build(nil); err != nil {
		return err
	}
	bin := filepath.Join("bin", exe("anvil"))
	cfg := filepath.Join(".dev", "anvil.toml")
	if _, err := os.Stat(cfg); errors.Is(err, os.ErrNotExist) {
		if err := run(nil, bin, "init", "--config", cfg); err != nil {
			return err
		}
	}
	return run(nil, bin, append([]string{"serve", "--config", cfg}, args...)...)
}

func docs(args []string) error {
	if _, err := os.Stat(filepath.Join("site", "node_modules")); err != nil {
		if err := run(nil, "npm", "--prefix", "site", "ci"); err != nil {
			return err
		}
	}
	return run(nil, "npm", "--prefix", "site", "run", "build")
}
