# pace.awk paces docs/media/first-run.tape's one real `lazyslice` run so a
# human can read it: the three lines explaining why public.customer's email,
# first_name and last_name are masked each get held on screen for a beat
# before the rest of the run continues to print. Every other line passes
# straight through. It never touches what lazyslice printed, only when each
# already-printed line reaches the terminal.
{
    print
    fflush()
    if ($0 ~ /^  public\.customer\.(email|first_name|last_name):/) {
        system("sleep 1.3")
    }
}
