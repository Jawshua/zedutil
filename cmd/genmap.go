/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/Jawshua/zedutil/pkg/parser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// genCmd represents the gen command
var genCmd = &cobra.Command{
	Use:   "genmap <schema.zed>",
	Short: "Generate a relation map file from a .zed schema file",
	Long: `Parses a .zed schema file and generates a map of the relations it contains.

Definitions may have bool and string annotations attached to them by adding an @attr line to the comment block.

  /** @attr boolAttr1 strAttr1=value
   * user represents a user that can be granted role(s)
   */
  definition user {}

The parsed result would appear as:

  entities:
    document_embedding_instance:
      relations: []
    metadata:
      comment: user represents a user that can be granted role(s)
      attributes:
        boolAttr1: true
        strAttr1: "value"
`,
	RunE: genmapAction,
	Args: cobra.MinimumNArgs(1),
}

func genmapAction(cmd *cobra.Command, args []string) error {
	var (
		format         = cmd.Flag("format").Value.String()
		outputFilename = cmd.Flag("output").Value.String()
		outputWriter   io.Writer
	)

	// Crude format validation, and defaulting from the file ext.
	if outputFilename != "-" && format == "" {
		format = strings.TrimPrefix(path.Ext(outputFilename), ".")

		if format == "yml" {
			format = "yaml"
		}
	}

	if format == "" {
		format = "json"
	}

	if format != "json" && format != "yaml" {
		return fmt.Errorf("unknown format: %s", format)
	}

	parsedSchema, err := parser.Parse(args[0])
	if err != nil {
		return err
	}

	// Open the output file
	if outputFilename == "-" {
		outputWriter = os.Stdout
	} else {
		outputFile, err := os.Create(outputFilename)
		if err != nil {
			return err
		}
		defer outputFile.Close()
		outputWriter = outputFile
	}

	var encoderError error
	switch format {
	case "json":
		encoder := json.NewEncoder(outputWriter)
		encoder.SetIndent("", "    ")
		encoderError = encoder.Encode(parsedSchema)
	case "yaml":
		encoder := yaml.NewEncoder(outputWriter)
		encoder.SetIndent(2)
		encoderError = encoder.Encode(parsedSchema)
	}

	if encoderError != nil {
		return fmt.Errorf("error encoding output with %s: %w", format, encoderError)
	}

	if cmd.Flag("quiet").Value.String() == "false" {
		if len(parsedSchema.Warnings) > 0 {
			cmd.PrintErrf("zedutil parser generated %d warnings while processing the schema:\n", len(parsedSchema.Warnings))
			for _, warning := range parsedSchema.Warnings {
				cmd.PrintErrf("* %s\n", warning)
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(genCmd)

	genCmd.PersistentFlags().StringP("format", "f", "", "set the output format [json or yaml]")
	genCmd.PersistentFlags().StringP("output", "o", "-", "set the output filename")
	genCmd.PersistentFlags().BoolP("quiet", "q", false, "don't output parser warnings to stderr")
}
