// Package docrefs checks citations throughout docs, pins requirement and
// acceptance-example entries, and preserves actor and flow ids. It also checks
// clause ids, requirement ids and paths in the four root review docs. Its
// filesystem fixtures exercise traversal and reporting; live coverage floors
// guard against checks that evaluate nothing.
package docrefs
