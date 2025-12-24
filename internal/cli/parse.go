package cli

import "fmt"

// Command represents parsed CLI input.
type Command struct {
	Action     string
	Subcommand string
	JSON       bool
	Force      bool
	Stdio      bool
}

// Parse converts argv into a Command description.
func Parse(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, fmt.Errorf("missing command")
	}
	cmd := args[0]
	switch cmd {
	case "init":
		c := Command{Action: "init"}
		for _, a := range args[1:] {
			if a == "--force" {
				c.Force = true
			} else {
				return Command{}, fmt.Errorf("unknown flag %s", a)
			}
		}
		return c, nil
	case "index":
		if len(args) < 2 {
			return Command{}, fmt.Errorf("missing index subcommand")
		}
		sub := args[1]
		switch sub {
		case "sync":
			return Command{Action: "index", Subcommand: "sync"}, nil
		case "status":
			parsed := Command{Action: "index", Subcommand: "status"}
			if len(args) > 2 {
				for _, a := range args[2:] {
					if a == "--json" {
						parsed.JSON = true
					} else {
						return Command{}, fmt.Errorf("unknown flag %s", a)
					}
				}
			}
			return parsed, nil
		default:
			return Command{}, fmt.Errorf("unknown index subcommand %s", sub)
		}
	case "serve":
		c := Command{Action: "serve"}
		if len(args) == 2 && args[1] == "--stdio" {
			c.Stdio = true
			return c, nil
		}
		if len(args) == 1 {
			return Command{}, fmt.Errorf("missing serve mode")
		}
		return Command{}, fmt.Errorf("unknown flag %s", args[1])
	default:
		return Command{}, fmt.Errorf("unknown command %s", cmd)
	}
}
