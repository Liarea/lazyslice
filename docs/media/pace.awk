# pace.awk paces docs/media/first-run.tape's one real `lazyslice` run so a
# human can read it: the three lines explaining why public.customer's email,
# first_name and last_name are masked each get held on screen for a beat
# before the rest of the run continues to print. Every other line passes
# straight through. It never touches what lazyslice printed, only when each
# already-printed line reaches the terminal.
#
# The pattern anchors on the three column names it paces
# (public.customer.email/first_name/last_name, each followed by ":") and not
# on internal/render/render.go's two-space Info/Decision/Progress prefix:
# that prefix is shared by every line the run prints, so matching on it told
# this script nothing about which lines were the masking explanations it
# exists to slow down (T-0289).
#
# The three lines are also this script's only proof that the run reached
# the classification it exists to pace: fewer than three matches means
# lazyslice's output no longer looks like the run this tape was written for
# (a --root that no longer reaches public.customer, an event message that
# changed, a run that errored before classifying), and continuing to print
# unpaced would record a GIF nobody reviewed. So pace.awk itself now fails
# in that case — exit 1, after passing every line through — rather than
# silently succeeding at 0 or 2 matches. `make gif`'s recipe checks this
# script's own exit status (docs/media/.gif-pace-status, written by the
# tape's hidden postamble under `set -o pipefail`) and fails the target when
# it is non-zero.
BEGIN {
    matched = 0
}
{
    print
    fflush()
    if ($0 ~ /public\.customer\.(email|first_name|last_name):/) {
        matched++
        system("sleep 1.3")
    }
}
END {
    if (matched < 3) {
        print "pace.awk: paced " matched " of 3 expected lines (public.customer.email/first_name/last_name); lazyslice's output no longer matches what this script anchors on" > "/dev/stderr"
        exit 1
    }
}
