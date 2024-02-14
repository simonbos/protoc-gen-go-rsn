package main

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

const (
	extension     = ".rsn.go"
	fmtPackage    = protogen.GoImportPath("fmt")
	regexpPackage = protogen.GoImportPath("regexp")
	errorsPackage = protogen.GoImportPath("errors")

	typeStructField   = "Type"
	parentStructField = "Parent"
)

type Resource struct {
	ServiceName string
	Type        string
	Patterns    []Pattern
}

func (r Resource) parentTypeTypeAlias() string {
	return r.Type + "ParentType"
}

func (r Resource) structName() string {
	return r.Type + "Rsn"
}

func (r Resource) parentStructName() string {
	return r.Type + "ParentRsn"
}

func (r Resource) parentStructFields() []string {
	result := make(map[string]struct{})
	for _, pattern := range r.Patterns {
		for _, field := range pattern.parentStructFields("") {
			result[field] = struct{}{}
		}
	}
	fields := make([]string, 0, len(result))
	for k := range result {
		fields = append(fields, k)
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i] < fields[j]
	})
	return fields
}

// lastStructFields should return a length of 1 for well-defined resource
// patterns (which have the same final collection identifier).
func (r Resource) lastStructFields() []string {
	result := make(map[string]struct{})
	for _, pattern := range r.Patterns {
		result[pattern.lastStructField("")] = struct{}{}
	}
	fields := make([]string, 0, len(result))
	for k := range result {
		fields = append(fields, k)
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i] < fields[j]
	})
	return fields
}

type Pattern struct {
	collectionIds []string
	resourceVars  []string
}

func (p Pattern) parentStructFields(prefix string) []string {
	result := make([]string, len(p.resourceVars)-1)
	for i := 0; i < len(p.resourceVars)-1; i++ {
		result[i] = prefix + strcase.ToCamel(p.resourceVars[i]) + "Id"
	}
	return result
}

func (p Pattern) lastStructField(prefix string) string {
	return prefix + strcase.ToCamel(p.resourceVars[len(p.resourceVars)-1]) + "Id"
}

func (p Pattern) parentTypeConst() string {
	if len(p.resourceVars) == 1 {
		// top-level
		return strcase.ToCamel(p.resourceVars[0]) + "RootParentType"
	}
	result := ""
	for _, resourceVar := range p.resourceVars {
		result += strcase.ToCamel(resourceVar)
	}
	return result + "ParentType"
}

func (p Pattern) parentTypeString() string {
	if len(p.resourceVars) == 1 {
		// top-level
		return ""
	}
	result := strcase.ToLowerCamel(p.resourceVars[0])
	for i := 1; i < len(p.resourceVars)-1; i++ {
		result += strcase.ToCamel(p.resourceVars[i])
	}
	return result
}

func (p Pattern) regexName() string {
	result := ""
	for i, resourceVar := range p.resourceVars {
		if i == 0 {
			result += strcase.ToLowerCamel(resourceVar)
		} else {
			result += strcase.ToCamel(resourceVar)
		}
	}
	return result + "RsnPattern"
}

func (p Pattern) parentRegexName(t string) string {
	result := strcase.ToLowerCamel(p.resourceVars[0])
	for i := 1; i < len(p.resourceVars)-1; i++ {
		result += strcase.ToCamel(p.resourceVars[i])
	}
	return result + t + "ParentPattern"
}

func (p Pattern) regexString() string {
	result := "^"
	for i, collectionId := range p.collectionIds {
		if i != 0 {
			result += "/"
		}
		result += collectionId + "/([^/]+)"
	}
	return result + "$"
}

func (p Pattern) parentRegexString() string {
	result := "^"
	for i := 0; i < len(p.collectionIds)-1; i++ {
		if i != 0 {
			result += "/"
		}
		result += p.collectionIds[i] + "/([^/]+)"
	}
	return result + "$"
}

func (p Pattern) formatString() string {
	result := ""
	for i, collectionId := range p.collectionIds {
		if i != 0 {
			result += "/"
		}
		result += collectionId + "/%s"
	}
	return result
}

func (p Pattern) parentFormatString() string {
	result := ""
	for i := 0; i < len(p.collectionIds)-1; i++ {
		if i != 0 {
			result += "/"
		}
		result += p.collectionIds[i] + "/%s"
	}
	return result
}

func NewPattern(patternString string) (Pattern, error) {
	splittedPattern := strings.Split(patternString, "/")
	if len(splittedPattern)%2 != 0 {
		return Pattern{}, errors.New("the pattern does not have an equal number of collection ids and resource variables")
	}
	var p Pattern
	for i := 0; i < len(splittedPattern); i += 2 {
		p.collectionIds = append(p.collectionIds, splittedPattern[i])
		p.resourceVars = append(p.resourceVars, strings.Trim(splittedPattern[i+1], "{}"))
	}
	return p, nil
}

func main() {
	options := protogen.Options{}

	options.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			version := "(unknown)"
			bi, ok := debug.ReadBuildInfo()
			if ok {
				version = bi.Main.Version
			}
			generateFile(gen, f, version)
		}
		return nil
	})
}

func parseResourcesFromDefinitions(file *protogen.File) []Resource {
	resources := make([]Resource, 0)
	options := file.Desc.Options()
	if options != nil {
		resourceDescriptors := proto.GetExtension(options, annotations.E_ResourceDefinition).([]*annotations.ResourceDescriptor)
		for _, resourceDescriptor := range resourceDescriptors {
			resource, err := parseResourceFromDescriptor(resourceDescriptor)
			if err != nil {
				// TODO: warning
				continue
			}
			resources = append(resources, resource)
		}
	}
	return resources
}

func parseResourcesFromMessages(file *protogen.File) []Resource {
	resources := make([]Resource, 0)
	for _, message := range file.Messages {
		options := message.Desc.Options()
		if options == nil {
			continue
		}
		if proto.HasExtension(options, annotations.E_Resource) {
			resourceDescriptor := proto.GetExtension(options, annotations.E_Resource).(*annotations.ResourceDescriptor)
			resource, err := parseResourceFromDescriptor(resourceDescriptor)
			if err != nil {
				// TODO: warning
				continue
			}
			resources = append(resources, resource)
		}
	}
	return resources
}

func parseResourceFromDescriptor(resourceDescriptor *annotations.ResourceDescriptor) (Resource, error) {
	if len(resourceDescriptor.Type) == 0 {
		return Resource{}, fmt.Errorf("resource descriptor should have a type")
	}
	if len(resourceDescriptor.Pattern) == 0 {
		return Resource{}, fmt.Errorf("resource descriptor should have at least 1 pattern")
	}

	resourceTypeSplitted := strings.Split(resourceDescriptor.Type, "/")
	if len(resourceTypeSplitted) != 2 {
		return Resource{}, fmt.Errorf("invalid resource type %s", resourceDescriptor.Type)
	}
	resource := Resource{
		ServiceName: resourceTypeSplitted[0],
		Type:        resourceTypeSplitted[1],
	}
	for _, patternString := range resourceDescriptor.Pattern {
		pattern, err := NewPattern(patternString)
		if err != nil {
			return Resource{}, fmt.Errorf("invalid pattern %s: %v", patternString, err)
		}
		resource.Patterns = append(resource.Patterns, pattern)
	}
	return resource, nil
}

func protocVersion(gen *protogen.Plugin) string {
	v := gen.Request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}
	var suffix string
	if s := v.GetSuffix(); s != "" {
		suffix = "-" + s
	}
	return fmt.Sprintf("v%d.%d.%d%s", v.GetMajor(), v.GetMinor(), v.GetPatch(), suffix)
}

func generateFile(plugin *protogen.Plugin, file *protogen.File, version string) {
	definitionResources := parseResourcesFromDefinitions(file)
	messageResources := parseResourcesFromMessages(file)

	if len(definitionResources) == 0 && len(messageResources) == 0 {
		return
	}

	g := plugin.NewGeneratedFile(file.GeneratedFilenamePrefix+extension, file.GoImportPath)

	// Package statement
	g.P("// Code generated by protoc-gen-go-rsn. DO NOT EDIT.")
	g.P("// versions:")
	g.P("// - protoc-gen-go-rsn ", version)
	g.P("// - protoc            ", protocVersion(plugin))
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()

	// Generate code for each definition resource
	for _, resource := range definitionResources {
		generateResource(g, resource)
	}

	// Generate code for each message resource
	for _, resource := range messageResources {
		generateResource(g, resource)
	}
}

func generateResource(g *protogen.GeneratedFile, resource Resource) {
	g.P("// ", resource.parentTypeTypeAlias(), " indicates the possible parent types for resource '", resource.Type, "'.")
	g.P("type ", resource.parentTypeTypeAlias(), " = string")
	g.P()

	g.P("const (")
	for _, pattern := range resource.Patterns {
		g.P("// ", pattern.parentTypeConst(), " is a possible parent type for resource '", resource.Type, "', used for '", pattern.parentTypeString(), "'.")
		g.P(pattern.parentTypeConst(), " ", resource.parentTypeTypeAlias(), " = \"", pattern.parentTypeString(), "\"")
	}
	g.P(")")
	g.P()

	g.P("// ", resource.parentStructName(), " is the representation of a '", resource.Type, "' parent resource name.")
	g.P("// This contains all possible identifiers for all parent types. The Type field implicitly declares the")
	g.P("// identifiers used for that context.")
	g.P("type ", resource.parentStructName(), " struct{")
	for _, structField := range resource.parentStructFields() {
		g.P(structField, " string")
	}
	g.P(typeStructField, " ", resource.parentTypeTypeAlias())
	g.P("}")
	g.P()

	g.P("// ResourceName formats r to a resource name.")
	g.P("func (r ", resource.parentStructName(), ") ResourceName() string {")
	g.P("switch r.", typeStructField, " {")
	for _, pattern := range resource.Patterns {
		g.P("case ", pattern.parentTypeConst(), ":")
		g.P("return ", g.QualifiedGoIdent(fmtPackage.Ident("Sprintf")), "(\"", pattern.parentFormatString(), "\", ", strings.Join(pattern.parentStructFields("r."), ", "), ")")
	}
	g.P("}")
	g.P("return \"\"")
	g.P("}")
	g.P()

	g.P("// IsZero reports whether r represents the zero resource name.")
	g.P("func (r ", resource.parentStructName(), ") IsZero() bool {")
	parentStructFields := resource.parentStructFields()
	parentZeroChecks := make([]string, len(parentStructFields)+1)
	for i, structField := range parentStructFields {
		parentZeroChecks[i] = fmt.Sprintf("len(r.%s) == 0", structField)
	}
	parentZeroChecks[len(parentStructFields)] = fmt.Sprintf("len(r.%s) == 0", typeStructField)
	g.P("return ", strings.Join(parentZeroChecks, " && "))
	g.P("}")
	g.P()

	g.P("var (")
	for _, pattern := range resource.Patterns {
		g.P(pattern.parentRegexName(resource.Type), " = ", g.QualifiedGoIdent(regexpPackage.Ident("MustCompile")), "(`", pattern.parentRegexString(), "`)")
	}
	g.P(")")
	g.P()

	g.P("// Parse", resource.Type, "ParentResourceName parses the given parent resource name to a parent resource name.")
	g.P("// Errors for unknown patterns.")
	g.P("func Parse", resource.Type, "ParentResourceName(resourceName string) (", resource.parentStructName(), ", error) {")
	for _, pattern := range resource.Patterns {
		g.P("if match := ", pattern.parentRegexName(resource.Type), ".FindStringSubmatch(resourceName); match != nil {")
		g.P("return ", resource.parentStructName(), "{")
		for i, structField := range pattern.parentStructFields("") {
			g.P(structField, ": match[", i+1, "],")
		}
		g.P(typeStructField, ": ", pattern.parentTypeConst(), ",")
		g.P("}, nil")
		g.P("}")
	}
	g.P("return ", resource.parentStructName(), "{}, ", g.QualifiedGoIdent(errorsPackage.Ident("New")), "(\"invalid parent resource name for resource type ", "'", resource.Type, "' in service '", resource.ServiceName, "'\")")
	g.P("}")
	g.P()

	g.P("// ", resource.structName(), " is the representation of a '", resource.Type, "' resource name.")
	g.P("type ", resource.structName(), " struct{")
	g.P(parentStructField, " ", resource.parentStructName())
	for _, structField := range resource.lastStructFields() {
		g.P(structField, " string")
	}
	g.P("}")
	g.P()

	g.P("// ResourceName formats r to a resource name.")
	g.P("func (r ", resource.structName(), ") ResourceName() string {")
	g.P("switch r.", parentStructField, ".", typeStructField, " {")
	for _, pattern := range resource.Patterns {
		g.P("case ", pattern.parentTypeConst(), ":")
		g.P("return ", g.QualifiedGoIdent(fmtPackage.Ident("Sprintf")), "(\"", pattern.formatString(), "\", ", strings.Join(append(pattern.parentStructFields("r."+parentStructField+"."), pattern.lastStructField("r.")), ", "), ")")
	}
	g.P("}")
	g.P("return \"\"")
	g.P("}")
	g.P()

	g.P("// IsZero reports whether r represents the zero resource name.")
	g.P("func (r ", resource.structName(), ") IsZero() bool {")
	lastStructFields := resource.lastStructFields()
	resourceZeroChecks := make([]string, len(lastStructFields)+1)
	resourceZeroChecks[0] = fmt.Sprintf("r.%s.IsZero()", parentStructField)
	for i, structField := range lastStructFields {
		resourceZeroChecks[i+1] = fmt.Sprintf("len(r.%s) == 0", structField)
	}
	g.P("return ", strings.Join(resourceZeroChecks, " && "))
	g.P("}")
	g.P()

	g.P("var (")
	for _, pattern := range resource.Patterns {
		g.P(pattern.regexName(), " = ", g.QualifiedGoIdent(regexpPackage.Ident("MustCompile")), "(`", pattern.regexString(), "`)")
	}
	g.P(")")
	g.P()

	g.P("// Parse", resource.Type, "ResourceName parses the given resource name to a resource name.")
	g.P("// Errors for unknown patterns.")
	g.P("func Parse", resource.Type, "ResourceName(resourceName string) (", resource.structName(), ", error) {")
	for _, pattern := range resource.Patterns {
		g.P("if match := ", pattern.regexName(), ".FindStringSubmatch(resourceName); match != nil {")
		g.P("return ", resource.structName(), "{")
		g.P(parentStructField, ": ", resource.parentStructName(), "{")
		for i, structField := range pattern.parentStructFields("") {
			g.P(structField, ": match[", i+1, "],")
		}
		g.P(typeStructField, ": ", pattern.parentTypeConst(), ",")
		g.P("},")
		g.P(pattern.lastStructField(""), ": match[", len(pattern.parentStructFields(""))+1, "],")
		g.P("}, nil")
		g.P("}")
	}
	g.P("return ", resource.structName(), "{}, ", g.QualifiedGoIdent(errorsPackage.Ident("New")), "(\"invalid resource name for resource type ", "'", resource.Type, "' in service '", resource.ServiceName, "'\")")
	g.P("}")
	g.P()
}
