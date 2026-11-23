package main

import (
	"crypto/x509"
	"fmt"
)

func main() {
	// Reference the package without loading certificate stores or user state.
	pool := x509.NewCertPool()
	if pool == nil {
		panic("certificate pool unavailable")
	}
	fmt.Println("security ok")
}
