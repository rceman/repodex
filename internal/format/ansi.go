package format

const (
	ansiReset        = "\x1b[0m"
	ansiBold         = "\x1b[1m"
	ansiDim          = "\x1b[2m"
	ansiUnderline    = "\x1b[4m"
	ansiUnderlineOff = "\x1b[24m"
	ansiCyan         = "\x1b[36m"
	ansiGreen        = "\x1b[32m"
)

func ansiWrap(enabled bool, prefix, s, suffix string) string {
	if !enabled || s == "" {
		return s
	}
	return prefix + s + suffix
}
