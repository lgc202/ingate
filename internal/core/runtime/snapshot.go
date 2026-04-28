// Package runtime defines target handoff types.
package runtime

// RuntimeSnapshot is the compiled configuration for one runtime target.
type RuntimeSnapshot struct {
	Target  string
	Gateway string
	Version string
	Config  any
}
