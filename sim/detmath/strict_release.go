//go:build release

package detmath

// releaseBuild is true in release builds: Finite returns 0 and calls NonFiniteHook.
const releaseBuild = true
