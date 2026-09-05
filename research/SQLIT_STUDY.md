# sqlit study: how a zero-config first run is actually built

Phase 1 research. Written 2026-09-04.

**Method.** Cloned [`Maxteabag/sqlit`](https://github.com/Maxteabag/sqlit) to `/private/tmp/claude-501/lazysnap-scratch/sqlit` and read it at commit `2a09c28` (two commits past the `v1.6.3` tag; every file quoted below is byte-identical between `v1.6.3` and that commit, so all permalinks below point at `v1.6.3` for stability). Also read [lazygit's README](https://github.com/jesseduffield/lazygit/blob/master/README.md) and its [auto-generated keybindings reference](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md), and the [Show HN thread](https://news.ycombinator.com/item?id=46276002) that launched sqlit, which is the best available record of what users hit on their own first run. Reddit is unreachable from this environment; HN, GitHub issues/PRs, and official docs were used instead.

---

## 1. sqlit as of September 2026

| Fact | Value | Source |
|---|---|---|
| Repository | `Maxteabag/sqlit`, MIT, Python | [GitHub](https://github.com/Maxteabag/sqlit) |
| Created | 2025-12-13 | GitHub API (`repos/Maxteabag/sqlit`, `created_at`) |
| Stars / forks / open issues | 4,801 / 162 / 15 | GitHub API, fetched 2026-09-04 |
| Commits / contributors / tags | 559 / 38 / 61 | `git rev-list --count HEAD`, `git shortlog -sn`, `git tag` on the clone |
| Latest release | `v1.6.3`, 2026-08-27 | [releases](https://github.com/Maxteabag/sqlit/releases), [PyPI JSON API](https://pypi.org/pypi/sqlit-tui/json) |
| PyPI package | `sqlit-tui`, requires Python ≥ 3.10 | [PyPI](https://pypi.org/project/sqlit-tui/) |
| Last push | 2026-09-03 | GitHub API `pushed_at` |
| Launch | Show HN 2025-12-15, 190 points, 42 comments | [news.ycombinator.com/item?id=46276002](https://news.ycombinator.com/item?id=46276002) |

Positioning, verbatim from the README: "The lazygit of SQL databases" and "Connect and query your database from your terminal in seconds." ([README](https://github.com/Maxteabag/sqlit/blob/v1.6.3/README.md))

The relevant claim for us is the FAQ answer: sqlit is "inspired by lazygit - you can just jump in and there's no need for external documentation." ([README FAQ](https://github.com/Maxteabag/sqlit/blob/v1.6.3/README.md)) That is the same bet lazysnap is making, one layer down the stack.

Note the asymmetry with lazygit itself. lazygit's README says only "Call `lazygit` in your terminal inside a git repository" and then points at [external keybinding docs](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings) and a [config guide](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md). lazygit gets away with zero config because *git already put the config on disk* — a repo is self-describing. sqlit has no equivalent, so it had to **manufacture** the missing context by inspecting the machine. That manufacturing is what the rest of this document is about, and it is exactly lazysnap's problem too.

---

## 2. How sqlit achieves a zero-config first run

### 2.1 Docker container detection — the code path

This is the mechanism that makes `sqlit` useful on a machine it has never seen. It lives in one file, [`sqlit/domains/connections/discovery/docker_detector.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py) (419 lines), and it is worth walking end to end because every design choice in it is one lazysnap will have to make.

**Step 1 — is Docker there at all, and in which of four ways is it not?** ([lines 86–107](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L86-L107))

```python
def get_docker_status() -> DockerStatus:
    """Check if Docker is available and running."""
    try:
        import docker
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

Two things to steal. First, the failure modes are a **four-valued enum**, not a boolean — `AVAILABLE`, `NOT_RUNNING`, `NOT_INSTALLED`, `NOT_ACCESSIBLE` ([lines 18–24](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L18-L24)) — because each one has a different remedy and the UI renders a different sentence for each ([controller, lines 41–54](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/controllers/docker.py#L41-L54)). Second, `docker.from_env()` inherits the user's existing Docker configuration — [docker-py documents it as reading `DOCKER_HOST`, `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH`](https://docker-py.readthedocs.io/en/stable/client.html), the same variables the Docker CLI honours ([`dockerd` reference](https://docs.docker.com/reference/cli/dockerd/): "The Docker client honors the `DOCKER_HOST` environment variable"). So a user on a remote or rootless daemon gets detection for free, with no sqlit-specific setting. The default is the unix socket: "By default, a `unix` domain socket (or IPC socket) is created at `/var/run/docker.sock`, requiring either `root` permission, or `docker` group membership" ([same page](https://docs.docker.com/reference/cli/dockerd/)) — which is precisely why `NOT_ACCESSIBLE` needs its own message.

**Step 2 — which containers are databases?** Detection is delegated to whichever provider claims the image name ([lines 110–122](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L110-L122)):

```python
def _get_db_type_from_image(image_name: str) -> str | None:
    for db_type, detector in _iter_docker_detectors():
        if detector.match_image(image_name):
            return db_type
    return None
```

`_iter_docker_detectors()` walks the provider catalog and collects every provider that declares one ([lines 74–83](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L74-L83)). A detector is a frozen dataclass of pure data — image substrings, environment variable names, defaults ([`providers/docker.py`, lines 20–51](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/docker.py#L20-L51)):

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
```

Postgres's whole detection rule is seven lines of declaration inside its `ProviderSpec` ([lines 27–35](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/postgresql/provider.py#L27-L35)):

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

Those variable names and that default are not invented: the [official `postgres` image](https://hub.docker.com/_/postgres) documents `POSTGRES_PASSWORD`, `POSTGRES_USER` ("If it is not specified, then the default user of `postgres` will be used") and `POSTGRES_DB` ("If it is not specified, then the value of `POSTGRES_USER` will be used"). sqlit's `get_credentials` implements exactly that fallback chain, including the `database → default_database` fallback ([lines 34–51](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/docker.py#L34-L51)).

**Step 3 — where is it actually listening?** This is where the real complexity is, and it is instructive ([lines 279–298](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L279-L298)). The port is resolved by a cascade, not a lookup:

1. Look for a host binding of the provider's declared default container port (`5432/tcp`) in `NetworkSettings.Ports`.
2. If that fails, and the container publishes exactly **one** TCP host port, use it ([`_get_single_mapped_host_binding`, lines 156–170](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L156-L170)) — this catches `-p 5433:5432` and non-standard internal ports.
3. If still nothing and `HostConfig.NetworkMode == "host"`, fall back to the single exposed TCP port from `Config.ExposedPorts`, else the provider default.

And then the subtle one, `_resolve_published_host` ([lines 173–183](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L173-L183)): when a provider forces an IPv4 loopback but Docker published on `::`, the detector rewrites the host so the address family matches what Docker actually bound. That code exists because of a real bug — [PR #279, "Fix MSSQL Docker discovery on IPv4 loopback"](https://github.com/Maxteabag/sqlit/pull/279), whose author reports: "Docker discovery found the mapped port and credentials, but `localhost` resolved to `::1` while SQL Server was published on IPv4 loopback, so the connection failed." Followed by [PR #280, "Preserve Docker-published address families"](https://github.com/Maxteabag/sqlit/pull/280).

**Lesson for lazysnap: the hard part of container discovery is not finding the container, it is producing a host:port that actually connects.** Budget for the cascade and for `localhost` ≠ `127.0.0.1` ≠ `::1`.

**Step 4 — stopped containers are shown, not hidden.** `detect_database_containers()` scans `running` and `exited` separately and returns running first ([lines 337–362](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L337-L362)). Non-connectable entries are rendered disabled with a reason — `(not exposed)` or `(Stopped)` ([`tabs/docker.py`, lines 117–139](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/tabs/docker.py#L117-L139)). Showing the thing you cannot use, greyed, with the reason, is strictly better than an empty list: it answers "did it not find my database, or is my database not running?"

**Step 5 — the whole scanner is injectable.** `DockerScanProtocol` is a callable returning `(status, containers)`, with a real `DockerContainerScanner` and a `StaticDockerContainerScanner` that replays a fixed list ([lines 68–71, 365–381](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L365-L381)), wired through `AppServices.docker_detector` ([`services.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/app/services.py#L136)). That is why [`tests/test_docker_detector.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/tests/test_docker_detector.py) can assert image-pattern and credential behaviour with no daemon, while `tests/integration/docker_detect/` spins up real containers behind an opt-in `--run-docker-container` flag ([CONTRIBUTING](https://github.com/Maxteabag/sqlit/blob/v1.6.3/CONTRIBUTING.md)).

### 2.2 Connection saving

The important decision: **connecting and saving are separate actions.**

- `Enter` on a discovered container builds a config in memory and connects; nothing is written ([`screen.py`, `action_select`, lines 409–447](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/screen.py#L409-L447)).
- `s` explicitly saves, and only appears in the footer when the highlighted container is not already saved ([`shortcuts.py`, lines 88–106](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/shortcuts.py#L88-L106); [`action_save`, lines 511–536](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/screen.py#L511-L536)).

A saved config records `source="docker"` ([`container_to_connection_config`, lines 384–419](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L384-L419)), and that provenance tag is then used to decide whether a discovered container is "the same as" a saved connection — matching on host/port/database is only allowed for docker-sourced entries ([`is_container_saved`, lines 29–53](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/tabs/docker.py#L29-L53)). Deduplication is provenance-aware, not string-matching. Names are made unique on save rather than rejected ([`ensure_unique_name`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/save_connection.py#L34-L43)).

Storage: `connections.json` under the config dir, versioned (`{"version": 2, "connections": [...]}`) with silent best-effort migration from the v1 bare-list format ([`store/connections.py`, lines 100–124](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/store/connections.py#L100-L124)). The config dir is `$XDG_CONFIG_HOME/sqlit` (default `~/.config/sqlit`, matching the [XDG base directory spec](https://specifications.freedesktop.org/basedir-spec/latest/): "If `$XDG_CONFIG_HOME` is either not set or empty, a default equal to `$HOME`/.config should be used"), overridable with `SQLIT_CONFIG_DIR`, with a one-time migration from the legacy `~/.sqlit` ([`shared/core/store.py`, lines 25–72](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/core/store.py#L25-L72)).

There is also a **project mode** that lazysnap should copy outright: a positional path argument (`sqlit .`) routes connections, history and starred queries into `<project>/.sqlit/` instead of the global config, detected before argparse by a deliberately conservative path test ([`cli.py`, lines 40–95](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/cli.py#L40-L95)) and advertised in the parser epilogue.

### 2.3 Keyring use

`connections.json` never contains passwords. `_config_to_dict_without_passwords` strips them before write, and `None` is the sentinel meaning "fetch from the credentials service on next load" ([lines 224–243](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/store/connections.py#L224-L243)).

The service is an ABC with three implementations and a **probe-first selection function** ([`credentials.py`, lines 457–476](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L457-L476)):

```python
def build_credentials_service(settings_store=None) -> CredentialsService:
    ...
    if allow_plaintext is True:
        return PlaintextFileCredentialsService()
    if is_keyring_usable():
        return KeyringCredentialsService()
    return PlaintextCredentialsService()   # in-memory, NOT persisted
```

Note the fallback ordering. If the keyring is unusable, sqlit falls back to **in-memory** (secrets lost on exit), never silently to disk; on-disk plaintext requires the user to have set `allow_plaintext_credentials` ([line 27](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L27)). The probe itself is defensive: it rejects the `keyring.backends.fail` backend and any backend with priority ≤ 0, retries three times with a 100 ms delay for transient D-Bus/keychain flakiness, and does a real read against a random probe key ([lines 89–122](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L89-L122)). Reads retry twice more at call time ([`_get_with_retry`, lines 264–290](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L264-L290)). Keys are `"<connection name>:db"` / `"<connection name>:ssh"` under service name `"sqlit"` ([lines 24, 252–262](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/credentials.py#L252-L262)).

A failed probe is surfaced, not swallowed — at startup the app warns "Keyring unavailable: {error}. Saved passwords may not load." for 15 seconds, or prints to stderr when headless ([`startup_flow.py`, lines 152–165](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/app/startup_flow.py#L152-L165)).

The most transferable detail is `save_one` ([lines 265–368](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/store/connections.py#L265-L368)): renaming a connection is treated as a two-phase credential move — write the new keyring entries first, then rewrite the index, then delete the old entries, with explicit rollback (`_restore_credentials`) if any step fails. It exists because editing one connection used to rewrite every password ([PR #289](https://github.com/Maxteabag/sqlit/pull/289), [PR #290](https://github.com/Maxteabag/sqlit/pull/290)). Keyring writes are slow and can fail per-item; treat them as a transaction.

Worth recording the causal history: sqlit shipped without a keyring. The Show HN comment that changed it — "Is there a way to log in some way that doesn't leave your credentials saved in a text file...?" — got the reply "In the next release I am going to use Keyring to store credentials on the operating system's credential store" ([HN thread](https://news.ycombinator.com/item?id=46276002)). Credential storage was the single most-raised objection at launch. lazysnap connects to *production* databases; it will be raised harder.

### 2.4 Keybinding discoverability

Three layers, in increasing cost to the user:

1. **A context footer, always visible.** `ContextFooter` renders `label: key` pairs, left and right groups, greying out unavailable ones ([`widgets_footer.py`, lines 20–80](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/ui/widgets_footer.py#L20-L80)). It is rebuilt from the *current selection*, not from a static list — the connection picker recomputes `Connect`/`Select`, `Save`, `New`, `Refresh` per highlighted row ([`shortcuts.py`, lines 58–110](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/screens/connection_picker/shortcuts.py#L58-L110)). The verb changes to match what Enter will actually do.
2. **A leader menu on `<space>`** for the second tier of commands ([`config/keymap.template.json`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/config/keymap.template.json); [`core/leader_commands.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/core/leader_commands.py)), with per-command guards so unavailable commands are hidden rather than failing.
3. **`?` help**, which reorders itself to put the section for the currently-focused pane first, and supports `/` to filter ([`help.py`, lines 17–33, 74–83](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/ui/screens/help.py#L17-L33)).

Underneath, every binding is data. `LeaderCommandDef` and `ActionKeyDef` are dataclasses ([`core/keymap.py`, lines 45–68](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/core/keymap.py#L45-L68)), defaults ship as `config/keymap.template.json`, users override by key in `~/.config/sqlit/keymap.json`, and there is an in-app keybinding editor that writes that file and applies changes live without a restart ([`keybinding_editor.py`, lines 1–10](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/ui/screens/keybinding_editor.py#L1-L10)). Display formatting is centralised in `format_key` ([lines 11–42](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/core/keymap.py#L11-L42)) so `?`, the footer and the help screen render the same key the same way.

**The failure to learn from.** The top UX comment on Show HN was that the `<space>` menu's commands were "not reflected properly in the help text shown when you press ?", so the reporter activated fullscreen and then could not work out how to undo it, and could not find quit ([HN](https://news.ycombinator.com/item?id=46276002)). The author agreed and fixed it in the next release. The root cause is a second source of truth: a hand-maintained help text next to a data-driven keymap. lazygit avoids this by generating its keybindings document mechanically — the file opens with "_This file is auto-generated. To update, make the changes in the pkg/i18n directory and then run `go generate ./...` from the project root._" ([Keybindings_en.md](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md)), and its global table includes `` ? `` → "Open keybindings menu". lazysnap should generate help from the binding table in CI and fail the build on drift.

### 2.5 Autocomplete

A hand-written SQL completion engine, ~2,275 lines under [`sqlit/domains/query/completion/`](https://github.com/Maxteabag/sqlit/tree/v1.6.3/sqlit/domains/query/completion), with a dispatch module per statement shape (`create_table.py`, `alter_table.py`, `insert.py`, `update.py`, `delete.py`, `drop.py`, `truncate.py`, `create_index.py`, `create_view.py`). Suggestions are typed (`TABLE`, `COLUMN`, `KEYWORD`, `FUNCTION`, `SCHEMA`, `DATABASE`, `PROCEDURE`, `PARAMETER`, `ALIAS_COLUMN`, `OPERATOR`) and carry a `table_scope` so a column suggestion knows which alias it belongs to ([`completion/core.py`, lines 16–36](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/completion/core.py#L16-L36)).

The interaction rules matter more than the engine:

- It triggers on typing, not on a key — but only in vim INSERT mode, only in the query pane, and only when connected ([`autocomplete.py`, lines 117–167](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete.py#L117-L167)).
- 100 ms debounce, cancelled on each keystroke ([lines 160–167](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete.py#L160-L167)).
- Cursor movement without a text change dismisses it ([lines 233–247](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete.py#L233-L247)).
- Several explicit suppression latches — after accepting a completion, after Enter dismissed the dropdown, so a newline does not immediately re-open it ([lines 136–152](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete.py#L136-L152)).
- Column metadata for tables mentioned in the query is preloaded on an **idle scheduler at LOW priority**, cancelling any previous preload ([lines 189–202](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/query/ui/mixins/autocomplete.py#L189-L202)) — schema introspection never blocks a keystroke.
- `Tab` accepts; the README states the rule in one line: "Autocomplete triggers automatically in INSERT mode. Use `Tab` to accept."

For lazysnap the transferable part is not SQL completion (we barely need it) but the pattern: **speculative metadata loading during idle time, cancellable, never on the input path**, and dismissal rules that are stricter than the trigger rules.

### 2.6 How it presents errors

Four mechanisms, chosen by severity and by whether the error is *actionable*.

**(a) Severity routing in one place.** `notify()` sends information and warnings to the status bar with a timestamp; errors get pushed into the results table *and* raised as a modal ([`ui_status.py`, lines 387–439](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/ui/mixins/ui_status.py#L387-L439)). The modal is suppressed if another modal is already open, so errors never stack.

**(b) Errors are copyable.** `ErrorScreen` binds `y` to copy the message and flashes the widget to confirm ([`shared/ui/screens/error.py`, lines 13–70](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/ui/screens/error.py#L13-L70)). Its own footer shows `Copy` and `Close` — an error dialog that teaches its own keys.

**(c) Actionable errors are routed to handlers, not rendered.** This is the best idea in the codebase. Before any error reaches a dialog it passes through a handler registry ([`connection_error_handlers.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/connection_error_handlers.py)):

```python
if handle_connection_error(self, error, config):
    return
self.push_screen(ErrorScreen("Connection Failed", str(error)))
```
([`ui/mixins/connection.py`, lines 398–401](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/mixins/connection.py#L398-L401))

Two handlers ship. `MissingDriverHandler` catches `MissingDriverError`, caches the pending connection so it can auto-reconnect after the restart, and opens a package-setup screen ([lines 30–53](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/connection_error_handlers.py#L30-L53)). `AzureFirewallHandler` parses the client IP out of the error text, looks up the server, and offers to add the firewall rule, then retries the connection ([lines 56–111](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/ui/connection_error_handlers.py#L56-L111)). **An error that has a known fix is presented as the fix, not as a message.**

**(d) The environment is diagnosed before the command is offered.** `install_strategy.py` detects how sqlit itself was installed — `pipx`, `uv tool`, `uvx`, `uv run`, `conda`, `pip` — probes for [PEP 668](https://peps.python.org/pep-0668/) externally-managed environments, user-site availability and writable install paths, and maps PyPI names to Arch package names, before deciding whether it can auto-install or must print an exact manual command ([`install_strategy.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/install_strategy.py)). The README's driver table gives per-database `pipx inject` and `pip install` commands for 20 databases.

This whole subsystem exists because of first-run friction reported at launch. Two separate HN commenters hit "Connection failed, no module named 'psycopg2'" and needed thread replies to recover; a third filed [issue #133](https://github.com/Maxteabag/sqlit/issues/133) after `uvx sqlit-tui` failed because the console script was named `sqlit`. The fix for the latter is one line — the project now declares **both** entry points ([`pyproject.toml`, lines 172–175](https://github.com/Maxteabag/sqlit/blob/v1.6.3/pyproject.toml#L172-L175)):

```toml
[project.scripts]
sqlit = "sqlit.cli:main"
sqlit-tui = "sqlit.cli:main"
```

**(e) CLI errors get the remedy inline.** `sqlit docker` prints "Error: Docker is not accessible (permission denied)." followed by "Try adding your user to the docker group or running with sudo." ([`cli/commands.py`, lines 355–380](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/cli/commands.py#L355-L380)).

---

## 3. What made sqlit easy to adopt

**Repo structure.** Domain-first (`sqlit/domains/{connections,explorer,query,results,shell,process_worker}/`), each with `app/` (services), `domain/` (pure logic), `ui/`, `state/`, `store/`. Discovery, providers and UI screens are separated, so the Docker detector is testable with no Textual import. The provider system is the adoption engine: a new database is one directory under `providers/` containing a `provider.py` that calls `register_provider(SPEC)`, auto-imported by `pkgutil.iter_modules` at startup ([`catalog.py`, lines 22–41](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/providers/catalog.py#L22-L41)). A contributor adding Postgres-adjacent support edits **no central file**. 29 provider directories and 38 contributors are the result.

**README.** One-line install (`pipx install sqlit-tui`) above the fold, before any prose. Feature sections are one line of text plus an animated GIF (`docs/demos/`). Five install methods including AUR and a Nix flake. A "Try it without a database" section (`sqlit --mock=sqlite-demo`, backed by real mock profiles in [`mock_profiles.py`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/app/mock_profiles.py#L269-L290)). A full keybinding table in the README itself. An honest FAQ comparing against Harlequin and Lazysql. A driver-install matrix for `pipx`/`pip` per database.

**Release process.** Tag `v*` (or `workflow_dispatch`) triggers one workflow: create a GitHub release with `generate_release_notes: true`, build with `python -m build`, publish to PyPI via **Trusted Publishing** (`environment: pypi`, `permissions: id-token: write` — no long-lived token), then push the updated `PKGBUILD` to the AUR. There is an `aur_only` input to re-push AUR from an existing tag without re-releasing ([`.github/workflows/release.yml`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/.github/workflows/release.yml)). Version is derived from the tag by `hatch-vcs`, so there is no version string to forget. 61 tags in nine months.

**CI.** Build across Python 3.10–3.13, a `nix build .#sqlit` job, and — cheap and disproportionately valuable — a "Verify CLI entry point" step that just runs `python -c "from sqlit.cli import main"` ([`.github/workflows/ci.yml`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/.github/workflows/ci.yml)). Unit tests run under `uv` with a 60 s per-test timeout.

**Contributor docs.** [`CONTRIBUTING.md`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/CONTRIBUTING.md) gives a two-command dev setup, a tiered test story (SQLite tests need nothing; the full suite is one `docker compose -f infra/docker/docker-compose.test.yml up -d` with an `enterprise` profile for the heavy databases), a `pytest -k` line per database, and an environment-variable table with defaults for every database. Docker-detection tests that create containers are opt-in behind `--run-docker-container`. The file also carries the product **vision** — the CEQR (Connect, Explore, Query, Results) and EAFF (Easy, Aesthetic, Fun, Fast) filters, plus explicit rules like "sqlit should not require any external documentation at all", "There should be no settings or preferences with important exception of interface", and a keybinding decision hierarchy (intuitive → harmony → vim tradition). A contributor can predict whether a PR will be accepted before writing it. That is what keeps 38 contributors from pulling the tool apart.

---

## 4. What lazysnap should not copy

- **A second source of truth for help text.** The `?`-vs-`<space>` drift was the top usability complaint at launch. Generate.
- **A settings file.** sqlit's own vision document argues against settings; sqlit nonetheless has `settings.json` with `allow_plaintext_credentials`, mock flags, watchdog thresholds and process-worker tuning. Every one of those is a first-run question waiting to happen.
- **Breadth before depth.** 29 providers, 5 cloud discovery backends, SSH tunnels, vim text objects. CONCEPT.md already rules this out (Postgres only for v1); the study confirms the cost — the provider registry is elegant precisely because it had to be.
- **Silent best-effort persistence.** Migration failures, keyring delete failures and validation errors are swallowed with bare `except: pass` in several places (e.g. [`_migrate_connections_payload`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/store/connections.py#L117-L124), [`container_to_connection_config`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/connections/discovery/docker_detector.py#L415-L418)). For a tool whose output is a dataset, a swallowed error is a data-integrity bug.
- **A TUI-first mental model.** `sqlit` with no arguments does *not* open the connection picker; it mounts the app with the tree focused and waits for `<space>c` ([`startup_flow.py`, lines 82–99](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/domains/shell/app/startup_flow.py#L82-L99)). That is one keystroke of nothing. lazysnap's no-argument run should *do the work*.

---

## 5. lazysnap's equivalent, step by step

### 5.0 Ground rules this design must satisfy

From CONCEPT.md: at most one question on the happy path; configuration is emitted after a run, never demanded before it; masking on by default; never write to the source; one static binary; the same command works headless in CI; every TUI action reachable by a CLI flag first.

Consequence: **no file is created anywhere before the first run completes.** Discovery must be entirely inferential.

### 5.1 The discovery ladder

`lazysnap` with no arguments, in directory `D`, runs these in order and merges results into one candidate set. Each candidate carries a `provenance` string that is printed and stored, never a secret.

| # | Source | Yields | Provenance token |
|---|---|---|---|
| 0 | `./lazysnap.yml` | a complete previous run | `file:lazysnap.yml` |
| 1 | `--source` / `--target` flags, `LAZYSNAP_SOURCE` / `LAZYSNAP_TARGET` | explicit endpoints | `flag`, `env:LAZYSNAP_SOURCE` |
| 2 | Docker Engine API over `DOCKER_HOST` (default `unix:///var/run/docker.sock`) | running + exited postgres containers with real published ports and `POSTGRES_*` env | `docker:<container name>` |
| 3 | `compose.yaml` / `compose.yml` / `docker-compose.yml` / `docker-compose.yaml` in `D` | service names, images, `environment`, `env_file`, `ports` | `compose:<service>` |
| 4 | `.env`, `.env.local`, `.env.development` in `D` | `DATABASE_URL`, `POSTGRES_URL`, `TEST_DATABASE_URL`, `DATABASE_URL_TEST`, `SHADOW_DATABASE_URL`, `PG{HOST,PORT,USER,DATABASE}` | `env-file:.env#DATABASE_URL` |
| 5 | Process environment | same variable names | `env:DATABASE_URL` |
| 6 | `postgresql://localhost:5432` | a local server, if it answers | `local` |

Justification for each rung:

- **Rung 0 first** because a repeat run must ask nothing. This is the CI path.
- **Rung 2 before rung 3** because a running container is ground truth: its published port and its resolved environment are facts, whereas a Compose file's `ports` may be unpublished and its `environment` may contain unresolved `${VAR}` interpolation. sqlit's port cascade ([§2.1](#21-docker-container-detection--the-code-path)) is copied wholesale, including the address-family fix.
- **Rung 3 at all** because a Compose file gives us *human names* ("db", "db-test") and tells us what *should* exist — which is what lets us say "compose lists service `db` (postgres:16) but nothing is listening; run `docker compose up -d db`" instead of "no database found". The [Compose spec](https://docs.docker.com/reference/compose-file/services/) defines `image`, `environment` (map or `KEY=VALUE` array), `env_file` and `ports` (short `[HOST:]CONTAINER[/PROTOCOL]` or long `target`/`published`/`host_ip`), which is everything we need. [`compose-spec/compose-go`](https://github.com/compose-spec/compose-go) (Apache-2.0, actively maintained, last push 2026-09-03) is the reference parser if we go Go.
- **Rung 4** because `DATABASE_URL` in `.env` is the dominant convention: the [twelve-factor](https://12factor.net/config) rule is "The twelve-factor app stores config in _environment variables_", with "Resource handles to the database" named explicitly, and [Prisma](https://www.prisma.io/docs/orm/reference/connection-urls) ships `DATABASE_URL=postgresql://janedoe:mypassword@localhost:5432/mydb` in `.env` as its canonical example. Values parse as [PostgreSQL connection URIs](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS): `postgresql://[userspec@][hostspec][/dbname][?paramspec]`, both `postgresql://` and `postgres://` schemes. [`joho/godotenv`](https://github.com/joho/godotenv) (MIT, 10.6k stars, last push 2026-08-04) is the reference dotenv parser.
- **Rung 6** last, and only probed, never assumed.

Every candidate is then **probed**: connect, `SELECT 1`, count user tables. A candidate that fails to connect is kept in the report as unusable with its error, in the spirit of sqlit's greyed-out "(not exposed)" rows.

### 5.2 Choosing source and target without asking

Candidates are split by a deterministic rule, and the decision is **printed** so it can be contradicted:

1. **Target eligibility (safety gate, evaluated first).** A candidate may be a target only if it is (a) reachable, (b) not the chosen source, (c) on loopback, or a container published to loopback, or explicitly passed with `--target`, and (d) either empty of user tables *or* carrying a `lazysnap_meta` marker table from a previous lazysnap run. Anything else is ineligible and must be confirmed (Q3).
2. **Source ranking.** Prefer, in order: the candidate with the most user tables; then the one whose name lacks a `test`/`ci`/`shadow`/`tmp` marker; then the one named by `DATABASE_URL` over one found by container scan; then lexical order on provenance for determinism.
3. **Target ranking** among eligible candidates: prefer a candidate that shares a Compose project with the source; then an empty one; then a `*_test` / `*-test` name; then the highest port.

This is what makes CONCEPT.md's example resolve with zero questions: `shop-db` has 41 tables, `shop-db-test` has none, so source and target fall out of the counts, not out of a prompt.

**Never** does the source get written to; the source connection is opened with `default_transaction_read_only=on` and the tool refuses to proceed if the session is not read-only.

### 5.3 Scenario A — a folder with `docker-compose.yml`

```
$ lazysnap
  docker: 2 postgres containers running
    shop-db        postgres:16   127.0.0.1:5432   41 tables
    shop-db-test   postgres:16   127.0.0.1:5433   empty
  source shop-db → target shop-db-test        (--source/--target to change)
  41 tables · 17 columns look like personal data   (? to see why)
  root table? [customers]
```

1. Probe the Docker daemon (rung 2). Both containers are found; credentials come from `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` in each container's `Config.Env`, so **no password is asked for**.
2. Parse the Compose file (rung 3) only to attach service names and to notice services that are declared but not running. A declared-but-down service is reported, not fatal.
3. Probe both, count tables, apply §5.2. Print the decision on one line with the flags that override it.
4. Introspect the source: tables, keys, FKs, samples; classify personal-data columns.
5. **Ask Q1 (root table).** One question. Enter accepts the default.
6. Plan, extract, mask, load, verify, write `lazysnap.yml`.

Degradations, none of which add a question:
- Daemon not running / not installed / permission denied → four-valued status like sqlit's, printed as one line, and discovery **continues** to rungs 3–6.
- Compose service declared but not up → `docker compose up -d db` printed as the next action.
- Container running but port not published → shown as unusable with `(not exposed)` and the `ports:` line it would need.

### 5.4 Scenario B — a folder with `DATABASE_URL` in `.env`

```
$ lazysnap
  .env: DATABASE_URL → postgres@db.internal:5432/shop   41 tables   (read-only)
  no local target found
  target? [start postgres:16 on 127.0.0.1:55432 (throwaway container)]
  root table? [customers]
```

1. Rungs 2 and 3 find nothing (no daemon containers, no compose file), rung 4 finds `DATABASE_URL`.
2. Parse it as a Postgres URI. If it carries a password, use it for this run only. If it does not: try the OS keyring, then `PGPASSWORD`, then `~/.pgpass`, then prompt — the prompt is **not** counted against the happy-path budget because a run against a password-protected remote database that the user has not authenticated to is by definition not the happy path.
3. Look for a target in this order: `TEST_DATABASE_URL` / `DATABASE_URL_TEST` / `SHADOW_DATABASE_URL` in the same `.env`; then any eligible rung-2/3/6 candidate; then none.
4. If a target was found → one question (Q1), exactly as scenario A.
5. If no target was found → **Q2 (target)** then **Q1 (root)**. Two questions, and this is the documented cost of a project that has a production URL and no local database. Q2's default, when Docker is available, is to start a throwaway `postgres:<same major as source>` published on a free loopback port, labelled `lazysnap.throwaway=true`; when Docker is not available the default is `postgresql://localhost:5432/<dbname>_local` if a local server answered, and otherwise Q2 has no default and the run exits with the two commands that would create one.

Credential policy, deliberately different from sqlit: **a typed password is used for the run and not persisted.** `lazysnap.yml` records the *provenance* (`source: env-file:.env#DATABASE_URL`, or `source: docker://shop-db`), never a secret, so the next run re-derives it the same way. The OS keyring is written only when the user opts in with `lazysnap remember` / `--save-credentials`, in which case we mirror sqlit's arrangement: [`zalando/go-keyring`](https://github.com/zalando/go-keyring) (MIT, actively maintained, last push 2026-07-24) or [`99designs/keyring`](https://github.com/99designs/keyring) (MIT, but last push 2024-05-07 — prefer the former), a service name of `lazysnap`, a key of `<provenance fingerprint>`, a usability probe before use, and an **in-memory** fallback rather than a silent plaintext file. Rationale: lazysnap runs unattended in CI far more often than sqlit does, and a secret written to a developer's keychain without being asked is exactly the "wait, how did it know?" moment sqlit's own vision document warns against.

### 5.5 Scenario C — a folder with nothing

```
$ lazysnap
  no postgres found.
    docker      not running
    compose     no compose file in this directory
    .env        no DATABASE_URL
    localhost   nothing listening on 5432

  next:
    lazysnap --source postgresql://user@host/db   point it at a database
    lazysnap --demo                               run the whole pipeline on a bundled fixture
    docker compose up -d                          if this project has a stack elsewhere
  exit 3
```

No questions. A **discovery report**, not an error message: each rung says what it looked for and what it found, which turns "it doesn't work" into "ah, my daemon is off". `--demo` is the direct analogue of `sqlit --mock=sqlite-demo` — a bundled pair of fixture databases the whole pipeline runs against, so a curious user gets to see the output before owning a database.

### 5.6 Scenario D — a second run (`lazysnap.yml` present)

Zero questions, always, interactive or not. The file is read, the provenance tokens are re-resolved (the container may have a new port; the `.env` may have a new URL), the plan is replayed, and the run either reproduces or fails loudly if the schema drifted. `--reconfigure` forces the ladder to run again.

### 5.7 The complete question catalogue

Every question the tool may ever ask, its default, and when it fires. **On the happy path exactly one of these fires: Q1.**

| ID | Question | Default | Fires when | Non-interactive behaviour | CLI equivalent |
|---|---|---|---|---|---|
| Q1 | `root table? [customers]` | Highest-ranked table: most inbound foreign keys, tie-broken by a known-root name list (`customers`, `users`, `accounts`, `organizations`, `tenants`, `companies`), then by estimated row count | Always, unless `--root` or `lazysnap.yml` supplies it | Uses the default and prints it | `--root customers` |
| Q2 | `target? [start postgres:16 on 127.0.0.1:55432]` | Start a throwaway labelled container if Docker is available; else a reachable local server; else **no default** | No target candidate passed the §5.2 eligibility gate | If a default exists, use it and print it; if not, exit 4 with the two commands that would create one | `--target postgresql://…` |
| Q3 | `target shop_local has 12 tables and no lazysnap marker. replace its contents? [no]` | **no** | Chosen target is non-empty and unmarked | Never auto-answered. Exit 5, naming three of the tables and the flag | `--replace-target` |
| Q4 | `password for postgres@db.internal:5432:` | none (no echo) | Source or target needs a password that keyring, `PGPASSWORD` and `~/.pgpass` did not supply | Exit 6, naming the provenance of the connection and the three places it looked | `PGPASSWORD`, `~/.pgpass`, `--save-credentials` |
| Q5 | `source is a remote host (db.internal). read 500 customers from it? [yes]` | **yes** | Source is not loopback and not a local container | Uses the default | `--allow-remote-source` / `--no-allow-remote-source` |
| Q6 | `2 sources look equally likely (shop-db, legacy-db). which? [shop-db]` | Highest-ranked by §5.2 | §5.2 ranking is an exact tie on every criterion | Uses the default and prints the tie | `--source` |

Design rules encoded in that table:

- **Every question has a CLI equivalent**, satisfying "the TUI is a thin layer" and "the same command a human types works headless in CI".
- **Every default is printed in the prompt**, so a user who hits Enter knows what they agreed to.
- **Exactly one question has no safe default (Q3)**, and it is the only destructive one. It never auto-answers, in interactive or non-interactive mode; the escape hatch is a flag whose name says what it does, as CLAUDE.md requires.
- **Masking is never a question.** Column classification is reported (`17 columns look like personal data (? to see why)`) and opt-outs are edited into `lazysnap.yml` after the fact.
- Non-interactive is detected by `!isatty(stdin)` **or** `CI` being set — [GitHub Actions documents `CI` as "Always set to `true`"](https://docs.github.com/en/actions/reference/workflows-and-actions/variables) — or `--non-interactive`. In that mode no prompt is ever printed; each question resolves to its default or to a distinct non-zero exit code.

### 5.8 Where state lives

| Thing | Location | When written |
|---|---|---|
| Run record | `<project>/lazysnap.yml` | After a successful run. Committed. Contains provenance tokens, root table, caps, masking decisions — never a secret. |
| Project-local state | `<project>/.lazysnap/` | Only if the user opts into project mode (`lazysnap .`), copying sqlit's [project routing](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/cli.py#L40-L95) |
| Global config | `$XDG_CONFIG_HOME/lazysnap/` (default `~/.config/lazysnap`), overridable by `LAZYSNAP_CONFIG_DIR` | Only when the user saves something |
| Secrets | OS keyring, service `lazysnap` | Only on `--save-credentials` |
| Nothing | — | Before the first run completes |

### 5.9 Error presentation

Copy sqlit's handler registry ([§2.6c](#26-how-it-presents-errors)) and make it the only path. Every error is a typed value carrying: **what we tried**, **the provenance of the inputs we used**, and **one runnable command**. Provenance is the addition sqlit lacks and lazysnap needs — "auth failed for `postgres@127.0.0.1:5432`, password came from container `shop-db`'s `POSTGRES_PASSWORD`" is a different debugging session from "auth failed".

| Error | Message names | Offered action |
|---|---|---|
| Docker socket absent | which of the four states | continue down the ladder; never fatal |
| Docker permission denied | the socket path | `sudo usermod -aG docker $USER` |
| Compose service down | service, image | `docker compose up -d <service>` |
| Container port unpublished | container, internal port | the `ports:` line to add |
| Auth failed | user, host, **credential provenance** | `--save-credentials`, or the `.env` key to fix |
| Source not read-only | the session setting | refuse; this is a bug in lazysnap, not the user |
| Target non-empty | three table names | `--replace-target` |
| FK verification failed | constraint, child table, orphan count | `--include <table>` or a cap to raise |
| Anything unexpected | one line | `--debug` for the stack; the message is copyable |

No stack traces by default. Distinct exit codes per class so CI can branch.

### 5.10 Keybinding and flag discoverability

- The run prints a **footer line** of the keys live during the run — `? why masked · s skip a table · q quit` — rebuilt from the same table that defines the bindings, in the spirit of sqlit's [`ContextFooter`](https://github.com/Maxteabag/sqlit/blob/v1.6.3/sqlit/shared/ui/widgets_footer.py#L20-L80).
- `?` during a run expands the personal-data classification inline (column, why, mask strategy), which is the promise CONCEPT.md already makes.
- `--help` groups flags by pipeline stage (discover / classify / plan / extract / load / verify), not alphabetically.
- `docs/KEYBINDINGS.md` and the flag reference are **generated** from the binding and flag tables, with a CI check that fails on drift. This is lazygit's arrangement ("_This file is auto-generated… run `go generate ./...`_") and is the direct fix for the `?`-vs-`<space>` drift that was sqlit's top launch complaint.

### 5.11 Adoption checklist lifted from sqlit

Not part of the first-run spec, but the study's other half:

- Two console entry points if we ever ship anything but a static binary; the `uvx sqlit-tui` failure ([#133](https://github.com/Maxteabag/sqlit/issues/133)) cost real users.
- One-line install above the fold in the README; GIF per capability; `--demo` documented in the first screenful.
- Tag-triggered release with generated notes and OIDC publishing, no long-lived tokens.
- A CI job that does nothing but confirm the binary starts and `lazysnap --version` works.
- `CONTRIBUTING.md` carries the *vision filter*, not just the build commands — sqlit's CEQR/EAFF section is why 38 contributors did not turn it into DBeaver.
- Fixtures as a `docker compose` file plus a documented env-var table with defaults, and container-creating tests behind an opt-in flag.

---

## 6. Gaps and unverified points

- **Reddit is unreachable from this environment**, so community sentiment is sampled only from HN and GitHub. The Show HN thread is nine months old; later friction may be recorded in places I could not read.
- **No public write-up by sqlit's author** on the Docker-detection design exists that I could find; §2.1 is reconstructed from source and from PRs #279/#280.
- **The Docker Engine API reference page does not itself restate the default socket path**; that claim is sourced from the [`dockerd` CLI reference](https://docs.docker.com/reference/cli/dockerd/) instead.
- **Whether docker-py honours `DOCKER_CONTEXT`** (as opposed to `DOCKER_HOST`) is unverified; the documented set is `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`. If lazysnap is written in Go this matters, because Docker contexts are common on macOS with Colima/OrbStack. **Flagged for the implementation phase.**
- **§5 is a proposal, not a validated design.** No user has run it. The specific claims that need testing first: that source/target can be chosen from table counts without a question in real projects; that Q2's throwaway-container default is acceptable rather than alarming; that a per-run password with no persistence is not annoying enough to make people write the URL into `lazysnap.yml` by hand.
- **Table-count-based source selection costs two connections before the first question.** On a slow remote source that may be several seconds of silence; the timing is unmeasured.
- **`lazysnap_meta` marker-table semantics** (name, schema, contents, and what happens when the target is shared with another tool) are named here but not specified.
