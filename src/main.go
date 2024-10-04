package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Struct to store type and interface information
type TypeInfo struct {
	Name       string
	Definition *ast.Definition
}

var (
	types      = make(map[string]*TypeInfo)
	enums      = make(map[string]*ast.Definition)
	queries    = make(map[string]*ast.FieldDefinition) // Для Query
	mutations  = make(map[string]*ast.FieldDefinition) // Для Mutation
	skipChecks bool
	debug      bool
)

func main() {
	// Get command-line parameters
	inputDir := flag.String("input", "./schemas", "Directory with GraphQL schemas")
	outputPath := flag.String("output", "./generated-types.ts", "Path for the output TypeScript file")
	flag.BoolVar(&skipChecks, "skipChecks", false, "Skip type mismatch checks")
	flag.BoolVar(&debug, "debug", false, "Print debug log")
	flag.Parse()

	// Check if input directory exists
	if _, err := os.Stat(*inputDir); os.IsNotExist(err) {
		log.Fatalf("Input directory does not exist: %s", *inputDir)
	}

	// Read all .graphql files from the specified directory
	err := filepath.Walk(*inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".graphql") {
			fmt.Printf("Processing file: %s\n", path)
			if err := processSchemaFile(path); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error processing schema files: %v", err)
	}

	// Generate TypeScript file
	if err := generateTypescriptFile(*outputPath); err != nil {
		log.Fatalf("Error generating TypeScript file: %v", err)
	}

	fmt.Printf("TypeScript file generation completed. File saved at: %s\n", *outputPath)
}

func debugPrint(format string, a ...any) {
	if debug {
		fmt.Printf(format, a...)
	}
}

// Function to process a single GraphQL schema file
func processSchemaFile(path string) error {
	// Read the schema file
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file %s: %v", path, err)
	}

	debugPrint("Parsing file: %s\n", path)

	// Parse the schema
	schema, err := gqlparser.LoadSchema(&ast.Source{
		Input: string(fileContent),
	})
	if err != nil {
		return fmt.Errorf("error parsing schema in file %s: %v", path, err)
	}

	// Process types and interfaces
	for _, typ := range schema.Types {
		debugPrint("Processing type: %s from file %s\n", typ.Name, path)
		if typ.Kind == ast.Object || typ.Kind == ast.Interface {
			if typ.Name == "Query" {
				// Добавляем все поля Query
				for _, field := range typ.Fields {
					debugPrint("Adding Query field: %s\n", field.Name)
					queries[field.Name] = field
				}
			} else if typ.Name == "Mutation" {
				// Добавляем все поля Mutation
				for _, field := range typ.Fields {
					debugPrint("Adding Mutation field: %s\n", field.Name)
					mutations[field.Name] = field
				}
			} else {
				if err := addTypeOrInterface(typ); err != nil {
					return err
				}
				debugPrint("Added type/interface: %s\n", typ.Name)
			}
		}

		// Process enums
		if typ.Kind == ast.Enum {
			debugPrint("Processing enum: %s from file %s\n", typ.Name, path)
			if err := addEnum(typ); err != nil {
				return err
			}
			debugPrint("Added enum: %s\n", typ.Name)
		}
	}

	return nil
}

// Add type or interface to the global list
func addTypeOrInterface(def *ast.Definition) error {
	existing, found := types[def.Name]
	if found {
		// Compare type or interface structure if skipChecks is not enabled
		if !skipChecks && !compareDefinitions(existing.Definition, def) {
			return fmt.Errorf("error: type or interface %s has conflicting definitions", def.Name)
		}
	} else {
		// Add new type or interface
		types[def.Name] = &TypeInfo{
			Name:       def.Name,
			Definition: def,
		}
	}
	return nil
}

// Add enum to the global list
func addEnum(enum *ast.Definition) error {
	existingEnum, found := enums[enum.Name]
	if found {
		// Compare enums if skipChecks is not enabled
		if !skipChecks && !compareEnums(existingEnum, enum) {
			return fmt.Errorf("error: enum %s has conflicting definitions", enum.Name)
		}
	} else {
		enums[enum.Name] = enum
	}
	return nil
}

// Compare the structures of two type or interface definitions
func compareDefinitions(a, b *ast.Definition) bool {
	if len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i].Name != b.Fields[i].Name || a.Fields[i].Type.String() != b.Fields[i].Type.String() {
			return false
		}
	}
	return true
}

// Compare the structures of two enums
func compareEnums(a, b *ast.Definition) bool {
	if len(a.EnumValues) != len(b.EnumValues) {
		return false
	}
	for i := range a.EnumValues {
		if a.EnumValues[i].Name != b.EnumValues[i].Name {
			return false
		}
	}
	return true
}

// Generate the final TypeScript file
func generateTypescriptFile(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	// Header
	file.WriteString(`/*
 * -------------------------------------------------------
 * THIS FILE WAS AUTOMATICALLY GENERATED (DO NOT MODIFY)
 * -------------------------------------------------------
 */

/* tslint:disable */
/* eslint-disable */

`)
	file.WriteString("type Nullable<T> = T | null;\n\n")

	// Generate enums in "mirror" style
	for _, enum := range enums {
		file.WriteString(fmt.Sprintf("export enum %s {\n", enum.Name))
		for _, value := range enum.EnumValues {
			file.WriteString(fmt.Sprintf("  %s = '%s',\n", value.Name, value.Name))
		}
		file.WriteString("}\n\n")
	}

	// Generate interfaces and types
	for _, typeInfo := range types {
		if typeInfo.Definition.Kind == ast.Object {
			file.WriteString(fmt.Sprintf("export interface %s {\n", typeInfo.Name))
		} else if typeInfo.Definition.Kind == ast.Interface {
			file.WriteString(fmt.Sprintf("export interface %s {\n", typeInfo.Name))
		}

		for _, field := range typeInfo.Definition.Fields {
			isOptional := !strings.HasSuffix(field.Type.String(), "!")
			fieldType := convertGraphqlTypeToTs(field.Type.String())
			if isOptional {
				file.WriteString(fmt.Sprintf("  %s?: Nullable<%s>;\n", field.Name, fieldType))
			} else {
				file.WriteString(fmt.Sprintf("  %s: %s;\n", field.Name, fieldType))
			}
		}

		file.WriteString("}\n\n")
	}

	// Generate Query interface
	if len(queries) > 0 {
		file.WriteString("export interface Query {\n")
		for _, query := range queries {
			isOptional := !strings.HasSuffix(query.Type.String(), "!")
			fieldType := convertGraphqlTypeToTs(query.Type.String())
			if isOptional {
				file.WriteString(fmt.Sprintf("  %s?: Nullable<%s>;\n", query.Name, fieldType))
			} else {
				file.WriteString(fmt.Sprintf("  %s: %s;\n", query.Name, fieldType))
			}
		}
		file.WriteString("}\n\n")
	}

	// Generate Mutation interface
	if len(mutations) > 0 {
		file.WriteString("export interface Mutation {\n")
		for _, mutation := range mutations {
			isOptional := !strings.HasSuffix(mutation.Type.String(), "!")
			fieldType := convertGraphqlTypeToTs(mutation.Type.String())
			if isOptional {
				file.WriteString(fmt.Sprintf("  %s?: Nullable<%s>;\n", mutation.Name, fieldType))
			} else {
				file.WriteString(fmt.Sprintf("  %s: %s;\n", mutation.Name, fieldType))
			}
		}
		file.WriteString("}\n\n")
	}

	return nil
}

// Convert GraphQL types to TypeScript types
func convertGraphqlTypeToTs(graphqlType string) string {
	// Remove '!' at the end, as this represents non-nullable type in GraphQL
	cleanType := strings.TrimSuffix(graphqlType, "!")

	// Check if this is an array
	if strings.HasPrefix(cleanType, "[") && strings.HasSuffix(cleanType, "]") {
		// This is an array, extract the inner type
		innerType := cleanType[1 : len(cleanType)-1]
		// Recursively call convertGraphqlTypeToTs for the inner type
		return "Array<" + convertGraphqlTypeToTs(innerType) + ">"
	}

	// Convert standard GraphQL types to TypeScript types
	switch cleanType {
	case "String":
		return "string"
	case "Int":
		return "number"
	case "Float":
		return "number"
	case "Boolean":
		return "boolean"
	case "ID":
		return "string" // In TypeScript, IDs can be represented as strings
	case "DateTime":
		return "string"
	case "JSONObject":
		return "Record<string, unknown>"
	default:
		// Keep custom types as they are
		return cleanType
	}
}
