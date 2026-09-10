// Command dependabot-auto-configure auto-configures .github/dependabot.yml
// for the detected repository shape.
package main

import (
	"context"
	"os"

	"github.com/larsartmann/dependabot-auto-configure/internal/cli"
)

func main() {
	os.Exit(cli.Execute(context.Background()))
}
