package yazi

func ExampleKeymap(host string) string {
	return `
[[mgr.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --remote-host ` + host + ` \"$@\""'
desc = "Send selection with sendrecv"
`
}

func ExamplePickerKeymap() string {
	return `
[[mgr.prepend_keymap]]
on = [ "g", "s" ]
run = 'shell --block --confirm "sendrecv send \"$@\""'
desc = "Pick a host and send selection with sendrecv"
`
}
