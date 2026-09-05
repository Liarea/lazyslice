# First-run study: lazygit, lazydocker, k9s

Phase 3 research, written 2026-09-05. Discharges research/OPEN_QUESTIONS.md item 5, which forbids writing the first-run ADR from research/SQLIT_STUDY.md alone, and item 7, which asks for a Docker context resolution order taken from working code rather than from memory. docs/adr/008-first-run.md is written from this document.

**Method.** Cloned all three repositories into `/private/tmp/claude-501/lazyslice-scratch/` on 2026-09-05 and read them at these commits. Every permalink below pins that commit, so a reader sees the lines I read.

| Repository | Commit | Commit date |
|---|---|---|
| [`jesseduffield/lazygit`](https://github.com/jesseduffield/lazygit) | [`c07f4d381b90419583b7ce04f87379654d983ebc`](https://github.com/jesseduffield/lazygit/commit/c07f4d381b90419583b7ce04f87379654d983ebc) | 2026-09-05 |
| [`jesseduffield/lazydocker`](https://github.com/jesseduffield/lazydocker) | [`7e7aadc2071d58031bf2daafca1fbd4093efc23f`](https://github.com/jesseduffield/lazydocker/commit/7e7aadc2071d58031bf2daafca1fbd4093efc23f) | 2026-04-19 |
| [`derailed/k9s`](https://github.com/derailed/k9s) | [`84852e6e47ae830b30c921ffa5838443387e096e`](https://github.com/derailed/k9s/commit/84852e6e47ae830b30c921ffa5838443387e096e) | 2026-09-03 |

Shallow clones (`--depth 1`), so no history claims are made below; every claim is about the state of the code at the commit named.

**Scope.** Five questions, asked of each tool: what it detects on a launch with no configuration; what it asks; how it resolves an ambiguous endpoint (Docker context, kube context); how it degrades when nothing is found; how it surfaces keybindings. §6 collects what the three agree on and §7 lists the eight findings that reach ADR-008.

**Contents**

1. [lazygit: the only one of the three that asks a question](#1-lazygit-the-only-one-of-the-three-that-asks-a-question)
2. [lazydocker: Docker endpoint resolution, in code](#2-lazydocker-docker-endpoint-resolution-in-code)
3. [lazydocker: compose as a naming source](#3-lazydocker-compose-as-a-naming-source)
4. [k9s: zero questions, a resolution order, and a running TUI with a warning](#4-k9s-zero-questions-a-resolution-order-and-a-running-tui-with-a-warning)
5. [Keybinding discoverability in all three](#5-keybinding-discoverability-in-all-three)
6. [What the three agree on](#6-what-the-three-agree-on)
7. [Findings that reach ADR-008](#7-findings-that-reach-adr-008)
8. [What none of them answers](#8-what-none-of-them-answers)

---

## 1. lazygit: the only one of the three that asks a question

lazygit's first-launch decision is structurally the same shape as lazyslice's Q1: the tool cannot do its job, the missing thing can be created, and creating it is a side effect the user may not want. lazygit's answer is in `setupRepo` ([`pkg/app/app.go` L194-L282](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L194-L282)).

**An explicit override skips setup entirely, it does not merely rank first.** The first three lines of the function:

```go
if env.GetGitDirEnv() != "" {
    // we've been given the git dir directly. Skip setup
    return false, nil
}
```

([`pkg/app/app.go` L197-L200](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L197-L200).) `GIT_DIR` set means no detection runs, no question is asked, and no fallback is consulted. This is not "the flag is rung 0 of a ladder"; it is "the ladder does not run".

**The question is a plain stdin prompt before the TUI exists, and its default is No.**

```go
case "prompt":
    // Offer to initialize a new repository in current directory.
    fmt.Print(app.Tr.CreateRepo)
    response, _ := bufio.NewReader(os.Stdin).ReadString('\n')
    shouldInitRepo = (strings.Trim(response, " \r\n") == "y")
```

([`pkg/app/app.go` L216-L220](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L216-L220).) The prompt text is `"Not in a git repository. Create a new git repository? (y/N): "` ([`pkg/i18n/english.go` L1629](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/i18n/english.go#L1629)). Pressing Enter creates nothing. Only the literal `y` proceeds. A second, non-blocking question follows on the yes branch — `"Branch name? (leave empty for git's default): "` ([`pkg/i18n/english.go` L1632](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/i18n/english.go#L1632)) — which is a parameter of the thing being created, asked only once the user has said yes, and which has a working default.

**The question's behaviour is a four-valued config key, not a boolean.**

```go
NotARepository string `yaml:"notARepository" jsonschema:"enum=prompt,enum=create,enum=skip,enum=quit"`
```

([`pkg/config/user_config.go` L38](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/user_config.go#L38), default `"prompt"` at [L993](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/user_config.go#L993).) The four values map onto the switch at [L215-L237](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L215-L237): `prompt` asks, `create` does it unasked, `skip` falls through to the fallback, `quit` prints `"Error: must be run inside a git repository"` and exits 1. An unrecognised value is itself an error with its own message ([L1634](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/i18n/english.go#L1634)) and exit 1, rather than a silent fallback to the default.

**Degradation is a fallback chain ending in a message and a non-zero exit.** Declining the question does not start the TUI empty. `openRecentRepo` walks the machine-local recent-repository list, `chdir`s into the first entry that still is a repository, and returns true ([`pkg/app/app.go` L171-L191](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L171-L191)). Only when that also fails does lazygit print `"Must open lazygit in a git repository. No valid recent repositories. Exiting."` and `os.Exit(1)` ([`pkg/app/app.go` L254-L259](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L254-L259); message at [`pkg/i18n/english.go` L1633](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/i18n/english.go#L1633)). The bare-repo case at [L263-L278](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/app.go#L263-L278) has the same shape: a `(y/n)` prompt, then the same recent-repo fallback, then the same message and exit.

**Known daemon-level errors are translated, not stack-traced.** A registry of `{originalError, newError}` pairs maps `"fatal: not a git repository"` and `"getwd: no such file or directory"` onto written messages ([`pkg/app/errors.go` L25-L35](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/app/errors.go#L25-L35)). This is the same pattern research/SQLIT_STUDY.md §5.7 adopted from sqlit, arrived at independently.

**Config-file creation is best-effort and silent on permission errors.** `findOrCreateConfigDir` is `os.MkdirAll(folder, 0o755)` ([`pkg/config/app_config.go` L139-L141](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/app_config.go#L139-L141)); the config file itself is created empty when missing, and a permission error is swallowed:

```go
case ConfigFilePolicyCreateIfMissing:
    file, err := os.Create(path)
    if err != nil {
        if os.IsPermission(err) {
            // apparently when people have read-only permissions they prefer us to fail silently
            continue
        }
```

([`pkg/config/app_config.go` L182-L190](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/app_config.go#L182-L190).) The lookup order is `$CONFIG_DIR`, then the legacy `jesseduffield/lazygit/` path under XDG, then `lazygit/` under XDG, then a computed default under `xdg.ConfigHome` ([`pkg/config/app_config.go` L738-L757](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/app_config.go#L738-L757)), with state (recent repos) under `xdg.StateFile` ([L762-L772](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/app_config.go#L762-L772)). Note the asymmetry that matters for lazyslice: lazygit's unwritable-config case is silent because nothing is lost; lazyslice's unwritable-`.gitignore` case is a masking-key decision and ARCHITECTURE.md §9 "The repository" already makes it loud. The precedent supports degrading rather than failing; it does not support degrading quietly when a safety property changes.

## 2. lazydocker: Docker endpoint resolution, in code

This is the direct answer to research/OPEN_QUESTIONS.md item 7. lazydocker resolves the daemon endpoint in `determineDockerHost` ([`pkg/commands/docker.go` L526-L585](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L526-L585)), whose own doc comment states the order:

```go
// determineDockerHost tries to the determine the docker host that we should connect to
// in the following order of decreasing precedence:
//   - value of "DOCKER_HOST" environment variable
//   - host retrieved from the current context (specified via DOCKER_CONTEXT)
//   - "default docker host" for the host operating system, otherwise
```

The implementation, in order:

1. `DOCKER_HOST` if non-empty, returned immediately ([L533-L536](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L533-L536)).
2. `DOCKER_CONTEXT` if set; otherwise `cliconfig.Load(cliconfig.Dir()).CurrentContext`, that is the `currentContext` field of `~/.docker/config.json` ([L538-L545](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L538-L545)).
3. An empty context name, **or the literal name `default`**, short-circuits to the platform default socket, with the reason in the code: "On some systems (windows) `default` is stored in the docker config as the currentContext" ([L547-L553](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L547-L553)).
4. Otherwise the context store is opened at `cliconfig.ContextStoreDir()` and the named context's metadata read; the `docker` endpoint's `Host` is returned ([L555-L576](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L555-L576)).
5. A context that exists but whose host is empty — creatable with `docker context create foo --docker "host="`, which the code comments call out by name — falls back to the platform default socket rather than to an empty endpoint ([L577-L584](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L577-L584)).

The platform defaults are per-OS build-tagged constants: `unix:///var/run/docker.sock` ([`pkg/commands/docker_host_unix.go` L5-L7](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker_host_unix.go#L5-L7)) and `npipe:////./pipe/docker_engine` ([`pkg/commands/docker_host_windows.go` L3-L5](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker_host_windows.go#L3-L5)).

Three further details worth taking:

**`client.FromEnv` is deliberately not used**, and the reason is not the one research/SQLIT_STUDY.md §5.1 gives:

```go
// We avoid using client.FromEnv because it includes WithVersionFromEnv() which
// sets manualOverride=true when DOCKER_API_VERSION is set, preventing API version
// negotiation even when WithAPIVersionNegotiation() is specified.
```

([`pkg/commands/docker.go` L88-L99](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L88-L99), referencing [lazydocker issue 715](https://github.com/jesseduffield/lazydocker/issues/715).) The client is built from exactly three options — `WithTLSClientConfigFromEnv()`, `WithAPIVersionNegotiation()`, `WithHost(dockerHost)`. So `FromEnv` is wrong for two independent reasons: it ignores contexts (the SQLIT_STUDY reason) *and* it pins the API version (this one). There is a regression test for the second, [`pkg/commands/docker_test.go` L14-L60](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker_test.go#L14-L60).

**That third option and that reason are `docker/docker`'s, and lazyslice pins a different client.** lazydocker's `go.mod` pins `github.com/docker/docker v28.5.2+incompatible`; ARCHITECTURE.md §13 pins `github.com/moby/moby/client v0.6.0` and lists `docker/docker` as explicitly not a dependency. Read at `v0.6.0` in the module cache on 2026-09-05, the pinned client differs on both points:

- `WithAPIVersionNegotiation()` is documented `Deprecated: API-version negotiation is now enabled by default and this options is now a no-op` and its body is `return nil` (`client_options.go` L381-L395). Copying lazydocker's three-option call would therefore add a call that does nothing and fails `staticcheck`'s SA1019, which `.golangci.yml` enables.
- `manualOverride` does not exist in the module at all. The equivalent mechanism is `WithAPIVersionFromEnv`, which sets `cfg.envAPIVersion` from `DOCKER_API_VERSION` (`client_options.go` L357-L369); a non-empty `envAPIVersion` makes the constructor call `setAPIVersion` (`client.go` L226-L227), and `setAPIVersion` does `negotiated.Store(true)` (`client.go` L365-L371). `FromEnv` applies that option (`client_options.go` L90-L102), so the *effect* lazydocker documents is real in the pinned client too — reached by a different code path, and with the remedy being "do not call `FromEnv`" rather than "add `WithAPIVersionNegotiation()`".

The finding lazyslice takes from lazydocker is therefore the diagnosis, not the call. ADR-008 §3 states the two-option construction against the pinned client.

**An `ssh://` endpoint is not passed to the client directly.** When the resolved host starts with `ssh://`, lazydocker writes it back into the environment and lets an SSH handler replace it with a local tunnelled unix socket, re-reading `DOCKER_HOST` afterwards ([`pkg/commands/docker.go` L103-L127](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L103-L127); the handler is [`pkg/commands/ssh/ssh.go` L45-L120](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/ssh/ssh.go#L45-L120)).

**A failed connection is a one-line message and exit 0, not a stack trace.** `main` checks `client.IsErrConnectionFailed(err)` and prints `"connection to docker client failed. You may need to restart the docker client"`, then `os.Exit(0)` ([`main.go` L86-L101](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/main.go#L86-L101); message at [`pkg/i18n/english.go` L158](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/i18n/english.go#L158)). A permission-denied socket gets its own translated message that ends in a command-shaped instruction and a link: `"Can't access docker socket at: unix:///var/run/docker.sock\nRun lazydocker as root or read https://docs.docker.com/install/linux/linux-postinstall/"` ([`pkg/i18n/english.go` L162](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/i18n/english.go#L162), matched at [`pkg/app/app.go` L71-L88](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/app/app.go#L71-L88)). Note that the message hard-codes the default socket path rather than printing the endpoint that was actually resolved — a small defect worth not copying.

## 3. lazydocker: compose as a naming source

lazydocker never parses `docker-compose.yml`. It decides whether it is inside a compose project by *running* compose and checking the exit status, and it gets service names by asking compose for them.

**Project detection is `docker compose config --quiet` in the working directory.** The command template is `"{{ .DockerCompose }} config --quiet"` ([`pkg/config/app_config.go` L403](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/config/app_config.go#L403)), run at startup; a non-zero exit sets `InDockerComposeProject = false` and logs a warning rather than failing ([`pkg/commands/docker.go` L145-L156](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L145-L156)). Whether `docker compose` or the legacy `docker-compose` binary is used is itself probed, by running `docker compose version` and falling back ([L176-L186](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L176-L186)).

**Service names come from `docker compose config --services`, one name per line, and nothing else.** No ports, no environment, no connection detail ([`pkg/commands/docker.go` L431-L460](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L431-L460)).

**Everything authoritative comes off the running container's labels.**

```go
newContainer.ServiceName = ctr.Labels["com.docker.compose.service"]
newContainer.ProjectName = ctr.Labels["com.docker.compose.project"]
newContainer.ContainerNumber = ctr.Labels["com.docker.compose.container"]
newContainer.OneOff = ctr.Labels["com.docker.compose.oneoff"] == "True"
```

([`pkg/commands/docker.go` L419-L422](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L419-L422).) The display name prefers a `name` label, then the first entry of `Names` with its leading `/` stripped, then the container ID ([L409-L418](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L409-L418)).

**Stopped containers are listed, not hidden.** `ContainerList` is called with `container.ListOptions{All: true}` ([`pkg/commands/docker.go` L374-L381](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L374-L381)), which is the same posture as ARCHITECTURE.md §9's rung 4.

**Two divergences from what ARCHITECTURE.md §9 assumes, both worth recording.**

*First*, lazydocker does **not** use `com.docker.compose.project.working_dir` anywhere — grepping the tree for `com.docker.compose` returns only the four labels above. It identifies "this directory's project" by matching the service names compose reports for the cwd against the `com.docker.compose.service` labels of running containers, taking that container's `com.docker.compose.project` as the local project, and falling back to the basename of the working directory when no container matches ([`pkg/commands/docker.go` L245-L268](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/commands/docker.go#L245-L268)). The comment gives the reason the basename is only a fallback: "We match compose service names against container labels to handle cases where the project name differs from the directory name (e.g. a `name:` directive in the compose file)". ARCHITECTURE.md §9's `working_dir` filter avoids both the subprocess and the mismatch, and is the better mechanism — but lazydocker's fallback chain is evidence that *some* fallback is needed when the label match yields nothing, and lazyslice's ladder currently states one ("then any") without saying what the header prints when it fires.

*Second*, lazydocker normalises a published bind address before using it: `0.0.0.0` becomes `localhost` ([`pkg/gui/containers_panel.go` L552-L568](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/gui/containers_panel.go#L552-L568)), and a port with an empty `IP` is treated as unpublished and skipped. That is the address-family normalisation research/SQLIT_STUDY.md §5.1 asks lazyslice to do at de-duplication time, present in a shipped tool.

**But note exactly where it sits, because it bounds what it is precedent for.** The whole of it is `openContainerInBrowser`: it takes the first published port of the container the user has selected and hands `http://<ip>:<port>/` to the OS's link opener. It never asks whether the daemon it is talking to is the local one, and it does not have to — the worst outcome of being wrong is a browser tab that does not load. lazyslice's use is the opposite: the normalised address becomes `Candidate.Local`, which is the input to the locality rule of the gate that decides which database gets dropped and recreated (THREAT_MODEL.md T2). So this is precedent for the *rewrite* and for treating an empty `IP` as unpublished, and it is precedent for nothing about locality. ADR-008 §4 supplies the precondition — the rewrite is legitimate only when the resolved Docker endpoint is local — which is a decision lazyslice has to make on its own evidence.

**Config on first launch is created empty and never prompted for.** `findOrCreateConfigDir` does `os.MkdirAll(folder, 0o755)` ([`pkg/config/app_config.go` L545-L554](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/config/app_config.go#L545-L554)) and `loadUserConfig` creates a zero-byte `config.yml` if absent, then unmarshals it over the compiled-in defaults ([L562-L587](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/config/app_config.go#L562-L587)). The directory is `$CONFIG_DIR` if set, else the legacy `jesseduffield` XDG vendor path if it exists, else the plain XDG path ([L527-L543](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/config/app_config.go#L527-L543)). lazydocker asks **no** questions at any point: grepping the tree for `bufio.NewReader(os.Stdin)` and `fmt.Scan` returns nothing outside tests.

## 4. k9s: zero questions, a resolution order, and a running TUI with a warning

k9s has the largest ambiguity surface of the three — a kubeconfig can hold dozens of contexts — and asks nothing. There is no stdin read in `cmd/` or `internal/` outside tests.

**Context resolution order.** `Config.Refine` takes the `--context` flag when set and otherwise asks the kubeconfig for its current context ([`internal/config/config.go` L89-L146](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/config.go#L89-L146)):

```go
if isStringSet(flags.Context) {
    if _, err := c.K9s.ActivateContext(*flags.Context); err != nil {
        return fmt.Errorf("k8sflags. unable to activate context %q: %w", *flags.Context, err)
    }
} else {
    n, err := cfg.CurrentContextName()
    ...
    if n != "" {
        _, err = c.K9s.ActivateContext(n)
        ...
    } else {
        slog.Debug("No context set, skipping context activation")
    }
}
```

`CurrentContextName` is the same two-step at the client layer — the `--context` flag, else `RawConfig().CurrentContext`, which is the standard `client-go` loader over `$KUBECONFIG` and `~/.kube/config` ([`internal/client/config.go` L156-L166](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/client/config.go#L156-L166)); `CurrentClusterName` layers `--cluster` over the same lookup ([L132-L154](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/client/config.go#L132-L154)). Namespace resolution is a third, independent ladder in the same function: `--all-namespaces`, else `--namespace`, else the active context's saved namespace, else the literal default ([`internal/config/config.go` L124-L143](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/config.go#L124-L143)). The shape to copy is that each ambiguous input has its own flag and its own stated fallback chain, and no chain ends in a prompt.

**Errors are accumulated, not returned.** `loadConfiguration` joins every failure into one `errs` and returns the config anyway ([`cmd/root.go` L133-L177](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/cmd/root.go#L133-L177)); `run` then discards it unless a context was actually configured:

```go
cfg, err := loadConfiguration()
if err != nil {
    // Only warn if there's an actual context configured
    if cfg != nil && cfg.K9s.ActiveContextName() != "" {
        slog.Warn("Fail to load global/context configuration", slogs.Error, err)
    }
}
```

([`cmd/root.go` L108-L114](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/cmd/root.go#L108-L114).) The TUI then starts regardless.

**Connectivity is probed only when there is something to probe, and a failure does not stop the launch.**

```go
if k9sCfg.K9s.ActiveContextName() != "" {
    if !conn.CheckConnectivity() { ... errors.Join(...) }
    if !conn.ConnectionOK() { slog.Warn("💣 Kubernetes connectivity toast!") ... }
    else { slog.Info("✅ Kubernetes connectivity OK") }
} else {
    slog.Info("No context configured")
}
```

([`cmd/root.go` L157-L170](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/cmd/root.go#L157-L170).) Three further places guard on the same "no context configured" condition and skip work rather than erroring: `SetActiveNamespace` ([`internal/config/config.go` L196-L201](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/config.go#L196-L201)), `Save` ([L296-L302](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/config.go#L296-L302)) and the context-activation branch quoted above.

**Config is written on the way in, and validated against a schema on read.** `Load` creates the file when it does not exist, then validates the bytes against an embedded JSON schema and unmarshals; a schema failure is joined into the returned error rather than aborting ([`internal/config/config.go` L271-L293](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/config.go#L271-L293)). Locations: `$K9S_CONFIG_DIR` wins outright and puts everything under one directory; otherwise XDG, with logs and screen dumps under `xdg.StateFile` rather than the config dir ([`internal/config/files.go` L89-L200](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/files.go#L89-L200)). Directory-creation failures for the optional subdirectories are `slog.Warn` and continue ([L131-L153](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/config/files.go#L131-L153)).

**A panic during init is caught and rendered.** `run` installs a deferred `recover` that logs the stack to the log file, prints the logo and a one-line `Boom!!` message to the terminal, and does not dump a stack trace at the user ([`cmd/root.go` L93-L101](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/cmd/root.go#L93-L101)).

## 5. Keybinding discoverability in all three

All three converge on the same two-layer arrangement research/SQLIT_STUDY.md §5.8 proposed, and two of them add a CI drift check.

**Layer 1: a footer computed from current state, not a static string.** lazygit builds the options bar from the current context's bindings unioned with the global bindings that the context has not shadowed, then filters on two per-binding predicates — `DisplayOnScreen` and `!IsDisabled()` — and prepends mode-specific entries (cherry-picking, bisect, rebase, patch-building) with their own colours ([`pkg/gui/options_map.go` L37-L105](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/gui/options_map.go#L37-L105)). k9s does the same from the other direction: the menu is re-hydrated on every view-stack push, pop and top change from that component's `Hints()`, and each hint carries a `Visible` flag the renderer honours ([`internal/ui/menu.go` L59-L130](https://github.com/derailed/k9s/blob/84852e6e47ae830b30c921ffa5838443387e096e/internal/ui/menu.go#L59-L130)).

**Layer 2: a generated reference, plus CI that fails on drift.** lazygit generates `docs-master/keybindings/Keybindings_<lang>.md` from the binding table via `go generate`, and stamps each file with the command that regenerates it: `"_This file is auto-generated. To update, make the changes in the pkg/i18n directory and then run `go generate ./...` from the project root._"` ([`pkg/cheatsheet/generate.go` L46-L83](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/cheatsheet/generate.go#L46-L83)). CI enforces it with `go generate ./... && git diff --quiet || (git status -s; echo "Auto-generated files not up to date. …" && exit 1)` ([`.github/workflows/ci.yml` L172-L174](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/.github/workflows/ci.yml#L172-L174)). lazydocker enforces the same property differently: `cheatsheet.Check()` regenerates into a temporary directory, diffs it against the committed files with `difflib.WriteUnifiedDiff`, prints the diff and the exact regeneration command, and exits 1 ([`pkg/cheatsheet/validate.go` L16-L59](https://github.com/jesseduffield/lazydocker/blob/7e7aadc2071d58031bf2daafca1fbd4093efc23f/pkg/cheatsheet/validate.go#L16-L59)). lazydocker's variant is the better model for lazyslice because it shows the developer *what* drifted rather than only that something did.

**`?` is the binding for the full list** in lazygit (`OptionMenu: Keybinding{"?"}`, [`pkg/config/user_config.go` L1033](https://github.com/jesseduffield/lazygit/blob/c07f4d381b90419583b7ce04f87379654d983ebc/pkg/config/user_config.go#L1033)), which is the key ARCHITECTURE.md §9 already assigns to "show why this root table".

## 6. What the three agree on

| Property | lazygit | lazydocker | k9s |
|---|---|---|---|
| Questions on a launch with nothing configured | one, only when the missing thing must be created | none | none |
| Default of the creating question | **No** (`(y/N)`, only literal `y` proceeds) | n/a | n/a |
| That question's behaviour is configurable | yes, four-valued (`prompt`/`create`/`skip`/`quit`) | n/a | n/a |
| Explicit override skips detection entirely | yes (`GIT_DIR`) | yes (`DOCKER_HOST` short-circuits) | yes (`--context` short-circuits) |
| Ambiguous endpoint resolved by a written order | n/a | yes, 5 steps, in a doc comment | yes, flag → kubeconfig current → none |
| Empty/absent endpoint falls back to a platform default | n/a | yes, per-OS constant | no — it starts with nothing active |
| Nothing found → | fallback chain, then message + exit 1 | one-line message + exit 0 | TUI starts, warning logged |
| Config file created unasked on first launch | yes, empty | yes, empty | yes, from defaults |
| Config-write failure | warn/continue (silent on permission) | error at startup | warn and continue |
| Footer computed from current state | yes | yes | yes |
| Generated keybindings reference | yes | yes | no |
| CI drift check on that reference | yes (`git diff --quiet`) | yes (unified diff + command) | n/a |

The one place the three genuinely disagree is what "nothing found" costs. lazygit exits 1 because it cannot do anything useful; lazydocker exits 0 because the daemon may simply be off and that is not the user's mistake; k9s runs anyway because a context can be picked from inside the TUI. lazyslice's §9 answer — exit 3 with the ladder printed and a command — is lazygit's posture, and it is the right one, because unlike k9s there is no in-TUI recovery: a source cannot be picked from a list that is empty.

## 7. Findings that reach ADR-008

1. **The Docker resolution order in ARCHITECTURE.md §9 is one step short.** ARCHITECTURE.md §8's `--docker-host` row reads "`DOCKER_HOST`, context, default sockets"; the shipped implementation has five steps, and the two missing ones both bite: `DOCKER_CONTEXT` as an override of `~/.docker/config.json`'s `currentContext`, and the literal context name `default` short-circuiting to the platform socket without touching the context store ([§2](#2-lazydocker-docker-endpoint-resolution-in-code)). A context whose host is empty is a third case that must fall back rather than produce an empty endpoint. ADR-008 states all five.

2. **`client.FromEnv` is wrong for a second reason nobody had recorded — and lazydocker's remedy does not port.** `FromEnv` pins the API version when `DOCKER_API_VERSION` is set and thereby defeats negotiation; lazydocker attributes this to `manualOverride` and answers it with a third client option ([§2](#2-lazydocker-docker-endpoint-resolution-in-code)). Both of those belong to `github.com/docker/docker/client`, which lazydocker pins and lazyslice does not. In the pinned `github.com/moby/moby/client v0.6.0` the effect arrives through `WithAPIVersionFromEnv` → `setAPIVersion` → `negotiated.Store(true)`, and `WithAPIVersionNegotiation()` is a deprecated no-op that `staticcheck` would reject. ADR-008 §3 names the **two** options to use and restates the reason against the pinned client, so `internal/discover/dockerctx` is neither written from `FromEnv` nor written from a call that does nothing.

3. **lazygit's creating question defaults to No; lazyslice's Q1 defaults to Yes.** This is a real disagreement, not an oversight: `git init` writes into the user's own project directory and is what they will be committing into, whereas Q1 creates a disposable, lazyslice-named container that exists to be written into, which is exactly the asymmetry research/SQLIT_STUDY.md §6 "Why Q1 defaults to Yes only with a human watching" argues. ADR-008 keeps Yes, records the contrary precedent, and takes lazygit's *other* half: the prompt string must show the default in its brackets, and a bare Enter must be the only thing the default applies to.

4. **Read the answer from the terminal, not from stdin.** lazygit reads with `bufio.NewReader(os.Stdin)` ([§1](#1-lazygit-the-only-one-of-the-three-that-asks-a-question)), so `echo something | lazygit` in a non-repo directory consumes a line of piped data as an answer. For lazyslice this is a safety question, not an ergonomics one, because Q1's yes branch creates a container and Q4's answer is a password. ADR-008 requires the prompt to open the controlling terminal and to treat "no controlling terminal" as `--yes`.

5. **An explicit endpoint should skip discovery, not win it.** All three tools short-circuit on the explicit value ([§6](#6-what-the-three-agree-on)). ARCHITECTURE.md §2's `Provenance` has `FromFlag` as a value alongside the rungs, which reads as "another candidate"; it is not, and the ladder must not run when both endpoints are given.

6. **Compose is a naming source in practice, not just in principle.** lazydocker runs compose for *names* and takes every connection-bearing fact off container labels and live port bindings ([§3](#3-lazydocker-compose-as-a-naming-source)). This is ADR-004's "`docker-compose.yml` is a naming source only" independently confirmed in a shipped tool — and lazydocker goes further than lazyslice needs to by shelling out, which lazyslice must not do, because ARCHITECTURE.md §8 fixes the subprocess set at exactly two.

7. **`0.0.0.0` must normalise to loopback before de-duplication — under a precondition lazydocker does not need and does not supply.** lazydocker does the rewrite in a browser-link helper ([§3](#3-lazydocker-compose-as-a-naming-source)), where being wrong costs a dead tab. ARCHITECTURE.md §12 puts loopback normalisation in `internal/dsn`, but §9's ladder does not say that a container publishing on `0.0.0.0:5432` and a `DATABASE_URL` naming `localhost:5432` are the same candidate. ADR-008 §4 says it, and adds what the precedent cannot: `0.0.0.0` means "every interface of the machine running the daemon", so the rewrite is sound only when that machine is this one. ADR-008 §3 makes a local endpoint the precondition of rungs 3 and 4 for that reason.

8. **The keybindings/flags reference needs the diff, not just the exit code.** lazydocker prints a unified diff and the regeneration command; lazygit prints `git status -s` ([§5](#5-keybinding-discoverability-in-all-three)). ARCHITECTURE.md §12 already has `tools/docgen` and a docs-drift CI job; ADR-008 records which of the two behaviours that job has, because it is a first-run-quality decision — the developer who hits it is a contributor on their first run.

## 8. What none of them answers

None of the three writes to anything destructible. lazygit writes into a repository the user already has, lazydocker only reads the daemon plus running compose subcommands, and k9s writes only its own config. So no precedent here bears on:

- the target eligibility gate (ARCHITECTURE.md §9 rules 1-5), which has no analogue in any of the three;
- what to do when the same endpoint appears as both source and target;
- the masking-key file and its `.gitignore` protection;
- truncating a marked target without asking.

Those remain decided by ADR-005 and ADR-004 on the evidence in research/SQLIT_STUDY.md and research/COMPLAINTS.md, and item 2 of research/OPEN_QUESTIONS.md — "the target side has no evidence base" — is **not** discharged by this study. It stays open. This study discharges items 5 and 7 only.
