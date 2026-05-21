package main

import (
	"context"
	"fmt"
	"os"

	resend "github.com/kylemistele/pulumi-resend/provider"
)

func main() {
	prov := resend.Provider()

	if err := prov.Run(context.Background(), resend.Name, resend.Version); err != nil {
		fmt.Fprintf(os.Stderr, "provider exited with error: %v\n", err)
		os.Exit(1)
	}
}
