// Package importguard holds the test that keeps command frameworks, including
// Cobra and its pflag dependency, out of internal packages. The judge must stay
// a pure function.
package importguard
