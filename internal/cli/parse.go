package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// Command represents parsed CLI input.
type Command struct {
	Action     string
	Subcommand string
	JSON       bool
	Force      bool
	Stdio      bool
	Q          string
	TopK       int
	IDs        []uint32
	MaxLines   int
	Score      bool
	NoFormat   bool
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
	case "status":
		c := Command{Action: "status"}
		for _, a := range args[1:] {
			if a == "--json" {
				c.JSON = true
			} else {
				return Command{}, fmt.Errorf("unknown flag %s", a)
			}
		}
		return c, nil
	case "sync":
		if len(args) > 1 {
			return Command{}, fmt.Errorf("unknown flag %s", args[1])
		}
		return Command{Action: "sync"}, nil
	case "scan":
		if len(args) > 1 {
			return Command{}, fmt.Errorf("unknown flag %s", args[1])
		}
		return Command{Action: "sync"}, nil
	case "search":
		c := Command{Action: "search"}
		i := 1
		for i < len(args) {
			switch args[i] {
			case "--q":
				if i+1 >= len(args) {
					return Command{}, fmt.Errorf("missing value for --q")
				}
				c.Q = args[i+1]
				i += 2
			case "--top_k":
				if i+1 >= len(args) {
					return Command{}, fmt.Errorf("missing value for --top_k")
				}
				val, err := strconv.Atoi(args[i+1])
				if err != nil {
					return Command{}, fmt.Errorf("invalid top_k %s", args[i+1])
				}
				if val < 0 {
					return Command{}, fmt.Errorf("top_k must be non-negative")
				}
				c.TopK = val
				i += 2
			case "--json":
				c.JSON = true
				i++
			case "--no-format":
				c.NoFormat = true
				i++
			case "--score":
				c.Score = true
				i++
			default:
				return Command{}, fmt.Errorf("unknown flag %s", args[i])
			}
		}
		if c.Q == "" {
			return Command{}, fmt.Errorf("missing required --q")
		}
		return c, nil
	case "fetch":
		c := Command{Action: "fetch"}
		i := 1
		for i < len(args) {
			switch args[i] {
			case "--ids":
				if i+1 >= len(args) {
					return Command{}, fmt.Errorf("missing value for --ids")
				}
				raw := strings.Split(args[i+1], ",")
				for _, r := range raw {
					r = strings.TrimSpace(r)
					if r == "" {
						continue
					}
					id, err := strconv.ParseUint(r, 10, 32)
					if err != nil {
						return Command{}, fmt.Errorf("invalid id %s", r)
					}
					c.IDs = append(c.IDs, uint32(id))
				}
				i += 2
			case "--max_lines":
				if i+1 >= len(args) {
					return Command{}, fmt.Errorf("missing value for --max_lines")
				}
				val, err := strconv.Atoi(args[i+1])
				if err != nil {
					return Command{}, fmt.Errorf("invalid max_lines %s", args[i+1])
				}
				if val < 0 {
					return Command{}, fmt.Errorf("max_lines must be non-negative")
				}
				c.MaxLines = val
				i += 2
			default:
				return Command{}, fmt.Errorf("unknown flag %s", args[i])
			}
		}
		if len(c.IDs) == 0 {
			return Command{}, fmt.Errorf("missing required --ids")
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
