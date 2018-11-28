package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

// The Generator is used to generate compilable go code from a yaml configuration
type Generator struct {
	Config Config
	Debug  bool
	Logger io.Writer
}

// NewGenerator creates a new Generator instance
func NewGenerator(config Config) *Generator {
	return &Generator{
		Config: config,
		Debug:  false,
		Logger: os.Stderr,
	}
}

// Generate reads a yaml type configuration from the `input` and writes the corresponding go code to the `output`.
func (g *Generator) Generate(input io.Reader, output io.Writer) error {
	g.logVerbose("Generating code from input %q with output package %q", g.Config.InputPath, g.Config.Package)
	conf, err := g.parseInput(input)
	if err != nil {
		return fmt.Errorf("could not parse type definition: %s", err)
	}

	err = conf.Validate()
	if err != nil {
		return err
	}

	if g.Config.OutputPath != "" {
		g.generateGoGenerateLine(output)
	}

	fmt.Fprintf(output, "package %s\n\n", g.Config.PackageName())
	g.generateImports(conf, output)
	g.generateGoldiGenComment(output)
	g.generateTypeRegistrationFunction(conf, output)

	// TODO: once done check if the output is valid go code
	return nil
}

func (g *Generator) parseInput(input io.Reader) (*TypesConfiguration, error) {
	g.logVerbose("Parsing input..")
	inputData, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}

	inputData = g.sanitizeInput(inputData)

	var config TypesConfiguration
	err = yaml.Unmarshal(inputData, &config)

	captureStrings(&config)

	return &config, err
}

func (g *Generator) sanitizeInput(input []byte) []byte {
	g.logVerbose("Sanitizing input..")
	var sanitizedInput = newSanitizer()

	line := &bytes.Buffer{}
	lineBeginning := true
	for _, c := range input {
		switch c {
		case '\n':
			if strings.TrimSpace(line.String()) != "" {
				sanitizedInput.Write(append(line.Bytes(), '\n'))
				line.Reset()
				lineBeginning = true
			}
		case '\t':
			if lineBeginning {
				line.WriteString("    ")
			} else {
				line.WriteByte(c)
			}
		case ' ':
			line.WriteByte(c)
		default:
			lineBeginning = false
			line.WriteByte(c)
		}
	}

	sanitizedInput.Write(line.Bytes())

	s := sanitizedInput.Bytes()
	g.logVerbose("Sanitized input is:\n%s", string(s))
	return s
}

// captureStrings reverts any escape sequences that were introduced during the input sanitizing.
func captureStrings(config *TypesConfiguration) {
	unescape := func(input string) string {
		output := strings.Replace(input, `\@`, `@`, -1)
		return output
	}

	for id, t := range config.Types {
		t.TypeName = unescape(t.TypeName)
		t.FuncName = unescape(t.FuncName)
		t.FactoryMethod = unescape(t.FactoryMethod)
		t.AliasForType = unescape(t.AliasForType)

		for i, s := range t.Configurator {
			t.Configurator[i] = unescape(s)
		}

		for i, a := range t.RawArguments {
			s, isString := a.(string)
			if !isString {
				continue
			}
			t.RawArguments[i] = unescape(s)
		}
		for i, a := range t.RawArgumentsShort {
			s, isString := a.(string)
			if !isString {
				continue
			}
			t.RawArgumentsShort[i] = unescape(s)
		}

		config.Types[id] = t
	}
}

func (g *Generator) generateGoGenerateLine(output io.Writer) {
	fmt.Fprintf(output, "//go:generate goldigen --in %q --out %q --package %s --function %s --overwrite --nointeraction\n",
		g.Config.InputName(), g.Config.OutputName(), g.Config.Package, g.Config.FunctionName,
	)
}

func (g *Generator) generateImports(conf *TypesConfiguration, output io.Writer) {
	g.logVerbose("Generating import packages (ignoring %q)", g.Config.Package)
	packages := conf.Packages("github.com/fgrosse/goldi")

	fmt.Fprint(output, "import (\n")
	for _, pkg := range packages {
		if pkg != "" && pkg != g.Config.Package {
			g.logVerbose("Detected new import package %q", pkg)
			fmt.Fprintf(output, "\t%q\n", pkg)
		}
	}

	fmt.Fprint(output, ")\n\n")
}

func (g *Generator) generateGoldiGenComment(output io.Writer) {
	fmt.Fprintf(output, "// %s registers all types that have been defined in the file %q\n", g.Config.FunctionName, g.Config.InputName())
	fmt.Fprintf(output, "//\n")
	fmt.Fprintf(output, "// DO NOT EDIT THIS FILE: it has been generated by goldigen v%s.\n", Version)
	fmt.Fprintf(output, "// It is however good practice to put this file under version control.\n")
	fmt.Fprintf(output, "// See https://github.com/fgrosse/goldi for what is going on here.\n")
}

func (g *Generator) generateTypeRegistrationFunction(conf *TypesConfiguration, output io.Writer) {
	fmt.Fprintf(output, "func %s(types goldi.TypeRegistry) {\n", g.Config.FunctionName)
	typeIDs := make([]string, len(conf.Types))
	i := 0
	maxIDLength := 0
	for typeID := range conf.Types {
		typeIDs[i] = typeID
		i++
		if len(typeID) > maxIDLength {
			maxIDLength = len(typeID)
		}
	}
	sort.Strings(typeIDs)

	if len(conf.Types) == 1 {
		typeID := typeIDs[0]
		typeDef := conf.Types[typeID]
		fmt.Fprint(output, "\t")
		fmt.Fprintf(output, "types.Register(%q, %s)", typeID, FactoryCode(typeDef, g.Config.Package))
		fmt.Fprint(output, "\n")
	} else {
		fmt.Fprint(output, "\ttypes.RegisterAll(map[string]goldi.TypeFactory{\n")
		for _, typeID := range typeIDs {
			typeDef := conf.Types[typeID]
			spaces := strings.Repeat(" ", maxIDLength-len(typeID))
			fmt.Fprintf(output, "\t\t%q: %s%s,\n", typeID, spaces, FactoryCode(typeDef, g.Config.Package))
		}

		fmt.Fprint(output, "\t})\n")
	}

	// close the outmost surrounding function
	fmt.Fprint(output, "}\n")
}

func (g *Generator) logVerbose(message string, args ...interface{}) {
	if g.Debug {
		fmt.Fprintf(g.Logger, message+"\n", args...)
	}
}

func (g *Generator) logWarn(message string, args ...interface{}) {
	fmt.Fprintf(g.Logger, message+"\n", args...)
}
