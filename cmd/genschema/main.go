// Command genschema generates JSON Schema from the Config struct.
//
// Usage:
//
//	go run ./cmd/genschema
//	go generate ./...
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"

	"github.com/alexmatias/librarian_fox/internal/config"
)

func main() {
	schema := jsonschema.Reflect(&config.Config{})
	schema.Title = "lfx configuration"
	schema.Description = "Configuration file for lfx — Clasifier, Restorer and Organizer of files."
	schema.Version = "http://json-schema.org/draft-04/schema#"

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	outputPath := "../../configs/lfx.schema.json"
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Schema generated: %s\n", outputPath)
}
