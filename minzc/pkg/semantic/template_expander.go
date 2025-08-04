package semantic

import (
	"fmt"
	"os"
	"strings"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser"
)

var debugTemplate = os.Getenv("DEBUG") != ""

// TemplateExpander handles @define template expansion as a preprocessing step
type TemplateExpander struct {
	templates map[string]*Template
	sourceCode string
}

// Template represents a @define template
type Template struct {
	Name       string
	Parameters []string
	Body       string
}

// NewTemplateExpander creates a new template expander
func NewTemplateExpander() *TemplateExpander {
	return &TemplateExpander{
		templates: make(map[string]*Template),
	}
}

// ExpandTemplates is the main preprocessing step - runs BEFORE any other metaprogramming
func (e *TemplateExpander) ExpandTemplates(file *ast.File) (*ast.File, error) {
	if debugTemplate {
		fmt.Println("DEBUG: Starting template expansion")
	}
	
	// First pass: collect all template definitions
	var newDecls []ast.Declaration
	
	for _, decl := range file.Declarations {
		if template, ok := decl.(*ast.DefineTemplate); ok {
			if debugTemplate {
				fmt.Printf("DEBUG: Found DefineTemplate: Body=%q, Params=%v, Args=%v\n", 
					template.Body, template.Parameters, template.Arguments)
			}
			if template.Body != "" {
				// This is a template definition - store it
				if len(template.Parameters) > 0 {
					e.registerTemplate(template)
				}
				// Don't add template definitions to output
			} else if len(template.Arguments) > 0 {
				// This is a template invocation - expand it later
				if debugTemplate {
					fmt.Printf("DEBUG: Adding template invocation to expand later (%d args)\n", len(template.Arguments))
				}
				newDecls = append(newDecls, template)
			} else {
				if debugTemplate {
					fmt.Printf("DEBUG: Skipping empty template (no body, no args)\n")
				}
			}
		} else {
			newDecls = append(newDecls, decl)
		}
	}
	
	// Second pass: expand all template invocations
	var expandedDecls []ast.Declaration
	
	if debugTemplate {
		fmt.Printf("DEBUG: Second pass - %d declarations to process\n", len(newDecls))
	}
	
	for _, decl := range newDecls {
		if template, ok := decl.(*ast.DefineTemplate); ok && template.Body == "" {
			if debugTemplate {
				fmt.Printf("DEBUG: Expanding template invocation with %d args\n", len(template.Arguments))
			}
			// Expand template invocation
			expanded, err := e.expandTemplate(template)
			if err != nil {
				return nil, err
			}
			// Parse the expanded code and add declarations
			if expanded != "" {
				if debugTemplate {
					fmt.Printf("DEBUG: Expanded code:\n%s\n", expanded)
				}
				// Parse the expanded MinZ code
				parsedDecls, err := e.parseExpandedCode(expanded)
				if err != nil {
					return nil, fmt.Errorf("failed to parse expanded template: %w", err)
				}
				expandedDecls = append(expandedDecls, parsedDecls...)
			}
		} else {
			expandedDecls = append(expandedDecls, decl)
		}
	}
	
	// Return new file with expanded templates
	file.Declarations = expandedDecls
	return file, nil
}

// registerTemplate stores a template definition
func (e *TemplateExpander) registerTemplate(template *ast.DefineTemplate) {
	if len(template.Parameters) == 0 {
		return
	}
	
	// Create a unique key based on parameter count
	// This allows matching by arity
	key := fmt.Sprintf("template_%d_params", len(template.Parameters))
	
	if debugTemplate {
		fmt.Printf("DEBUG: Registering template with key %s\n", key)
		fmt.Printf("  Parameters: %v\n", template.Parameters)
		fmt.Printf("  Body: %s\n", template.Body)
	}
	
	e.templates[key] = &Template{
		Name:       key,
		Parameters: template.Parameters,
		Body:       template.Body,
	}
}

// expandTemplate expands a template invocation
func (e *TemplateExpander) expandTemplate(invocation *ast.DefineTemplate) (string, error) {
	if len(invocation.Arguments) == 0 {
		return "", fmt.Errorf("template invocation requires arguments")
	}
	
	// Look up template by argument count
	key := fmt.Sprintf("template_%d_params", len(invocation.Arguments))
	
	if debugTemplate {
		fmt.Printf("DEBUG: Looking for template with key %s\n", key)
		fmt.Printf("  Available templates: %v\n", e.templates)
		fmt.Printf("  Arguments: %d\n", len(invocation.Arguments))
	}
	
	template, exists := e.templates[key]
	if !exists {
		return "", fmt.Errorf("no template found with %d parameters", len(invocation.Arguments))
	}
	
	// Perform substitution
	result := template.Body
	
	// Replace {0}, {1}, {2}, etc. with arguments
	for i, arg := range invocation.Arguments {
		placeholder := fmt.Sprintf("{%d}", i)
		
		// Convert argument to string
		var value string
		switch a := arg.(type) {
		case *ast.StringLiteral:
			value = a.Value
		case *ast.NumberLiteral:
			value = fmt.Sprintf("%d", a.Value)
		case *ast.BooleanLiteral:
			if a.Value {
				value = "true"
			} else {
				value = "false"
			}
		case *ast.Identifier:
			value = a.Name
		default:
			value = fmt.Sprintf("<%T>", arg)
		}
		
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	return result, nil
}

// parseExpandedCode parses expanded template code into AST declarations
func (e *TemplateExpander) parseExpandedCode(code string) ([]ast.Declaration, error) {
	// Create a temporary MinZ source with the expanded code
	tempSource := code
	
	// Use the parser to parse the expanded code
	p := parser.New()
	decls, err := p.ParseString(tempSource, "expanded_template")
	if err != nil {
		return nil, err
	}
	
	return decls, nil
}