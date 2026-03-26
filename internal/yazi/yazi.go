package yazi

func ExamplePluginKeymap() string {
	return `
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = "plugin sendrecv"
desc = "Pick a host and send selection with sendrecv"
`
}

func ExampleFixedHostKeymap(host string) string {
	return `
[[mgr.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --remote-host ` + host + ` \"$@\""'
desc = "Send selection with sendrecv"
`
}

func ExampleShellPickerKeymap() string {
	return `
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = 'shell --block --confirm "sendrecv send \"$@\""'
desc = "Pick a host and send selection with sendrecv"
`
}
