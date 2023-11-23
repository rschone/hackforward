package corefile

type logger interface {
	Err(msg string) error
	Errf(format string, args ...interface{}) error
}
