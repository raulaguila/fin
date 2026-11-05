// Command admin runs operator actions against the database.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/composition"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/infra"
	"github.com/raulaguila/authenticity/pkg/logger"
)

const usage = "usage: admin promote <email> | admin disable <email>"

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	action, ok := map[string]func(*admin.Service, context.Context, string) error{
		"promote": (*admin.Service).Promote,
		"disable": (*admin.Service).Disable,
	}[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	cfg := config.MustLoad()
	log := logger.New(logger.Config{Level: "warn", Format: cfg.Log.Format})
	res, err := infra.New(cfg, log, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer res.Shutdown(log)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := action(composition.NewAdmin(cfg, res), ctx, os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		res.Shutdown(log)
		os.Exit(1)
	}
	fmt.Println(os.Args[1], "ok:", os.Args[2])
}
