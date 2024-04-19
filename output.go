package jsonschema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// format ---

func (e *ValidationError) schemaURL() string {
	if ref, ok := e.ErrorKind.(*kind.Reference); ok {
		return ref.URL
	} else {
		return e.SchemaURL
	}
}

func (e *ValidationError) absoluteKeywordLocation() string {
	var schemaURL string
	var keywordPath []string
	if ref, ok := e.ErrorKind.(*kind.Reference); ok {
		schemaURL = ref.URL
		keywordPath = nil
	} else {
		schemaURL = e.SchemaURL
		keywordPath = e.ErrorKind.KeywordPath()
	}
	return fmt.Sprintf("%s%s", schemaURL, encode(jsonPtr(keywordPath)))
}

func (e *ValidationError) skip() bool {
	if len(e.Causes) == 1 {
		_, ok := e.ErrorKind.(*kind.Reference)
		return ok
	}
	return false
}

func (e *ValidationError) display(sb *strings.Builder, verbose bool, indent int, absKwLoc string) {
	if !e.skip() {
		if indent > 0 {
			sb.WriteByte('\n')
			for i := 0; i < indent-1; i++ {
				sb.WriteString("  ")
			}
			sb.WriteString("- ")
		}
		indent = indent + 1

		prevAbsKwLoc := absKwLoc
		absKwLoc = e.absoluteKeywordLocation()

		if _, ok := e.ErrorKind.(kind.Schema); ok {
			sb.WriteString(e.ErrorKind.String())
		} else {
			sb.WriteString(fmt.Sprintf("at %s", quote(jsonPtr(e.InstanceLocation))))
			if verbose {
				schLoc := absKwLoc
				if prevAbsKwLoc != "" {
					pu, _ := split(prevAbsKwLoc)
					u, f := split(absKwLoc)
					if u == pu {
						schLoc = fmt.Sprintf("S#%s", f)
					}
				}
				sb.WriteString(fmt.Sprintf(" [%s]", schLoc))
			}
			sb.WriteString(fmt.Sprintf(": %s", e.ErrorKind))
		}
	}
	for _, cause := range e.Causes {
		cause.display(sb, verbose, indent, absKwLoc)
	}
}

func (e *ValidationError) Error() string {
	var sb strings.Builder
	e.display(&sb, false, 0, "")
	return sb.String()
}

func (e *ValidationError) GoString() string {
	var sb strings.Builder
	e.display(&sb, true, 0, "")
	return sb.String()
}

func jsonPtr(tokens []string) string {
	var sb strings.Builder
	for _, tok := range tokens {
		sb.WriteByte('/')
		sb.WriteString(escape(tok))
	}
	return sb.String()
}

// --

// Flag is output format with simple boolean property valid.
type FlagOutput struct {
	Valid bool `json:"valid"`
}

// The `Flag` output format, merely the boolean result.
func (e *ValidationError) FlagOutput() *FlagOutput {
	return &FlagOutput{Valid: false}
}

// --

type OutputUnit struct {
	Valid                   bool         `json:"valid"`
	KeywordLocation         string       `json:"keywordLocation"`
	AbsoluteKeywordLocation string       `json:"AbsoluteKeywordLocation,omitempty"`
	InstanceLocation        string       `json:"instanceLocation"`
	Error                   *OutputError `json:"error,omitempty"`
	Errors                  []OutputUnit `json:"errors,omitempty"`
}

type OutputError struct{ Kind ErrorKind }

func (k OutputError) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.Kind.String())
}

// The `Basic` structure, a flat list of output units.
func (e *ValidationError) BasicOutput() *OutputUnit {
	out := e.output(true, false, "", "")
	return &out
}

// The `Detailed` structure, based on the schema.
func (e *ValidationError) DetailedOutput() *OutputUnit {
	out := e.output(false, false, "", "")
	return &out
}

func (e *ValidationError) output(flatten, inRef bool, schemaURL, kwLoc string) OutputUnit {
	if !inRef {
		if _, ok := e.ErrorKind.(*kind.Reference); ok {
			inRef = true
		}
	}
	if schemaURL != "" {
		kwLoc += e.SchemaURL[len(schemaURL):]
		if ref, ok := e.ErrorKind.(*kind.Reference); ok {
			kwLoc += jsonPtr(ref.KeywordPath())
		}
	}
	schemaURL = e.schemaURL()

	keywordLocation := kwLoc
	if _, ok := e.ErrorKind.(*kind.Reference); !ok {
		keywordLocation += jsonPtr(e.ErrorKind.KeywordPath())
	}

	out := OutputUnit{
		Valid:            false,
		InstanceLocation: jsonPtr(e.InstanceLocation),
		KeywordLocation:  keywordLocation,
	}
	if inRef {
		out.AbsoluteKeywordLocation = e.absoluteKeywordLocation()
	}
	for _, cause := range e.Causes {
		causeOut := cause.output(flatten, inRef, schemaURL, kwLoc)
		if cause.skip() {
			causeOut = causeOut.Errors[0]
		}
		if flatten {
			errors := causeOut.Errors
			causeOut.Errors = nil
			causeOut.Error = &OutputError{cause.ErrorKind}
			out.Errors = append(out.Errors, causeOut)
			if len(errors) > 0 {
				out.Errors = append(out.Errors, errors...)
			}
		} else {
			out.Errors = append(out.Errors, causeOut)
		}
	}
	if len(out.Errors) == 0 {
		out.Error = &OutputError{e.ErrorKind}
	}
	return out
}
