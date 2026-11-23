package detmath

// SetStrict switches Finite between debug (panic) and release (clamp + hook)
// behaviour for tests. It returns a function that restores the previous mode.
func SetStrict(v bool) (restore func()) {
	prev := strict
	strict = v
	return func() { strict = prev }
}
