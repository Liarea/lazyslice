# sqlit study: how a zero-config first run is actually built

Phase 1 research. Written 2026-09-05.

**Method.** Cloned [`Maxteabag/sqlit`](https://github.com/Maxteabag/sqlit) into `/private/tmp/claude-501/lazysnap-scratch/sqlit` and read it at commit `2a09c28e5402da581da7cb38e581468bafdb9064` (2026-09-03). That commit is two commits past the `v1.6.3` tag, and `git diff --stat v1.6.3 HEAD` touches only four files — `sqlit/domains/connections/providers/mssql/adapter.py`, `sqlit/shared/ui/widgets_tables.py`, and two test files. Every file quoted below is byte-identical between `v1.6.3` and the commit I read, so **all permalinks in this document point at `v1.6.3`** and can be trusted to show what I saw.

I also read [lazygit's README](https://github.com/jesseduffield/lazygit/blob/master/README.md) and its [auto-generated keybindings reference](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md), and the [Show HN thread](https://news.ycombinator.com/item?id=46276002) that launched sqlit — which is the single best record of what real users hit on their own first run, because the author answered nearly every complaint in it.

Reddit is unreachable from this environment. Community evidence comes from Hacker News (via the [Algolia API](https://hn.algolia.com/api)), the GitHub REST API, GitHub issues and PRs, and official vendor documentation.

**Contents**

1. [sqlit as of September 2026](#1-sqlit-as-of-september-2026)
2. [How sqlit achieves a zero-config first run](#2-how-sqlit-achieves-a-zero-config-first-run)
3. [What made sqlit easy to adopt](#3-what-made-sqlit-easy-to-adopt)
4. [What lazysnap should not copy](#4-what-lazysnap-should-not-copy)
5. [lazysnap's equivalent, step by step](#5-lazysnaps-equivalent-step-by-step)
6. [The complete question catalogue](#6-the-complete-question-catalogue)
7. [Gaps and unverified points](#7-gaps-and-unverified-points)

---

## 1. sqlit as of September 2026

| Fact | Value | Source |
|---|---|---|
| Repository | `Maxteabag/sqlit` — MIT, Python, not archived | [GitHub API `repos/Maxteabag/sqlit`](https://api.github.com/repos/Maxteabag/sqlit), fetched 2026-09-05 |
| Created | 2025-12-13 | same |
| Stars / forks / open issues / watchers | 4,801 / 162 / 15 / 18 | same |
| Last push | 2026-09-03 | same |
| Commits / contributors / tags | 559 / 38 / 62 | `git rev-list --count HEAD`, `git shortlog -sn`, `git tag` on the clone |
| Latest release | `v1.6.3`, published 2026-08-27 | [releases API](https://api.github.com/repos/Maxteabag/sqlit/releases) |
| PyPI distribution | `sqlit-tui` 1.6.3, `requires-python >= 3.10` | [PyPI JSON API](https://pypi.org/pypi/sqlit-tui/json) |
| Console scripts | **two**: `sqlit` and `sqlit-tui`, both `sqlit.cli:main` | [`pyproject.toml` L172-174](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L172-L174) |
| Hard runtime dependencies | `textual[syntax]`, `textual-fastdatatable`, `pyperclip`, `keyring`, `docker`, `sqlparse` | [`pyproject.toml` L28-35](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L28-L35) |
| Launch | Show HN, 2025-12-15, 190 points, 42 comments | [news.ycombinator.com/item?id=46276002](https://news.ycombinator.com/item?id=46276002) |

Positioning, verbatim from the README: "The lazygit of SQL databases", and "Connect and query your database from your terminal in seconds." ([README](https://github.com/Maxteabag/sqlit/blob/v1.6.3/README.md))

The claim that matters to us is the FAQ answer: sqlit is "inspired by [lazygit](https://github.com/jesseduffield/lazygit) - you can just jump in and there's no need for external documentation." ([README FAQ](https://github.com/Maxteabag/sqlit/blob/v1.6.3/README.md)) That is the same bet lazysnap is making, one layer down the stack.

**Note the asymmetry with lazygit itself.** For reference, lazygit is at 81,999 stars, latest release `v0.64.1` on 2026-08-12 ([GitHub API](https://api.github.com/repos/jesseduffield/lazygit)). Its README says only "Call `lazygit` in your terminal inside a git repository" and then points at [external keybinding docs](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings) and a [config guide](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md). lazygit gets away with zero config because **git already put the config on disk** — a repository is self-describing, and `lazygit` run in the wrong directory simply has nothing to show.

sqlit has no such gift. A directory does not tell you which database it belongs to. So sqlit had to **manufacture** the missing context by inspecting the machine. That manufacturing is what the rest of section 2 is about, and it is exactly lazysnap's problem, with a harder safety constraint bolted on.

---

## 2. How sqlit achieves a zero-config first run

### 2.0 What the first run actually is

`sqlit` with no arguments falls through to `args.command is None` in [`sqlit/cli.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/cli.py#L842), builds `RuntimeConfig` and `AppServices`, and launches the Textual app with `startup_connection=None`. There is no config file check that can fail, no first-run wizard, and no "run `sqlit init` first". The connection picker opens, and while it is rendering it fires a background worker that scans Docker:

```python
def load_async(self) -> None:
    self._state.loading = True
    self._screen._rebuild_list()
    self._screen.run_worker(self._detect_worker, thread=True)
```

([`controllers/docker.py` L21-28](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/controllers/docker.py#L21-L28))

The scan is off the UI thread, the list renders immediately with a loading marker, and containers appear when they appear. **The first screen is never blocked on discovery.** That is the single most important structural decision in the whole first-run design, and it costs almost nothing to copy.

### 2.1 Docker container detection — the code path

This is the mechanism that makes `sqlit` useful on a machine it has never seen. It lives in one 419-line file, [`sqlit/domains/connections/discovery/docker_detector.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py), and every design choice in it is one lazysnap will also have to make.

#### Step 1 — is Docker there, and in which of four ways is it not?

```python
def get_docker_status() -> DockerStatus:
    """Check if Docker is available and running."""
    try:
        import docker  # pyright: ignore[reportMissingModuleSource]
    except ImportError:
        return DockerStatus.NOT_INSTALLED

    try:
        client = docker.from_env()
        client.ping()
        return DockerStatus.AVAILABLE
    except Exception as e:
        error_str = str(e).lower()
        if "permission denied" in error_str:
            return DockerStatus.NOT_ACCESSIBLE
        if "connection refused" in error_str or "connect" in error_str:
            return DockerStatus.NOT_RUNNING
        return DockerStatus.NOT_RUNNING
```

([lines 86-107](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L86-L107))

Three things to steal, one to fix.

**Steal 1: failure is a four-valued enum, not a boolean.** `AVAILABLE`, `NOT_RUNNING`, `NOT_INSTALLED`, `NOT_ACCESSIBLE` ([lines 18-24](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L18-L24)). Each value has a different remedy, so each renders a different sentence — in the TUI:

```python
if status == DockerStatus.NOT_INSTALLED:
    return "(Docker not detected)"
if status == DockerStatus.NOT_RUNNING:
    return "(Docker not running)"
if status == DockerStatus.NOT_ACCESSIBLE:
    return "(Docker not accessible)"
if status == DockerStatus.AVAILABLE and not containers:
    return "(no database containers found)"
return None
```

([`controllers/docker.py` L41-54](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/controllers/docker.py#L41-L54))

and, with the actual fix attached, in the CLI:

```python
elif status == DockerStatus.NOT_ACCESSIBLE:
    print("Error: Docker is not accessible (permission denied).")
    print("Try adding your user to the docker group or running with sudo.")
    return 1
```

([`cli/commands.py` L355-380](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/cli/commands.py#L355-L380))

Note the asymmetry: the TUI shows the *state*, the CLI shows the *state plus the remedy*. The TUI can afford to be terse because the user can see the rest of the screen; the CLI line is all the user gets. lazysnap should print the remedy in both, because a lazysnap run that finds no database has nothing else on screen to soften the dead end.

**Steal 2: inherit the user's Docker configuration rather than inventing a setting.** `docker.from_env()` is documented by docker-py as reading `DOCKER_HOST`, `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH` ([docker-py client docs](https://docker-py.readthedocs.io/en/stable/client.html)) — the same variables the Docker CLI honours ([`dockerd` reference](https://docs.docker.com/reference/cli/dockerd/)). A user on a remote or rootless daemon gets detection for free, with zero sqlit-specific configuration. The default is the unix socket, which is also why `NOT_ACCESSIBLE` needs its own message: the socket "requir[es] either `root` permission, or `docker` group membership" ([`dockerd` reference](https://docs.docker.com/reference/cli/dockerd/)).

**Steal 3: string-matching the exception is ugly and it works.** `"permission denied" in error_str` is not a stable API. But the alternative — enumerating docker-py's exception hierarchy — buys precision the user cannot perceive, and the fallback (`NOT_RUNNING`) is the most common true answer anyway. Do the same, and put the classifier behind one function so it is one place to fix.

**Fix: Docker *contexts* are not covered.** `DOCKER_HOST` is only one of the ways a developer points at a daemon. Docker's own documentation says a context is used for "all `docker` commands... unless overridden with environment variables such as `DOCKER_HOST` and `DOCKER_CONTEXT`, or on the command-line with the `--context` and `--host` flags", with contexts stored in "a `meta.json` file below `~/.docker/contexts/`" ([Docker contexts docs](https://docs.docker.com/engine/manage-resources/contexts/)). Neither docker-py's documented set nor Go's `client.FromEnv` — which the Go SDK documents as "the equivalent of using the WithTLSClientConfigFromEnv, WithHostFromEnv, and WithVersionFromEnv options", reading `DOCKER_HOST`, `DOCKER_API_VERSION`, `DOCKER_CERT_PATH`, `DOCKER_TLS_VERIFY` ([pkg.go.dev docker/docker/client](https://pkg.go.dev/github.com/docker/docker/client)) — resolves a context. **This matters for lazysnap because Go is the chosen language** (docs/BUILD_PLAN.md) and because Colima and OrbStack users on macOS routinely have a non-default context and an empty `DOCKER_HOST`. A lazysnap that reports "Docker not running" to an OrbStack user has failed its first run. See §5.1.

#### Step 2 — which containers are databases?

Image matching is delegated to whichever provider claims the image name:

```python
def _get_db_type_from_image(image_name: str) -> str | None:
    for db_type, detector in _iter_docker_detectors():
        if detector.match_image(image_name):
            return db_type
    return None
```

([lines 110-122](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L110-L122)), where `_iter_docker_detectors()` walks the provider catalog and collects every provider that declares one ([lines 74-83](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L74-L83)).

A detector is a **frozen dataclass of pure data** — no logic, no I/O, no subclassing:

```python
@dataclass(frozen=True)
class DockerDetector:
    image_patterns: tuple[str, ...]
    env_vars: dict[str, tuple[str, ...]]
    default_user: str | None = None
    default_database: str | None = None
    preferred_host: str = "localhost"
    default_user_requires_password: bool = False
    post_process: Callable[[DockerCredentials, Mapping[str, str]], DockerCredentials] | None = None

    def match_image(self, image_name: str) -> bool:
        image_lower = image_name.lower()
        return any(pattern in image_lower for pattern in self.image_patterns)
```

([`providers/docker.py` L20-32](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/docker.py#L20-L32))

Adding Postgres support to the detector is therefore nine lines of declaration inside the provider's spec:

```python
    docker_detector=DockerDetector(
        image_patterns=("postgres",),
        env_vars={
            "user": ("POSTGRES_USER",),
            "password": ("POSTGRES_PASSWORD",),
            "database": ("POSTGRES_DB",),
        },
        default_user="postgres",
    ),
```

([`providers/postgresql/provider.py` L27-36](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/postgresql/provider.py#L27-L36))

`match_image` is a plain lowercased substring test. `"postgres" in "docker.io/library/postgres:16-alpine"` is true; so is `"postgres" in "myorg/postgres-exporter:latest"` — a false positive that costs one failed connection attempt and nothing else. sqlit accepted the false positive to keep the rule one line. That is the correct trade for a picker; **it is the wrong trade for lazysnap**, where guessing wrong about which database is the source of a snapshot is not free.

#### Step 3 — credentials from the container's own environment

```python
    def get_credentials(self, env_vars: dict[str, str]) -> DockerCredentials:
        def get_first(values: tuple[str, ...]) -> str | None:
            for key in values:
                if key in env_vars:
                    return env_vars[key]
            return None

        user = get_first(self.env_vars.get("user", ()))
        password = get_first(self.env_vars.get("password", ()))
        database = get_first(self.env_vars.get("database", ())) or self.default_database
```

([`providers/docker.py` L34-51](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/docker.py#L34-L51))

The environment is read off the container's inspect payload: `container.attrs.get("Config", {}).get("Env", [])`, split on the first `=` ([lines 221-236](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L221-L236)).

This is the whole trick, and it is worth stating plainly: **the container already contains the credentials, because that is how the official images are configured.** `POSTGRES_PASSWORD` is a required variable for the official Postgres image. Anyone running Postgres in Docker has already told Docker the password, so a tool that can talk to the Docker daemon does not need to ask the user for it. No config file, no prompt, no wizard.

There is a small, careful touch here worth noting: a provider whose metadata says `requires_auth` is false gets an empty-string password rather than `None`, with the comment "This prevents the UI from prompting for a password" ([lines 313-317](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L313-L317)). `None` means "unknown, go ask"; `""` means "known to be empty". Conflating the two would produce a spurious prompt on a database that needs no password — a question that fails the zero-config promise for no gain. lazysnap has exactly one question to spend; this distinction is how you avoid spending it by accident.

#### Step 4 — the host and port, which is where the bodies are buried

Port resolution walks three fallbacks in order ([lines 279-298](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L279-L298)):

1. the host binding for the provider's declared default port (5432 for Postgres);
2. failing that, `_get_single_mapped_host_binding` — if the container publishes exactly one TCP host port, use it ([lines 156-170](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L156-L170)). This is what rescues `-p 55432:5432`;
3. failing that, if `HostConfig.NetworkMode == "host"`, take the single exposed TCP port, or the default.

Then the host itself, which took two merged pull requests to get right:

```python
def _resolve_published_host(preferred_host: str, bound_host: str | None) -> str:
    """Keep forced-TCP hosts on the address family published by Docker."""
    if preferred_host != "127.0.0.1" or not bound_host:
        return preferred_host
    try:
        address = ip_address(bound_host)
    except ValueError:
        return bound_host
    if address.is_unspecified:
        return "::1" if address.version == 6 else preferred_host
    return bound_host
```

([lines 173-184](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L173-L184))

The bug this fixes is described in the PR that introduced it: "Docker discovery found the mapped port and credentials, but `localhost` resolved to `::1` while SQL Server was published on IPv4 loopback, so the connection failed." ([PR #279, merged 2026-08-03](https://github.com/Maxteabag/sqlit/pull/279)). The follow-up, [PR #280](https://github.com/Maxteabag/sqlit/pull/280), generalised it: "retain Docker binding addresses when a provider forces TCP connections; map IPv4 and IPv6 wildcard publications to their matching loopback addresses".

**This is the single highest-value paragraph in this study for implementation.** Auto-detection that produces a connection string that does not connect is worse than no auto-detection, because the user now has to debug someone else's guess. `localhost` is not an address; it is a name that resolves differently on different machines. Use the address family Docker actually published on.

Two more robustness details from the same file, both worth copying verbatim in spirit:

- Image names are resolved best-effort through three fallbacks — `container.image.tags[0]`, then `Config.Image`, then `container.image.short_id` — each wrapped in its own `try/except` ([lines 199-218](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L199-L218)). Digest-pinned and tagless images are common in real compose files and would otherwise vanish from discovery.
- **Exited containers are detected too**, listed after running ones, with `connectable=False` ([`detect_database_containers`, lines 337-361](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L337-L361)). Showing a stopped container the user recognises, greyed out, is far better UX than an empty list — it converts "this tool is broken" into "ah, my database is down". lazysnap should do the same and offer to start it.

### 2.2 Connection saving

Discovery does not persist anything. A detected container becomes a config only when the user acts, via `container_to_connection_config` ([lines 384-419](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L384-L419)), which stamps `source="docker"` on the resulting `ConnectionConfig` — so the provenance of every saved connection is recorded, not inferred later.

Saving is deliberately forgiving ([`app/save_connection.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/save_connection.py)):

- `ensure_unique_name` appends `-2`, `-3`, … rather than rejecting a duplicate name (L34-43). No modal, no re-typing.
- `is_config_saved` treats a connection as already-saved if the **name** matches *or* the `(host, database)` pair matches (L21-31) — so the picker does not offer to save the same container twice under a different name.
- If the credential write fails but the connection record wrote, the result is still `saved=True` with a `warning` at `warning_severity="error"` (L69-81). The user keeps the connection; they are told the password did not persist. A partial success is reported as a partial success rather than being rolled back into a total failure.

Note that saving is **opt-in and one key**: `Save: s` only appears in the footer when the highlighted container is not already saved (§2.4). Discovery is ephemeral by default. lazysnap should hold the same line — a snapshot run writes `lazysnap.yml` because that is the artefact of the run, not because the tool wants to remember the user.

### 2.3 Keyring use

Storage is an abstract `CredentialsService` with three implementations chosen at runtime ([`app/credentials.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py)):

```python
def build_credentials_service(settings_store: Any | None = None) -> CredentialsService:
    settings = settings_store.load_all()
    allow_plaintext = settings.get(ALLOW_PLAINTEXT_CREDENTIALS_SETTING)

    # If user explicitly chose plaintext, use it regardless of keyring availability
    if allow_plaintext is True:
        return PlaintextFileCredentialsService()

    # Otherwise, try keyring first
    if is_keyring_usable():
        return KeyringCredentialsService()

    # Keyring unavailable - fall back to in-memory (not persisted)
    return PlaintextCredentialsService()
```

([L457-476](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L457-L476))

The ordering is the point. **The insecure option exists, but only the user can choose it**; the automatic fallback when no keyring exists is *in-memory and non-persistent*, i.e. it fails safe by forgetting rather than by writing a secret to disk. And when it does forget, the save path surfaces "Connections are not persisted in this session" as a warning rather than failing silently ([`save_connection.py` L57-58](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/save_connection.py#L57-L58)).

Keys are namespaced by service name and type — service `"sqlit"` (L24), account `f"{connection_name}:{key_type}"` where `key_type` is `db` or `ssh` (L252-262). Backend platforms are documented in the class docstring as macOS Keychain, Windows Credential Locker, Linux Secret Service (L229-241), matching the README's FAQ.

Two details that only show up in production and are cheap to copy:

- **The keyring is probed before it is trusted.** `_is_keyring_usable()` rejects `keyring.backends.fail`, rejects any backend with `priority <= 0`, and then does a read-only `keyring.get_password(KEYRING_SERVICE_NAME, f"probe:{secrets.token_hex(8)}")` against a random non-existent key, retrying three times with a 0.1s sleep "to handle transient D-Bus/keyring daemon issues" (L89-123). A random probe key is important: it can never collide with a real credential and it never writes.
- **Reads retry too.** `_get_with_retry` gives two retries at 0.2s, with the comment "A short retry helps with transient keyring/DBus/Keychain hiccups" (L264-291). A headless Linux CI box with a flaky Secret Service is a real environment.

There is also an escape hatch that is more important than the keyring for our use case: `password_command`. A connection can carry a shell command whose stdout is the password, run at connect time ([`connection_flow.py` L52-60](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/connection_flow.py#L52-L60)), documented in the README as `--password-command "op read 'op://Work/prod-db/password'"`. This lets a team commit a connection definition with **no secret in it at all**. lazysnap needs exactly this for `lazysnap.yml`, which is meant to be committed.

Provenance: keyring support was not in the original launch. On the Show HN thread a commenter asked whether credentials could avoid "leav[ing] your credentials saved in a text file or your bash history", and the author replied: "In the next release I am going to use Keyring to store credentials on the operating system's credential store." ([HN thread](https://news.ycombinator.com/item?id=46276002)) The lesson is not that keyring is hard; it is that **credential storage is the first thing a professional audience asks about**, and shipping without an answer costs you the exact users you want.

### 2.4 Keybinding discoverability

sqlit's mechanism has three layers, and the interesting one is the middle.

**Layer 1 — a footer that is a view over a binding list.** `ContextFooter.set_bindings(left, right)` takes lists of `KeyBinding(key, label, action, disabled)` and renders them; a disabled binding renders struck-through and dimmed rather than disappearing ([`widgets_footer.py` L20-80](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/ui/widgets_footer.py#L20-L80)). Showing an unavailable action as unavailable teaches the key exists; hiding it teaches nothing.

**Layer 2 — the footer contents are computed from the current selection's state, not from the current screen.** This is the part most tools miss:

```python
    shortcuts = provider_shortcuts
    if not shortcuts:
        action_label = "Connect" if is_connectable else "Select"
        shortcuts = [(action_label, "enter")]
        if show_save:
            shortcuts.append(("Save", "s"))
        if current_tab == TAB_CONNECTIONS:
            shortcuts.append(("New", "n"))

    if current_tab in (TAB_DOCKER, TAB_CLOUD):
        shortcuts.append(("Refresh", "f"))
```

([`connection_picker/shortcuts.py` L98-110](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/shortcuts.py#L98-L110), with `show_save` set only when `not is_container_saved(connections, container)` at L88-93)

Same key, different verb, driven by what is under the cursor: `<enter>` reads "Connect" on a running container and "Select" on something you cannot connect to; `s` for Save appears only when saving would actually do something. The footer is a live answer to "what can I do right now", not a static cheat sheet.

**Layer 3 — the help screen is generated from the live keymap.** `generate_help_sections()` resolves every key through the active keymap with a literal fallback:

```python
        def k(action: str, fallback: str) -> str:
            key = keymap.action(action)
            return format_key(key) if key else fallback
```

([`shell/state/machine.py` L168-207](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/state/machine.py#L168-L207)) — with the docstring stating why: "Keys are resolved from the active keymap so custom keybindings show up here too." A `STATE_TO_HELP_SECTION` map (L53) lets the help open scrolled to the section matching the state the user was in when they pressed `?`.

This was not there at launch either. The top-voted piece of UX feedback on the Show HN thread was precisely this drift:

> "It seems you put some menu items behind what I'll call '[space] mode'... This is not reflected properly in the help text shown when you press ? and that was a source of confusion for me. Especially since I managed to activate the fullscreen mode for one pane AND turn it off, but then couldn't figure out how I did it; and also, I did not find the space-Q option to Quit at first." ([HN, user `no_news_is`](https://news.ycombinator.com/item?id=46276002))

The author's reply: "Great UX feedback, that's going to be sorted out in the next release."

**And the drift is still there — it just moved to the README.** The in-app help is generated and correct; the README's keybinding table is hand-written and has rotted. Comparing [README's Keybindings table](https://github.com/Maxteabag/sqlit/blob/v1.6.3/README.md) against [`config/keymap.template.json`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/config/keymap.template.json):

| README says | Actual binding in `keymap.template.json` |
|---|---|
| `h` — Query history | `show_history` is `backspace`; in `query_normal`, `h` is `cursor_left` |
| `n` — New query (clear all) | `new_query` is `N`; `n` is unbound in `query_normal` |
| `d` — Clear query | `d` is `delete_leader_key` (the vim delete operator) |
| `v` / `y` / `Y` / `a` — View cell / Copy cell / Copy row / Copy all | In `results`, `v` is `view_cell` and `y` is `results_yank_leader_key` — a *prefix* opening the `ry` menu (`c` cell, `y` row, `a` all, `e` export). `Y` and `a` are unbound in `results` |
| `Ctrl+Q` — Quit | Works, but is not in the keymap at all; a code comment says quit "lives only as Textual's built-in App.BINDINGS (ctrl+q) and the `:q` command", deliberately un-rebindable "to avoid the footgun of a user accidentally locking themselves out of the exit key" ([`core/keymap.py` L377-380](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/core/keymap.py#L377-L380)) |

So four of the eleven rows on the front-page keybinding table of a 4,800-star project are wrong, nine months after the exact same class of bug was the top launch complaint. **Generating the in-app help was necessary and not sufficient.** lazygit solved the whole problem: its keybinding reference opens with "_This file is auto-generated. To update, make the changes in the pkg/i18n directory and then run `go generate ./...` from the project root._" ([Keybindings_en.md](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md)), organised by context and panel rather than by key. That is the standard to hit: **every rendering of a binding, in-app or in-docs, comes from one table, and CI fails on drift.**

The Ctrl+Q comment deserves its own note, because it is a genuinely good instinct that generalises: **the escape hatch must not be user-configurable.** For lazysnap the equivalent is cancel — `q`/`Ctrl+C` must always abort a run and always roll back cleanly, and no keymap file may take that away.

### 2.5 Autocomplete

sqlit's SQL completion engine is ~2,275 lines across [`sqlit/domains/query/completion/`](https://github.com/Maxteabag/sqlit/tree/v1.6.3/sqlit/domains/query/completion), with statement-specific handlers (`insert.py`, `update.py`, `delete.py`, `create_table.py`, `alter_table.py`, `drop.py`, `truncate.py`, `create_index.py`, `create_view.py`) dispatched from `get_context(sql, cursor_pos)` in [`completion.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/completion/completion.py), using `sqlparse` plus alias maps, CTE-name extraction and identifier-namespace resolution.

Most of that is irrelevant to lazysnap, which has no SQL editor. Four things are not:

1. **It triggers itself.** README: "Autocomplete triggers automatically in INSERT mode. Use `Tab` to accept." No key to remember, one key to accept.
2. **The schema it completes against is loaded lazily, in threads, with a spinner.** [`autocomplete_schema.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete_schema.py) runs a dozen distinct `run_worker(..., thread=True)` jobs — `get-databases`, `load-tables-*`, `load-views-*`, `load-procedures-*`, `load-columns-*` — batching processing at `SCHEMA_PROCESS_BATCH_SIZE = 200` and tracking `_schema_completed_jobs / _schema_total_jobs` for progress. Columns for a table are fetched only when that table is referenced.
3. **Everything is cached in one place and shared.** `_db_object_cache` carries a comment saying it is "Shared cache for raw DB objects - used by both tree and autocomplete" — the explorer and the completer introspect once between them.
4. **The completion result is dialect-formatted at the last moment**, via the provider's `format_autocomplete_identifier` ([`autocomplete_suggestions.py` L20-25](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete_suggestions.py#L20-L25)), so quoting rules live with the provider rather than in the completer.

For lazysnap the transferable pattern is: **introspect once, in the background, into one cache that every consumer reads** — the classifier, the planner, the root-table completer, and the TUI all need the same schema, and introspecting it twice on a 400-table database is a visible stall.

### 2.6 How it presents errors

Three distinct mechanisms, and the ordering between them is the design.

**(a) Enumerated states with per-state remedies.** Covered in §2.1: `DockerStatus` is four values because there are four fixes.

**(b) A handler registry that converts an error into an offered action.** This is the good idea:

```python
_DEFAULT_HANDLERS: tuple[ConnectionErrorHandler, ...] = (
    AzureFirewallHandler(),
    MissingDriverHandler(),
)


def handle_connection_error(app: ConnectionErrorApp, error: Exception, config: ConnectionConfig) -> bool:
    for handler in _DEFAULT_HANDLERS:
        if handler.can_handle(error):
            handler.handle(app, error, config)
            return True
    return False
```

([`ui/connection_error_handlers.py` L114-125](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/connection_error_handlers.py#L114-L125))

Each handler is `can_handle(error) -> bool` plus `handle(app, error, config)`, and `handle` *does the fix*, it does not describe it. `MissingDriverHandler` writes a pending-connection cache so the connection can be resumed after the restart that installing a driver requires, then pushes the install screen (L37-53). `AzureFirewallHandler` parses the offending IP out of the error text, looks up the Azure server, offers to add the firewall rule, and **retries the connection itself** if the rule was added (L57-111).

The pattern generalises perfectly: *recognised error → screen that performs the remedy → retry the original action.* Unrecognised errors fall through to generic display, and `handle_connection_error` returns `False` so the caller knows to do that. Adding a new remedy is adding one entry to a tuple.

**(c) Errors with the exact command for *this* machine.** `MissingDriverError` carries `driver_name`, `extra_name`, `package_name`, `module_name`, `import_error` ([`providers/exceptions.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/exceptions.py)), and [`install_strategy.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/install_strategy.py) probes how sqlit itself is running — `pipx`, `uv-tool`, `uvx`, `uv`, `conda`, `pip`, `unknown` (L102-130) — before deciding what to print or run. It distinguishes `uv tool install` (persistent) from `uvx` (ephemeral) because "the two require different injection commands, so they must not be conflated" (L106-109), checks PEP 668 externally-managed environments, checks whether install paths are writable, and even maps PyPI names to Arch package names (`psycopg2-binary` → `python-psycopg2`, L75-91).

That is a lot of machinery to print one line. It exists because of one Show HN comment:

> "Tried it out on a local test postgres db. First error: 'Connection failed, no module named psycopg2'" ([HN, user `waterTanuki`](https://news.ycombinator.com/item?id=46276002))

and the author's response to the surrounding thread: "the pipx comment made me decide to put much more thought into how sqlit helps with package installation on runtime, and I'm going to suggest pipx by default and it's also going to give the correct commands for every popular package manager."

The README now leads the driver table with "Most of the time you can just run `sqlit` and connect. If a Python driver is missing, `sqlit` will show (and often run) the right install command for your environment."

**The generalisable rule: an error message that names a fix the user cannot copy-paste is only half an error message.** lazysnap, as a single static Go binary, sidesteps the entire driver problem — which is a real argument for the language choice — but inherits the principle for every other failure: name the fix, in the form the user's machine will accept.

**(d) What sqlit does *not* have.** There is no `friendly_error()` / `humanize()` layer: `grep` for such a translator across the codebase finds nothing. Unrecognised driver errors reach the user raw. That is a defensible choice for a query tool where the user is a SQL speaker reading a SQL error. It is **not** defensible for lazysnap, where the failure will often be `permission denied for table foo` during a 20-minute extract and the user needs to know which of six things to do about it.

---

## 3. What made sqlit easy to adopt

### 3.1 Repository structure

The layout is domain-partitioned, not layer-partitioned: `sqlit/domains/{connections,query,shell,process_worker}/`, each split into `app/` (use cases), `domain/` (rules), `discovery/`, `providers/`, `ui/`, `cli/`, `store/`, with cross-cutting code in `sqlit/shared/` and `sqlit/core/`. A single commit, [`Domain partitioning (#57)`](https://github.com/Maxteabag/sqlit/commit/2d5aa54), moved to this shape early.

The consequence that matters for contribution: **adding a database is adding a directory.** Each provider is `{adapter,provider,schema}.py` under `providers/<name>/`, registering a `ProviderSpec` via `register_provider(SPEC)`. Docker detection, the URL-scheme table, the default port, whether SSH is supported, and the CLI's provider-specific flags all come from that one spec — `_get_schema_value_flags()` in [`cli.py` L26-37](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/cli.py#L26-L37) literally derives the CLI surface by iterating provider schemas. That is how a solo project got to 30-odd databases and 38 contributors in nine months: the marginal cost of a database is bounded and obvious.

There is a real cost, visible in the tree: the connections domain has grown `discovery/cloud/{aws,azure,gcp}` with its own registry and caches, plus SSH tunnels, plus mock providers, plus a dependency-install wizard. The "connect" step is now larger than the query engine. lazysnap should expect the same gravity around discovery and budget for it, and should also notice that sqlit paid for breadth with a hard runtime dependency on `docker` and `keyring` for every user regardless of need ([`pyproject.toml` L28-35](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L28-L35)).

### 3.2 README

What it does right, in order of how much it matters:

- **`pipx install sqlit-tui` is above the fold**, centred, before the badges finish. There is nothing to read before there is something to run.
- **The feature list is a demo reel.** Four animated GIFs in the first screenful — providers, history, filter, and `demo-docker-picker.gif` for Docker discovery — plus 24 theme screenshots under `docs/screenshots/all/`. Docker discovery is documented in one sentence and one GIF: "Automatically finds running database containers. Press 'Enter' to connect, sqlit figures out the details for you."
- **A try-it-with-no-database path**: `sqlit --mock=sqlite-demo`, documented immediately after `sqlit` itself. The mock machinery is real code (`MockConfig` in [`shared/app/runtime.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/app/runtime.py) can inject fake containers, fake missing drivers, fake query delays) and doubles as the test harness. Evaluation without prerequisites is the cheapest adoption win available.
- **A Motivation section that names the enemy** (SSMS, Electron, VS Code extensions) and tells a story. It reads as a person's opinion, not a product page.
- **The FAQ answers the two questions a professional actually has**: where do my passwords go, and why should I use this instead of Harlequin/lazysql.
- **Five install channels**: pipx, uv, pip, AUR, Nix flake.

What it does badly, and we should not: the hand-maintained keybinding table has drifted (§2.4), and the driver-matrix table is 25 rows of the front page — sqlit's own CONTRIBUTING vision says "should not render one unnecessary pixel", which the README does not honour.

### 3.3 Release process

[`.github/workflows/release.yml`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/.github/workflows/release.yml) is triggered by `push: tags: ['v*']` (plus a `workflow_dispatch` with an `aur_only` escape hatch), and runs four jobs:

- `release` — creates the GitHub release with `generate_release_notes: true`. Nobody hand-writes changelogs.
- `build` — `python -m build`, uploads `dist/` as an artifact.
- `publish` — downloads that artifact and publishes with `pypa/gh-action-pypi-publish`, in `environment: pypi` with `permissions: id-token: write`. **Trusted publishing via OIDC; there is no long-lived PyPI token in the repo's secrets.**
- `aur` — computes the sha256 **from the built tarball rather than waiting for PyPI** (with an explicit comment saying so), rewrites `pkgver`/`pkgrel`/`sha256sums` in `aur/PKGBUILD` with `sed`, and pushes to the AUR.

So a release is: tag, push. Four releases shipped in the three weeks before `v1.6.3` — `v1.6.0` on 2026-08-06, then `v1.6.1`, `v1.6.2`, `v1.6.3` on 23, 26 and 27 August (62 tags total against 559 commits, per `git tag` and `git rev-list --count HEAD` on the clone) — a cadence that is only possible when releasing is one command.

[`.github/workflows/ci.yml`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/.github/workflows/ci.yml) is worth copying for one job in particular. Alongside the unit-test matrix (3.10/3.12 under `uv`) and a `nix build .#sqlit` job, there is a `build` job across Python 3.10-3.13 whose entire purpose is:

```yaml
      - name: Check package builds
        run: python -m build

      - name: Verify CLI entry point
        run: |
          python -c "from sqlit.cli import main; print('CLI import OK')"
```

**A CI job that does nothing but prove the thing installs and starts.** For lazysnap that is: goreleaser builds every target, and the resulting binary answers `lazysnap --version`. Cheap, and it catches the class of failure that costs you a user on their first minute.

### 3.4 Contributor docs

[`CONTRIBUTING.md`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/CONTRIBUTING.md) is unusual and mostly excellent.

The mechanical half is complete and honest about cost. There is a tiered test story so a drive-by contributor is never blocked on infrastructure: `pytest tests/cli/ -v` for CLI end-to-end; `pytest tests/ -v -k sqlite` explicitly labelled "No Docker Required"; then the full suite behind `docker compose -f infra/docker/docker-compose.test.yml up -d` with an honest "Wait for the databases to be ready (about 30-45 seconds)"; an `--profile enterprise` for the heavyweight containers (Db2, Trino, Presto, Oracle 11g); and tests that create containers hidden behind an opt-in flag, `pytest tests/integration/docker_detect/ -v --run-docker-container` (the flag is registered in [`tests/conftest.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/tests/conftest.py#L47)). Every fixture's environment variables are tabulated with defaults, per database. Combined with `.pre-commit-config.yaml`, `pyrightconfig.json` and a Ruff config in `pyproject.toml`, a contributor can get to green locally without asking anyone.

The other half is the interesting one: **CONTRIBUTING.md carries the product vision, and uses it as a filter on pull requests.** It defines "CEQR" (Connecting, Exploring, Querying, Results) and "EAFF" (Easy, Aesthetically pleasing, Fun, Fast), then states the rejection rule outright: "If an idea or feature does *not* achieve any of the 'CBQV' elements adhering to all of the 'EAFF' requirements. It does not belong to sqlit." (The `CBQV`/`CEQR` typo is in the original.)

Several of its rules are directly usable by lazysnap:

- On daily use: "Every feature in sqlit should have a target audience in which they will use it every time they use sqlit. If nobody is going to a feature every day. It does not belong to sqlit."
- On magic: "sqlit should never do anything under the hood that the user might have interest in understanding... User should never ask 'wait, how did it know?' 'why is this here?' 'why did it work then, but not now?'" — and the distinction that makes it operable: "Universal state problems deem for magical fixes. Conditional state problems, explicit user awareness."
- On settings: "There should be no settings or preferences with important exception of interface (aesthetics, keyboard bindings)... Do not include a feature that a user finds annoying. Settings to disable a feature is a symptom of this."
- On keybindings: "all necessary keybindings to do 'CEQR' well, must be visible at all times", with everything else "hidden behind help `<?>` or command menu `<space>`", and a stated decision hierarchy: "1. Intuitive to learn 2. Harmony 3. Traditions (vim, specifically)".

The "magic" rule is the one lazysnap most needs, and it is stricter for us than for sqlit. Auto-detecting *which database to read* and *which database to write* is exactly a conditional-state problem, so by sqlit's own rule it demands explicit user awareness — printed decisions, every time, with the flag that changes them. That is the principle behind §5.2.

One more adoption note visible in the repo: PR #279's body ends "_Implementation and review were AI-assisted; the final diff and test evidence were manually reviewed._" A norm of disclosing AI assistance *and* attaching test evidence to the PR — "regression test failed before the change and passed afterward" — is worth adopting verbatim.

### 3.5 The adoption failure worth learning from

The single worst first-run experience sqlit shipped was not a bug in its code. From the Show HN thread:

> "I was surprised to find that I could not run it with uvx: `% uvx sqlit` … `SyntaxError: Missing parentheses in call to 'print'`" — followed by "That's not the same package. You should try sqlit-tui", "for uvx you need to do `uvx --from sqlit-tui sqlit`", and finally: "Ah, thanks. This worked great. **I was fooled by the package name.**" ([HN, users `lgas`, `hiichbindermax`, `mrbump`](https://news.ycombinator.com/item?id=46276002))

The same trap was filed two months later as [issue #133, "Can't install with psycopg2-binary"](https://github.com/Maxteabag/sqlit/issues/133), where the reporter's `uvx --with psycopg2-binary sqlit-tui` failed with "An executable named `sqlit-tui` is not provided by package `sqlit-tui`", and their fallback `uvx ... sqlit` installed an unrelated, long-abandoned PyPI package called `sqlit` that is not even Python 3 source.

Two independent defects, both now fixed, both instructive:

1. **The distribution name and the command name differed**, so `uvx <name>` — the most natural thing a uv user types — resolved to a squatted package. `pyproject.toml` now declares *both* `sqlit` and `sqlit-tui` as console scripts ([L172-174](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L172-L174)), so `uvx sqlit-tui` works.
2. **Someone else owned the obvious name.** Nothing in the code could have fixed that.

For lazysnap: the binary name, the repo name, the install command and the package/tap/formula name must be one word, and that word must be checked for prior claims **before the name is committed to** — on Homebrew core and popular taps, on the AUR, on crates.io/npm/PyPI regardless of our language, and as a Go module path. See NAME.md.

---

## 4. What lazysnap should not copy

- **Substring image matching as a sufficient identity test.** `"postgres" in image_name` is fine for populating a picker the user chooses from. lazysnap uses the answer to decide what to *read from* and what to *write to*. Match on the image, then **verify by connecting and asking the server** (`SELECT version()`), and treat the container as a candidate, never as an answer.
- **A hard runtime dependency on Docker for every user.** sqlit imports `docker` unconditionally ([`pyproject.toml` L33](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L33)). In Go this is a vendored SDK rather than an install-time burden, but the behavioural rule stands: **no Docker daemon must never be an error path**, only a shorter discovery ladder.
- **Prompting for a password inside the flow as the primary mechanism.** sqlit's `needs_db_password` → prompt sequence ([`domain/passwords.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/domain/passwords.py), [`app/connection_flow.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/connection_flow.py)) is right for a long-lived interactive session. lazysnap's headline command must work unattended in CI, so a prompt is a fallback, never a step.
- **Thirty databases.** sqlit's provider catalog is its strength and its sprawl. CONCEPT.md's non-goals already say Postgres first; the provider-spec *shape* is worth copying now so that breadth is cheap later, but not the breadth.
- **A hand-maintained keybinding table in the README.** Demonstrated broken above. Generate it.
- **Raw driver errors reaching the user unmediated** (§2.6d).
- **A settings file with feature toggles.** sqlit's own vision doc forbids it and sqlit mostly holds the line — the one exception, `allow_plaintext_credentials`, is a security downgrade the user must choose explicitly, which is the right kind of exception. lazysnap's equivalent boundary is absolute: no flag disables masking wholesale.

---

## 5. lazysnap's equivalent, step by step

Everything below is a **proposal**, not a validated design. Nobody has run it. §7 lists what needs testing first.

### 5.0 The rules this design has to satisfy

From CONCEPT.md and CLAUDE.md:

1. First run asks **at most one question**, then works.
2. Config is **emitted after** a run, never demanded before it.
3. Anything that might be personal data is masked unless the user opts out per column, with a stated reason.
4. The tool **never holds write access to the source**.
5. The same command a human types works headless in CI.
6. Every TUI action is reachable by a CLI flag first.
7. Documentation is never the fix for a confusing first run — change the default or the question.

Rules 1 and 4 are in tension, and resolving that tension is the substance of this section. Rule 4 means lazysnap must decide which of two databases gets written to. Getting that wrong is the worst possible failure — it writes masked fake rows into production. Rule 1 says we cannot buy safety by asking.

### 5.1 The discovery ladder

`lazysnap` with no arguments runs the ladder below **in order, concurrently where possible, with a hard 2-second budget for the whole discovery phase**, and prints what it found. Each rung yields zero or more *candidates*, each carrying `(dsn, provenance, confidence)`.

| # | Probe | Yields | Notes |
|---|---|---|---|
| 0 | `./lazysnap.yml` | The whole previous run | If present, this is not a first run; see Scenario D in §5.5 |
| 1 | `$DATABASE_URL`, then `.env`, `.env.local`, `.env.development` in the working directory | One DSN each | Parse per the Postgres URI grammar `postgresql://[userspec@][hostspec][/dbname][?paramspec]`, accepting both `postgresql://` and `postgres://` schemes ([libpq docs](https://www.postgresql.org/docs/current/libpq-connect.html)). Also honour `POSTGRES_URL`, `PG_URL`, `DB_URL` as aliases |
| 2 | libpq environment: `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`/`PGPASSFILE`, `PGSERVICE` | One DSN | These are the documented libpq connection defaults ([libpq-envars](https://www.postgresql.org/docs/current/libpq-envars.html)). A developer who has `psql` working already has this set; using it costs nothing and looks like telepathy. Prefer `PGPASSFILE`/`~/.pgpass` over `PGPASSWORD`, which the same page warns against because "some operating systems allow non-root users to see process environment variables via `ps`" |
| 3 | Running Docker containers whose image matches a Postgres pattern | One candidate per container | sqlit's mechanism (§2.1), plus the compose labels below |
| 4 | Exited Postgres containers | Candidates marked `stopped` | Shown, not hidden; offered as "start it?" |
| 5 | `docker-compose.yml` / `compose.yaml` in the working directory or a parent | Service names only | See below — this is a **naming** source, not a connection source |

**Docker contexts must be resolved at rung 3.** Because Go's `client.FromEnv` does not read them ([pkg.go.dev](https://pkg.go.dev/github.com/docker/docker/client)) but Docker's CLI does ([contexts docs](https://docs.docker.com/engine/manage-resources/contexts/)), lazysnap resolves the daemon endpoint in this order, matching the CLI's documented precedence: `--host` flag → `DOCKER_HOST` → `DOCKER_CONTEXT` → the current context from `~/.docker/config.json` and `~/.docker/contexts/*/meta.json` → the default unix socket. Without this, every OrbStack and Colima user sees "Docker not running" on their first run. **This is the highest-risk single defect in the first-run path and it belongs in the first integration test.**

**Why compose is rung 5 and not rung 1.** A `docker-compose.yml` describes intent, not reality. Ports can be `${DB_PORT:-5432}:5432`, and Compose resolves those from a `.env` in the project directory with a documented precedence order ([Compose env-var precedence](https://docs.docker.com/compose/how-tos/environment-variables/envvars-precedence/)); services can be behind profiles and not running; the file in the directory may not be the file the running containers were started from. Reading the YAML to *guess* a port is how you produce a connection string that does not connect — the §2.1 failure mode.

What the running containers *do* carry is authoritative provenance, because Compose stamps it on them. From Compose's own source: `ProjectLabel = "com.docker.compose.project"`, `ServiceLabel = "com.docker.compose.service"`, `WorkingDirLabel = "com.docker.compose.project.working_dir"`, `ConfigFilesLabel = "com.docker.compose.project.config_files"`, `OneoffLabel = "com.docker.compose.oneoff"` ([`pkg/api/labels.go`](https://github.com/docker/compose/blob/main/pkg/api/labels.go)). So lazysnap:

- filters running containers to those whose `com.docker.compose.project.working_dir` **is the current directory or an ancestor of it** — this is what makes `lazysnap` in a project folder find *that project's* database and not the four other Postgres containers on the machine;
- uses `com.docker.compose.service` as the display name (`db`, `postgres`, `shop-db`), which is what the developer calls it;
- skips containers labelled `com.docker.compose.oneoff` (they are `compose run` throwaways);
- falls back to `container.name` when there are no Compose labels.

And it takes the port from the live port bindings, with sqlit's three fallbacks and `_resolve_published_host`'s address-family fix (§2.1). `docker compose ps --format json` exposes the same data in a `Publishers` field with `PublishedPort` ([compose ps docs](https://docs.docker.com/reference/cli/docker/compose/ps/)) and is a reasonable cross-check, but shelling out to the CLI is a worse dependency than the SDK; use it only in a diagnostic subcommand.

**Every candidate is then verified by connecting**, in parallel, with a 1-second dial timeout, and asking three questions: `SELECT version()`, the count of user tables in non-system schemas, and whether the current role can create a schema. A candidate that fails to connect is kept in the printed report with its error, and never silently dropped — the user needs to see that lazysnap found their database and could not log in to it.

### 5.2 Choosing source and target without asking

Verification gives, per candidate: reachable, table count, approximate row count, writability, and provenance. From those:

**A candidate is eligible to be the target if and only if** it is reachable, the current role can create objects in it, and *either* it has zero user tables, *or* it contains the `lazysnap_meta` marker table written by a previous lazysnap run. Nothing else is ever written to, at any confidence, under any flag short of an explicit `--target`.

**The source is the candidate with the most tables that is not the chosen target.** Source connections are opened with `default_transaction_read_only = on` set on the session and every statement issued inside a read-only transaction, so rule 4 is enforced by the server rather than by our discipline.

**Both decisions are printed, always, with the flag that changes them**, honouring sqlit's own "conditional state problems, explicit user awareness" rule:

```
  source  shop-db        (compose service "db", 41 tables)        --source
  target  shop-db-test   (compose service "db-test", empty)       --target
```

Ambiguity is resolved by these tie-breaks, in order, and the reasoning is available under `?`:

1. exactly one eligible target → take it;
2. more than one → prefer the one whose Compose service or database name matches `/(test|local|dev|snapshot|scratch)$/`;
3. still tied → prefer the one lazysnap itself created (marker table present);
4. still tied → this is undetermined, and the ladder rule in §6 applies.

Note the asymmetry that makes this safe: **the target test is a hard gate, the source test is a heuristic.** A wrong source produces a useless snapshot and wastes a minute. A wrong target destroys data. So the source may be guessed; the target may only be *qualified*.

### 5.3 The one-question rule, stated exactly

> **lazysnap asks at most one blocking question per run. It is the first item in the decision ladder that lazysnap could not determine with confidence. Every item below the asked one takes its computed default, and every default is printed as a decision alongside the flag that changes it. If an item cannot be determined and has no safe default, lazysnap does not ask — it stops, prints each probe it ran and what that probe found, and gives the exact command to run instead.**

The decision ladder is: **source → target → root table → row count → masking**.

- *Source* has no safe default. It is never asked; failure to find one stops the run (§5.5).
- *Target* has a safe default whenever a local Postgres exists. When the only Postgres reachable is the source itself and Docker is available, the target becomes the question (Q1).
- *Root table* has a computed default. On the happy path this is the one question (Q2), exactly as CONCEPT.md's transcript shows.
- *Row count* defaults to 500 and is never asked.
- *Masking* is never asked; it happens, and it is explained.

In a non-interactive session (no TTY, or `--yes`), no question is ever asked: every default is taken, and any question whose default is "no" becomes a hard failure naming the flag that would have allowed it. That is how rule 5 (`same command works in CI`) and rule 1 (`one question`) coexist.

### 5.4 Root-table default, and why it can be defaulted at all

The one question is answerable by pressing Enter only if the default is usually right. The default is computed from the introspected schema, which lazysnap already has:

For each table: `score = (inbound FK count) − (outbound FK count)`, then discard anything that looks like a lookup table (fewer than ~1,000 rows and no inbound FKs from more than two distinct tables, or a name matching `/_(types?|statuses|kinds|codes)$/` or a known set like `countries`, `currencies`, `migrations`, `schema_migrations`), then break ties by preferring a name in `{customers, users, accounts, organizations, organisations, tenants, companies, clients}`, then by row count descending.

The top-scoring table is the default; `?` at the prompt shows the ranked top five with the score components, so the user can see *why* `customers` was proposed. This satisfies rule 7 — the answer to "why did it pick that?" is one keypress, not a doc page.

`--root <table>` skips the question entirely, which is what CI uses and what `lazysnap.yml` records.

### 5.5 The three scenarios

#### Scenario A — a folder with a `docker-compose.yml`

The happy path, and the one CONCEPT.md draws.

```
$ lazysnap
  docker · compose project "shop" (working_dir matches .)
  found 2 postgres containers: shop-db (5432, 41 tables), shop-db-test (5433, empty)
  source  shop-db        41 tables · read-only session          --source
  target  shop-db-test   empty · writable                       --target
  17 of 214 columns look like personal data   (? to see why)
  root table? [customers]
  planning… 23 tables in slice, ~18k rows
  extract ▓▓▓▓▓▓▓▓▓▓ 18,204 rows   mask 17 cols   load ▓▓▓▓▓▓▓▓▓▓
  ✓ 500 customers and everything they touch, in 38s
  ✓ foreign keys verified · wrote lazysnap.yml (commit it for CI)
```

Sequence: resolve the Docker endpoint (contexts included) → list running containers → filter by `com.docker.compose.project.working_dir` → match Postgres images → read `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` from each container's env → resolve host and port from live bindings with the address-family fix → connect and verify both → apply the eligibility gate → print both decisions → introspect the source → classify columns → **ask the one question** → plan, extract, mask, load, verify, write `lazysnap.yml`.

Note the credential step needs no prompt for the same reason sqlit's does not (§2.1, step 3): the containers already carry the passwords.

Variants:
- **Only one Postgres container, and it has tables.** It is the source; there is no eligible target; Q1 fires (§6). Everything below Q1, including the root table, takes its default and is printed.
- **The second container exists but is exited.** It is shown, and offered: "target shop-db-test is stopped — start it? [Y/n]". This is Q1 in a friendlier form; the root table still takes its default. Copied directly from sqlit's decision to surface exited containers.
- **Compose file present, nothing running.** lazysnap does **not** parse the YAML to build a DSN. It names the services it saw and stops: `compose.yaml defines services db, db-test — none are running. try: docker compose up -d db`.

#### Scenario B — a folder with `DATABASE_URL` in `.env`

```
$ lazysnap
  env · DATABASE_URL → postgres://app@db.prod.internal:5432/shop
  source  db.prod.internal/shop   412 tables · read-only session   --source
  no local postgres found to load into
  start one? postgres:16 as lazysnap-target-shop on port 55432 [Y/n]
```

The DSN in `.env` is almost always the *application's* database — which is the source, and is frequently remote. There is then no eligible target, so the target rung is the first undetermined item and Q1 is the one question. The root table takes its default and is printed:

```
  root table  customers  (23 inbound FKs, 0 outbound)             --root
```

Details that matter:

- **The password may be absent from the URL.** Resolve in order: the URL's own userinfo → `PGPASSWORD` → `PGPASSFILE`/`~/.pgpass` → the OS keyring under service `lazysnap` → prompt. The prompt is a secret prompt, not a configuration choice; it does not count against the one-question budget, it never appears when any earlier source succeeds, and it never appears in CI (a missing credential there is a hard failure naming `PGPASSWORD`/`--password-command`).
- **Never persist a discovered secret without being asked.** After a prompted password, print a hint — `tip: lazysnap creds save stores this in your OS keyring` — rather than a question. This is sqlit's keyring design (§2.3) with the interaction budget removed.
- **`lazysnap.yml` records the reference, never the value**: `source: {from: env, var: DATABASE_URL}`. sqlit's `password_command` (§2.3) is the precedent for a committed connection definition that contains no secret; lazysnap should support `password_command` for the same reason.
- **A remote source deserves a louder read-only confirmation.** The header prints `read-only session` for every source, but for a source that is not on `localhost`/`127.0.0.1`/`::1`/a local container, it also prints the role and asserts `default_transaction_read_only` was accepted by the server. If the server refuses to set it, lazysnap stops. Rule 4 is not advisory.

#### Scenario C — a folder with nothing

No `lazysnap.yml`, no `.env`, no `DATABASE_URL`, no `PG*`, no Docker containers.

lazysnap **does not ask a question**, because there is no safe default to ask *about* — a prompt for a DSN is strictly worse than a command line, since the user cannot edit history, cannot tab-complete, and cannot re-run it. It prints the ladder it walked, marks each rung, and stops with exit code 3:

```
$ lazysnap
  no database found here.

  looked for                                              result
  ./lazysnap.yml                                          not found
  $DATABASE_URL, ./.env, ./.env.local                     not found
  $PGHOST / $PGSERVICE                                    not set
  docker (context "orbstack")                             running · 0 postgres containers
  ./compose.yaml, ./docker-compose.yml                    not found

  point it at a database:
      lazysnap postgres://user@localhost:5432/shop

  or start one and re-run:
      docker run -d --name shop-db -e POSTGRES_PASSWORD=… -p 5432:5432 postgres:16
```

Three properties of that output are deliberate. It shows **what was checked**, so the user can tell "wrong directory" from "wrong tool". It shows **the resolved Docker context by name**, which is how the Colima/OrbStack user discovers lazysnap is talking to a daemon they did not expect. And it ends in **a command, not a doc link** — rule 7.

Sub-cases:
- **Docker not available at all.** The docker row reads one of four things — `not installed`, `not running`, `not accessible (permission denied — add your user to the docker group, or set DOCKER_HOST)`, or `context "x" unreachable` — mirroring sqlit's four-state enum (§2.1) but always with the remedy attached, because unlike sqlit's picker there is nothing else on the screen.
- **A Postgres container is running but not from this project.** It *is* offered, marked with its provenance: `docker · postgres-16 (5432, 41 tables) — not from this compose project`. Being able to say "yes, that one" is worth more than directory purity; being told where it came from is what stops it being magic.

#### Scenario D — the second run

`./lazysnap.yml` exists. Zero questions. The file supplies source reference, target reference, root table, row count, per-column masking decisions and opt-outs. lazysnap re-verifies the target's eligibility gate (a file cannot authorise writing to a database that has since acquired data without the marker), prints the same decision header with `from lazysnap.yml` as the provenance, and runs. `--reconfigure` re-enters the first-run path. This is what CI runs, and it is the same binary and the same command.

### 5.6 Where state lives

- **`./lazysnap.yml`** — emitted by a successful run, meant to be committed. References, never secrets. Records the decisions, the plan, the classification results with reasons, and the opt-outs.
- **`~/.config/lazysnap/`**, overridable by `$LAZYSNAP_CONFIG_DIR`, respecting `$XDG_CONFIG_HOME` — machine-local, non-essential state only (last-used targets, a cached schema fingerprint). Directly modelled on sqlit's `_resolve_config_dir()` precedence: "`$SQLIT_CONFIG_DIR` if set (no migration); `$XDG_CONFIG_HOME/sqlit` (falling back to `~/.config/sqlit`)" ([`shared/core/store.py` L25-78](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/core/store.py#L25-L78)). Deleting this directory must never break a run.
- **OS keyring, service name `lazysnap`, account `<host>:<port>/<database>`** — only ever written by an explicit `lazysnap creds save`. Backend via [`zalando/go-keyring`](https://github.com/zalando/go-keyring) (MIT, 1,324 stars, last pushed 2026-07-24 — [GitHub API](https://api.github.com/repos/zalando/go-keyring), checked 2026-09-05) or equivalent; probe it before trusting it, and fall back to *not storing* rather than to a plaintext file, which is the safe half of sqlit's fallback chain (§2.3).
- **The target's `lazysnap_meta` table** — one row per run: run id, source fingerprint, root table, row count, masking secret id (not the secret), timestamp, tool version. This is what makes the target eligibility gate work across runs and across machines.

### 5.7 Error presentation

Adopt sqlit's handler-registry pattern (§2.6b) wholesale: an ordered list of handlers, each `CanHandle(error) bool` and `Handle(...)` that *performs* the remedy and retries, with unhandled errors falling through to a generic display. The v1 registry:

| Recognised condition | What lazysnap offers |
|---|---|
| Docker unreachable in any of the four states | The state's specific remedy line; re-probe on retry |
| Docker context resolves to an unreachable endpoint | Name the context and its endpoint; list the other contexts found |
| `password authentication failed for user "x"` | Show which of the five credential sources supplied the password; offer to prompt |
| `connection refused` on a container we detected | Note the published address family; if we chose `localhost`, retry once on the published address (the §2.1 / PR #279 failure) |
| `database "x" does not exist` | List the databases the role can see |
| `permission denied for table x` during extract | Name the table, the role, and the exact `GRANT SELECT` statement; offer to skip the table and record the skip in `lazysnap.yml` |
| Source refuses `default_transaction_read_only` | Stop. This is a safety rail, not a warning |
| Target non-empty without the marker table | Refuse; print the table count and the `--target` flag; in interactive mode this is Q3 |
| FK verification fails after load | Name the constraint, the child rows, and the parent that was capped; point at the fan-out cap that caused it |
| Extract interrupted | Roll the target back; say what was rolled back |

Two rules on top of the pattern. **Every error names the stage it happened in** — discover / classify / plan / extract / mask / load / verify — because a 20-minute run that fails needs to say where. And **no raw driver error reaches the user without a lazysnap sentence above it**, which is the gap in sqlit (§2.6d): the raw text stays, underneath, for the user who wants it.

### 5.8 Discoverability of keys and flags

- **A footer rebuilt from the same table that defines the bindings**, in the spirit of `ContextFooter` (§2.4 layer 1), and **computed from current state** in the spirit of `build_picker_shortcuts` (layer 2): `? why masked` only appears once classification has run; `s skip table` only appears while a table is being extracted.
- **`?` during a run** expands the classification inline — column, why it matched, which masker will be used — which is the promise CONCEPT.md already makes.
- **`--help` grouped by pipeline stage** (discover / classify / plan / extract / mask / load / verify), not alphabetically, so the flag list teaches the model.
- **`docs/KEYBINDINGS.md` and the flag reference are generated** from the binding and flag tables, with a CI check that fails on drift — lazygit's arrangement ("_This file is auto-generated… run `go generate ./...`_"), and the direct fix for the drift still visible in sqlit's README (§2.4).
- **Cancel is not rebindable.** sqlit's reasoning for `ctrl+q` applies with more force to a tool that writes to databases.

### 5.9 Adoption checklist lifted from §3

Not part of the first-run spec, but the other half of what this study is for:

- One binary name, checked for prior claims on Homebrew, AUR, and the major package registries before the name is fixed (the `sqlit` / `sqlit-tui` trap, §3.5).
- One-line install above the fold; a GIF per capability; a `--demo` path that needs no database, documented in the first screenful.
- Tag-triggered release with generated notes and OIDC/keyless publishing — no long-lived tokens in repo secrets.
- A CI job whose only purpose is to prove the artefact installs and starts (`lazysnap --version` on every built target).
- Tiered tests: unit with no dependencies, integration behind a documented `docker compose` fixture with an env-var table of defaults, and container-creating tests behind an opt-in pytest/`go test` flag.
- `CONTRIBUTING.md` carries the vision filter, not just the build commands. sqlit's CEQR/EAFF section is why 38 contributors did not turn it into DBeaver.
- PR template that asks for test evidence and discloses AI assistance, as sqlit's PRs do.

---

## 6. The complete question catalogue

Every question lazysnap may ask, with its default, when it fires, and what happens with no TTY. **Q2 is the only question on the happy path.** The ladder rule in §5.3 guarantees at most one of Q1/Q2/Q3 is asked in any run.

| # | Question | Default | Fires when | Non-interactive behaviour | Flag that avoids it |
|---|---|---|---|---|---|
| **Q1** | `no local postgres found to load into. start one? postgres:16 as lazysnap-target-<project> on port <free> [Y/n]` | **Yes** | Source determined; no eligible target; Docker available | Takes the default (starts the container). `--no-create-target` turns it into a hard failure | `--target <dsn>` |
| **Q1′** | `target <name> is stopped — start it? [Y/n]` | **Yes** | An eligible target exists but its container is exited | Takes the default | `--target <dsn>` |
| **Q2** | `root table? [customers]` | **The top-scoring table** per §5.4; `?` shows the ranked candidates and why | Source and target both determined (the happy path) | Takes the default | `--root <table>` |
| **Q3** | `target <name> has 41 tables and no lazysnap marker. wipe and load? [y/N]` | **No** | An explicit `--target` points at a non-empty, unmarked database | **Hard failure**, naming `--allow-nonempty-target` | `--allow-nonempty-target` |
| **Q4** | `password for <role>@<host>/<db>:` | none — a secret prompt, not a configuration choice | All five credential sources (URL, `PGPASSWORD`, `PGPASSFILE`/`~/.pgpass`, keyring, `password_command`) came up empty | **Hard failure**, naming `PGPASSWORD` and `--password-command` | `--password-command`, `lazysnap creds save` |
| **Q5** | `<n> rows already in <table> in the target. keep them? [y/N]` | **No** (truncate) | Re-running into a marked target whose contents are stale | Takes the default | `--append` |

Things that are deliberately **not** questions, with their fixed defaults:

| Decision | Default | Why it is not a question | Flag |
|---|---|---|---|
| Which candidate is the source | Most tables among non-target candidates | Printed as a decision; wrong answers are cheap and reversible | `--source` |
| Which candidate is the target | The one passing the eligibility gate, tie-broken by name then by marker | Printed; wrong answers are prevented by the gate, not by asking | `--target` |
| Row count | **500** | CONCEPT.md's transcript; a number nobody has an opinion about until they do | `-n`, `--rows` |
| Whether to mask a column classified as personal data | **Always mask** | Rule 3. No flag disables masking wholesale, ever | `--unmask <table.column>`, per column, recorded in `lazysnap.yml` |
| Which masker to use for a column | The classifier's choice for that category | Printed under `?` with the reason; changed in `lazysnap.yml` after the run | `lazysnap.yml` |
| Per-table fan-out cap | A computed cap that keeps the slice near the requested size | Printed in the plan; a question here would be unanswerable without seeing the plan first | `--cap <table>=<n>` |
| Whether to write `lazysnap.yml` | **Always**, on success | It is the record of the run, not a preference | `--no-config` |
| Whether the run is reproducible | Always records the masking secret *id*, never the secret | Secrets are never written to a committed file | `--secret-file` |

**Interaction budget audit.** Scenario A: Q2 only. Scenario B with a remote source: Q1 only (root table defaults, printed). Scenario B with a local Postgres also present: Q2 only. Scenario C: no questions, a stop with a command. Scenario D: no questions. In CI: no questions, ever, in every scenario.

---

## 7. Gaps and unverified points

- **Reddit is unreachable from this environment**, so community sentiment is sampled only from Hacker News and GitHub. The Show HN thread is nine months old; friction discovered since then may be recorded in places I could not read. A second, lower-traffic HN submission exists ([id=48271934](https://news.ycombinator.com/item?id=48271934), 2026-05-25, 20 points, 5 comments) and adds nothing.
- **No public write-up by sqlit's author** on the Docker-detection design exists that I could find. §2.1 is reconstructed from source and from the bodies of PRs [#279](https://github.com/Maxteabag/sqlit/pull/279) and [#280](https://github.com/Maxteabag/sqlit/pull/280).
- **docker-py's handling of `DOCKER_CONTEXT` is unverified.** The [documented set](https://docker-py.readthedocs.io/en/stable/client.html) for `from_env()` is `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`. I did not find a statement either way about contexts. For Go this is settled — [`client.FromEnv`](https://pkg.go.dev/github.com/docker/docker/client) reads four variables and contexts are not among them — which is why §5.1 specifies resolving them ourselves. **The precise file format of `~/.docker/contexts/*/meta.json` is not documented on the pages I read**; the implementation should read it from the Docker CLI's own source or use a maintained helper, and this is flagged as an implementation-phase task rather than a settled design.
- **§5 is a proposal that nobody has run.** The specific claims most in need of testing, in order of risk: (a) that the target eligibility gate is *not* so strict that scenario A's happy path fails on real projects whose test database already has migrated-but-empty tables — a migrated schema has tables and no rows, so "zero user tables" may be the wrong test and "zero rows in every user table" may be right; (b) that the root-table scoring proposes the table a developer expects on at least, say, eight of ten real schemas; (c) that Q1's "start a container" default reads as helpful rather than alarming; (d) that a per-run password prompt with no persistence is not annoying enough to drive people to write DSNs into `lazysnap.yml` by hand.
- **The discovery budget is unmeasured.** §5.1 asserts a 2-second cap covering a Docker list plus parallel verification connections. Verification requires a connection and three queries per candidate; against a slow remote source that may blow the budget on its own. What lazysnap should do when discovery times out mid-verification (proceed with partial results? show a spinner and keep going?) is unspecified.
- **`lazysnap_meta` is named here but not specified** — schema, contents, what happens when the target is shared with another tool, and what happens when the marker exists but was written by an incompatible version.
- **The root-table heuristic has no evidence behind it.** In-degree minus out-degree is plausible and untested. Schemas with a single tenant table above the "customer" table (multi-tenant SaaS) will likely score the tenant table highest, which may or may not be the right answer.
- **I did not review sqlit's TUI rendering, theming, results virtualisation, SSH tunnels, or cloud discovery** beyond noting that they exist. They are not on lazysnap's path to a first run.
