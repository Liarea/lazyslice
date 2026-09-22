# docs/media/first-run.tape — T-0065: the VHS recording (under 25s of
# playback) of lazyslice's first run, against the real Pagila fixture in two
# disposable postgres:16 containers. `make gif` is the only intended way to
# run this: it starts
# the two containers on unusual host ports, loads
# testdata/pagila/pagila-schema.sql and pagila-data.sql into the source with
# psql, creates the read-only role the CodeRoleWritable warning recommends
# in the source and points LAZYSLICE_TAPE_SRC at it (T-0289 — the recording
# would otherwise carry that warning), exports LAZYSLICE_TAPE_SRC and
# LAZYSLICE_TAPE_TGT (both DSNs, with sslmode=disable, LAZYSLICE_TAPE_TGT
# still the superuser: the target is written to) for this tape to read,
# runs `vhs` against this file, and removes both containers on every exit
# path. Regenerate only for a v0.x minor (tracker log, T-0065); this file
# and docs/media/pace.awk are reviewed and committed like any other source
# file.
#
# Nothing typed below is a real value or a password, other than the
# throwaway container password `make gif` itself generates.
#
# vhs 0.11.0. See https://github.com/charmbracelet/vhs for the tape format.

Output docs/media/first-run.gif

Set Shell "bash"
Set Width 1200
Set Height 700
Set FontSize 15
Set Padding 16
Set TypingSpeed 8ms
Set WaitTimeout 15s
Set Framerate 5

# The secret is real (openssl rand), but nobody needs to read it, so the
# whole line — command and the export it leaves in the shell — is hidden.
# LAZYSLICE_TAPE_SRC and LAZYSLICE_TAPE_TGT come from `make gif`'s
# environment, already pointed at the two running containers (the source
# DSN as the read-only role, the target DSN as the superuser).
# The hidden preamble also defines `lazyslice` as a function around the
# freshly built binary: the visible command is then exactly what a person
# types after `brew install`, and the function writes the binary's own exit
# status to docs/media/.gif-status on the way out, where `make gif` reads it.
# Capturing the status this way keeps it off the screen: a Hide/Show pair
# around a separate `echo` stops recording frames but leaves the typed line
# on the terminal, so it reappeared in every frame after Show (T-0065's
# first recording).
# The preamble also shadows `awk` with a function that calls the real
# binary (`command awk`) and then writes its own exit status to
# docs/media/.gif-pace-status: the same off-screen trick as `lazyslice`
# above, chosen so the visible pipeline below stays exactly
# `| awk -f docs/media/pace.awk` — the real command a person would type —
# with no extra visible plumbing to capture pace.awk's status once it can
# fail (T-0289). `set -o pipefail` is not needed for this: the shadow
# function's own `$?` is awk's, independent of the pipe it sits in.
Hide
Type "clear"
Enter
Type "export LAZYSLICE_SECRET=$(openssl rand -hex 32)"
Enter
Type 'lazyslice() { bin/lazyslice "$@" 2>&1; echo $? > docs/media/.gif-status; }'
Enter
Type 'awk() { command awk "$@"; echo $? > docs/media/.gif-pace-status; }'
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 200ms
Show

# One run: source and target named by flag (sslmode=disable, in both DSNs),
# --root and -n answering the two questions an interactive run would
# otherwise ask, --no-config so this demo leaves no lazyslice.yml behind.
# The output is the real binary's, piped through docs/media/pace.awk only so
# a human watching the GIF can read it — the three lines explaining why
# public.customer's email, first_name and last_name are masked get held
# longer than the rest.
#
# The pipeline's own exit status is still awk's (now the shadow function's),
# always 0, so the `lazyslice` function above writes the binary's real
# status to docs/media/.gif-status and the `awk` function writes pace.awk's
# real status to docs/media/.gif-pace-status; `make gif` fails the recipe
# unless both read back 0. Wait for the prompt to return (vhs normalizes it
# to "> ").
Type 'lazyslice --source "$LAZYSLICE_TAPE_SRC" --target "$LAZYSLICE_TAPE_TGT" --root public.customer -n 200 --no-config | awk -f docs/media/pace.awk'
Enter
Wait /^>\s*$/
Sleep 1s

# Proof, not a promise: three customer rows read back from the target with
# a plain psql select. Real names and a real-shaped email, neither copied
# from the source.
Type 'psql "$LAZYSLICE_TAPE_TGT" -c "SELECT customer_id, first_name, last_name, email FROM public.customer ORDER BY customer_id LIMIT 3;"'
Enter
Wait /^>\s*$/
Sleep 1s
