package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type textInputOptions struct {
	Prefix          string
	FieldName       string
	InlineFlagName  string
	InlineValue     string
	StdinFlagName   string
	UseStdin        bool
	FileFlagName    string
	FilePath        string
	PositionalValue string
	AllowPositional bool
	Required        bool
}

func readTextInput(cmd *cobra.Command, opts textInputOptions) (string, bool, error) {
	type source struct {
		name  string
		value string
	}

	var sources []source
	if strings.TrimSpace(opts.InlineValue) != "" {
		sources = append(sources, source{name: flagName(opts.InlineFlagName), value: opts.InlineValue})
	}
	if opts.UseStdin {
		sources = append(sources, source{name: flagName(opts.StdinFlagName)})
	}
	if opts.FilePath != "" {
		sources = append(sources, source{name: flagName(opts.FileFlagName)})
	}
	if opts.AllowPositional && strings.TrimSpace(opts.PositionalValue) != "" {
		sources = append(sources, source{name: "positional " + opts.FieldName, value: opts.PositionalValue})
	}

	if len(sources) > 1 {
		names := make([]string, len(sources))
		for i, src := range sources {
			names[i] = src.name
		}
		return "", false, fmt.Errorf("%s: provide only one %s source (%s)", opts.Prefix, opts.FieldName, strings.Join(names, ", "))
	}

	if len(sources) == 0 {
		if opts.Required {
			return "", false, fmt.Errorf("%s is required", opts.FieldName)
		}
		return "", false, nil
	}

	var body string
	src := sources[0]
	switch src.name {
	case flagName(opts.StdinFlagName):
		raw, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", true, fmt.Errorf("%s: read stdin: %w", opts.Prefix, err)
		}
		body = string(raw)
	case flagName(opts.FileFlagName):
		raw, err := os.ReadFile(opts.FilePath)
		if err != nil {
			return "", true, fmt.Errorf("%s: read file: %w", opts.Prefix, err)
		}
		body = string(raw)
	default:
		body = src.value
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return "", true, fmt.Errorf("%s is empty", opts.FieldName)
	}
	return body, true, nil
}

func flagName(name string) string {
	if name == "" {
		return ""
	}
	return "--" + name
}
