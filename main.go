package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/go-pa/fenv"
	"github.com/google/subcommands"
	"github.com/thomasf/docker-remote-logs/internal/agent"
)

func main() {

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&agent.AgentCmd{}, "")
	fenv.Prefix("DRLOG_")
	fenv.MustParse()
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()

	os.Exit(int(subcommands.Execute(ctx)))

}
