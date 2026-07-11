package content

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadCitations parses the YAML file at path as a top-level map of cite-key ->
// Citation. A missing file is an error. An empty (or whitespace-only) file
// yields an empty, non-nil map with no error.
func LoadCitations(path string) (map[string]Citation, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	out := make(map[string]Citation)
	if len(raw) == 0 {
		return out, nil
	}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: parsing citations: %w", path, err)
	}
	// yaml.Unmarshal of an all-whitespace/comment document leaves out nil-typed
	// but our make() above is only replaced if the document had content; guard
	// against a nil result from a document that unmarshals to null.
	if out == nil {
		out = make(map[string]Citation)
	}
	return out, nil
}
