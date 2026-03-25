# Yazi Integration

Yazi is optional. The recommended integration is a shell binding that passes the current selection directly to the CLI.

If you want `g`, `s` to prompt for a host, keep the transfer logic in `sendrecv` and let Yazi act only as the launcher.

## Interactive picker

Use this when you want `sendrecv` to prompt for the destination host at send time.

```toml
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = 'shell --block --confirm "sendrecv send \"$@\""'
desc = "Pick a host and send selection with sendrecv"
```

This uses `fzf` when available and otherwise falls back to the built-in Go picker. `sendrecv` then runs the normal `send` flow with the chosen host, so archive, extract, and path-mode behavior stay in one place.

This is the most flexible option, but it uses `shell --block` because the host picker needs terminal control. That means Yazi stays suspended until the transfer command exits.

## Fixed host keymap

Use this when you mostly send to one machine and want Yazi to return immediately while the transfer runs as a task.

```toml
[[mgr.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --remote-host laptop --extract \"$@\""'
desc = "Send selection to laptop with sendrecv"
```

Because the host is already known, this can run as a normal Yazi shell task instead of a blocking picker flow. That means the job can continue in the background and be viewed from Yazi's task manager.

## Notes

- `"$@"` preserves Yazi multi-select behavior.
- `shell --block` is important for the interactive picker because terminal control is required.
- `fzf` is optional; if it is not installed, `sendrecv` falls back to its Go picker.
- Keep the host preset in the binding if you mostly send to one machine and want the fastest path or background-task behavior.
- The CLI remains the source of truth for archive, extract, and path-mode decisions.
- Archive-mode sends from Yazi still require `sendrecv` to be installed on the remote host.
- A native Lua plugin can still be added later, but it should stay a thin shim over the CLI instead of reimplementing transfer behavior.
