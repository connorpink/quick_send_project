# Yazi Integration

Yazi is optional. The recommended integration is a shell binding that passes the current selection directly to the CLI.

## Example keymap

```toml
[[manager.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --extract laptop \"$@\""'
desc = "Send selection with sendrecv"
```

## Notes

- `"$@"` preserves Yazi multi-select behavior.
- Keep the host preset in the binding for the fastest workflow.
- The CLI remains the source of truth for archive, extract, and path-mode decisions.
- Archive-mode sends from Yazi still require `sendrecv` to be installed on the remote host.
- A native Lua plugin can be added later without changing the transfer engine.
