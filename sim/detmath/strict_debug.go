//go:build !release

package detmath

// releaseBuild is false in debug and test builds, so Finite panics on NaN/±Inf (ADR-0021).
const releaseBuild = false
