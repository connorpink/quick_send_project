# Yazi Integration

Yazi is optional. The recommended integration is the companion plugin, [`connorpink/sendrecv`](https://github.com/connorpink/sendrecv), which keeps host selection inside Yazi and launches `sendrecv` as a normal background task.

## Recommended plugin

Install the plugin:

```bash
ya pkg add connorpink/sendrecv
```

Add this keymap:

```toml
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = "plugin sendrecv"
desc = "Pick a host and send selection with sendrecv"
```

Why this is the recommended path:

- Yazi stays open after you launch the transfer.
- The transfer shows up in Yazi's task manager.
- Host selection happens inside Yazi instead of taking over the terminal.
- `sendrecv` still owns all transfer logic.

## Shell fallback: interactive picker

If you do not want to install the plugin, you can still use a shell binding that prompts through `sendrecv` itself:

```toml
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = 'shell --block --confirm "sendrecv send \"$@\""'
desc = "Pick a host and send selection with sendrecv"
```

This is flexible, but it blocks Yazi until `sendrecv` exits because the host picker needs terminal control.

## Shell fallback: fixed host

If you mostly send to one machine, a fixed host binding is still a good simple option:

```toml
[[mgr.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --remote-host laptop --extract \"$@\""'
desc = "Send selection to laptop with sendrecv"
```

Because the host is already known, this runs as a normal Yazi task and returns you to Yazi immediately.

## Notes

- The plugin depends on `sendrecv hosts --json`, so keep `sendrecv` reasonably up to date.
- `"$@"` preserves Yazi multi-select behavior for shell bindings.
- The CLI remains the source of truth for archive, extract, and path-mode decisions.
- Archive-mode sends from Yazi still require `sendrecv` to be installed on the remote host.
