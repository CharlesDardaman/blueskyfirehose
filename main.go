package main

import (
	"fmt"
	"os"

	"github.com/CharlesDardaman/blueskyfirehose/firehose"
	cli "github.com/urfave/cli/v2"
)

func main() {
	run(os.Args)
}

func run(args []string) {

	app := cli.App{
		Name:    "firehose",
		Usage:   "simple firehose client for bluesky",
		Version: "0.2",
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "pds-host",
			Usage:   "method, hostname, and port of PDS instance",
			Value:   "https://bsky.social",
			EnvVars: []string{"ATP_PDS_HOST"},
		},
		&cli.StringFlag{
			Name:    "auth",
			Usage:   "path to JSON file with ATP auth info",
			Value:   "bsky.auth",
			EnvVars: []string{"ATP_AUTH_FILE"},
		},
	}
	app.Commands = []*cli.Command{
		firehose.Firehose,
	}

	fmt.Println(app.Run(args))

}
