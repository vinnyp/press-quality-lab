//go:build !darwin

package observe

// HelpArgumentLimit supplies the minimum Darwin ARG_MAX for scripted collection
// on unsupported hosts. Released collection on these hosts never runs discovery.
func HelpArgumentLimit() (int, error) { return 256 << 10, nil }
