package yazi

func ExampleKeymap(host string) string {
	return `
[[manager.prepend_keymap]]
on = [ "S" ]
run = 'shell --confirm "sendrecv send --extract ` + host + ` \"$@\""'
desc = "Send selection with sendrecv"
`
}
