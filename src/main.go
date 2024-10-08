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

	inputDir := flag.String("input", "./schemas", "Directory with GraphQL schemas")
	outputPath := flag.String("output", "./generated-types.ts", "Path for the output TypeScript file")
	flag.BoolVar(&skipChecks, "skipChecks", false, "Skip type mismatch checks")
	flag.BoolVar(&debug, "debug", false, "Print debug log")
	flag.Parse()

	if _, err := os.Stat(*inputDir); os.IsNotExist(err) {
		log.Fatalf("Input directory does not exist: %s", *inputDir)
	}

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

func processSchemaFile(path string) error {

	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file %s: %v", path, err)
	}

	debugPrint("Parsing file: %s\n", path)

	schema, err := gqlparser.LoadSchema(&ast.Source{
		Input: string(fileContent),
	})
	if err != nil {
		return fmt.Errorf("error parsing schema in file %s: %v", path, err)
	}

	for _, typ := range schema.Types {
		debugPrint("Processing type: %s from file %s\n", typ.Name, path)
		if typ.Kind == ast.Object || typ.Kind == ast.Interface {
			if typ.Name == "Query" {

				for _, field := range typ.Fields {
					debugPrint("Adding Query field: %s\n", field.Name)
					queries[field.Name] = field
				}
			} else if typ.Name == "Mutation" {

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

func addTypeOrInterface(def *ast.Definition) error {
	existing, found := types[def.Name]
	if found {

		if !skipChecks && !compareDefinitions(existing.Definition, def) {
			return fmt.Errorf("error: type or interface %s has conflicting definitions", def.Name)
		}
	} else {

		types[def.Name] = &TypeInfo{
			Name:       def.Name,
			Definition: def,
		}
	}
	return nil
}

func addEnum(enum *ast.Definition) error {
	existingEnum, found := enums[enum.Name]
	if found {

		if !skipChecks && !compareEnums(existingEnum, enum) {
			return fmt.Errorf("error: enum %s has conflicting definitions", enum.Name)
		}
	} else {
		enums[enum.Name] = enum
	}
	return nil
}

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

func generateRootInterfaces(buffer *strings.Builder, fields map[string]*ast.FieldDefinition, rootName string, deferredInterfaces map[string]bool, processedTypes map[string]bool) {
	if len(fields) > 0 {
		buffer.WriteString(fmt.Sprintf("export interface %s {\n", rootName))
		for _, field := range fields {
			isOptional := !strings.HasSuffix(field.Type.String(), "!")
			fieldType := convertGraphqlTypeToTs(field.Type.String())

			if isOptional {
				buffer.WriteString(fmt.Sprintf("  %s?: Nullable<%s>;\n", field.Name, fieldType))
			} else {
				buffer.WriteString(fmt.Sprintf("  %s: %s;\n", field.Name, fieldType))
			}

			bufferRequestInterface(field.Type.String(), processedTypes, deferredInterfaces)
		}
		buffer.WriteString("}\n\n")
	}
}

func generateEnums(buffer *strings.Builder) {
	for _, enum := range enums {
		buffer.WriteString(fmt.Sprintf("export enum %s {\n", enum.Name))
		for _, value := range enum.EnumValues {
			buffer.WriteString(fmt.Sprintf("  %s = '%s',\n", value.Name, value.Name))
		}
		buffer.WriteString("}\n\n")
	}
}

func writeFileHeader(buffer *strings.Builder) {
	buffer.WriteString(`/*
 * -------------------------------------------------------
 * THIS FILE WAS AUTOMATICALLY GENERATED (DO NOT MODIFY)
 * -------------------------------------------------------
 */

/* tslint:disable */
/* eslint-disable */

`)
	buffer.WriteString("type Nullable<T> = T | null;\n\n")
}

func isObjectType(typeName string) bool {
	_, found := types[typeName]
	return found
}

func extractCleanType(typeStr string) string {
	cleanType := strings.TrimSuffix(typeStr, "!")
	if strings.HasPrefix(cleanType, "[") && strings.HasSuffix(cleanType, "]") {
		cleanType = cleanType[1 : len(cleanType)-1]
		cleanType = strings.TrimSuffix(cleanType, "!")
	}
	return cleanType
}

func bufferMissingRequestInterfaces(deferredInterfaces map[string]bool) {
	for typeName := range types {
		// Проверяем, является ли тип объектом или массивом объектов
		if isObjectOrArrayOfObjects(typeName) && !deferredInterfaces[typeName] {
			// Добавляем его для дальнейшей обработки
			deferredInterfaces[typeName] = true
			bufferRequestInterface(typeName, make(map[string]bool), deferredInterfaces)
		}
	}
}

func generateTypeInterface(buffer *strings.Builder, typeInfo *TypeInfo) {
	if typeInfo.Definition.Kind == ast.Object {
		buffer.WriteString(fmt.Sprintf("export interface %s {\n", typeInfo.Name))
	} else if typeInfo.Definition.Kind == ast.Interface {
		buffer.WriteString(fmt.Sprintf("export interface %s {\n", typeInfo.Name))
	}

	for _, field := range typeInfo.Definition.Fields {
		isOptional := !strings.HasSuffix(field.Type.String(), "!")
		fieldType := convertGraphqlTypeToTs(field.Type.String())

		if isOptional {
			buffer.WriteString(fmt.Sprintf("  %s?: Nullable<%s>;\n", field.Name, fieldType))
		} else {
			buffer.WriteString(fmt.Sprintf("  %s: %s;\n", field.Name, fieldType))
		}
	}

	buffer.WriteString("}\n\n")
}

func generateRequestInterfaces(buffer *strings.Builder, deferredInterfaces map[string]bool) {
	for deferredType := range deferredInterfaces {
		typeInfo, found := types[deferredType]
		if found {
			projectionInterfaceName := fmt.Sprintf("%sRequest", deferredType)
			buffer.WriteString(fmt.Sprintf("export interface %s {\n", projectionInterfaceName))

			for _, field := range typeInfo.Definition.Fields {
				fieldTypeStr := field.Type.String()
				isOptional := !strings.HasSuffix(fieldTypeStr, "!")
				fieldType := determineFieldType(fieldTypeStr)

				if isOptional {
					buffer.WriteString(fmt.Sprintf("  %s?: %s;\n", field.Name, fieldType))
				} else {
					buffer.WriteString(fmt.Sprintf("  %s: %s;\n", field.Name, fieldType))
				}
			}

			buffer.WriteString("}\n\n")
		}
	}
}

func determineFieldType(fieldTypeStr string) string {
	cleanType := extractCleanType(fieldTypeStr)
	fieldType := ""

	if strings.HasPrefix(cleanType, "[") && strings.HasSuffix(cleanType, "]") {
		innerType := cleanType[1 : len(cleanType)-1]
		innerType = strings.TrimSuffix(innerType, "!")
		if isObjectType(innerType) {
			fieldType = fmt.Sprintf("Array<%sRequest>", innerType)
		} else {
			fieldType = "Array<boolean | number>"
		}
	} else if isObjectType(cleanType) {
		fieldType = fmt.Sprintf("%sRequest", cleanType)
	} else {
		fieldType = "boolean | number"
	}

	return fieldType
}

func generateTypescriptFile(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	// Buffer to collect Request interfaces
	var requestInterfacesBuffer strings.Builder
	processedTypes := make(map[string]bool)
	deferredInterfaces := make(map[string]bool)

	// Header
	writeFileHeader(&requestInterfacesBuffer)

	// Generate enums in "mirror" style
	generateEnums(&requestInterfacesBuffer)

	// Generate interfaces and types
	for _, typeInfo := range types {
		generateTypeInterface(&requestInterfacesBuffer, typeInfo)

		// "Помечаем" вложенные объекты для последующей генерации Request-интерфейсов
		for _, field := range typeInfo.Definition.Fields {
			if isObjectOrArrayOfObjects(field.Type.String()) {
				bufferRequestInterface(field.Type.String(), processedTypes, deferredInterfaces)
			}
		}
	}

	// Generate Query and Mutation interfaces
	generateRootInterfaces(&requestInterfacesBuffer, queries, "Query", deferredInterfaces, processedTypes)
	generateRootInterfaces(&requestInterfacesBuffer, mutations, "Mutation", deferredInterfaces, processedTypes)

	// Добавляем недостающие Request интерфейсы для типов, которые не вошли в родительские интерфейсы
	bufferMissingRequestInterfaces(deferredInterfaces)

	// Write all deferred Request interfaces
	generateRequestInterfaces(&requestInterfacesBuffer, deferredInterfaces)

	// Write all buffered Request interfaces
	file.WriteString(requestInterfacesBuffer.String())

	return nil
}

func bufferRequestInterface(returnType string, processedTypes map[string]bool, deferredInterfaces map[string]bool) {
	cleanType := extractCleanType(returnType)

	// Если тип уже обработан, пропускаем его
	if processedTypes[cleanType] {
		return
	}
	processedTypes[cleanType] = true

	// Проверяем, существует ли такой тип среди объектов
	typeInfo, found := types[cleanType]
	if found {
		// "Помечаем" тип для дальнейшей обработки
		deferredInterfaces[cleanType] = true

		// Рекурсивно обрабатываем поля
		for _, field := range typeInfo.Definition.Fields {
			if isObjectOrArrayOfObjects(field.Type.String()) {
				bufferRequestInterface(field.Type.String(), processedTypes, deferredInterfaces)
			}
		}
	}
}

func isObjectOrArrayOfObjects(fieldType string) bool {
	cleanType := extractCleanType(fieldType)
	_, found := types[cleanType]
	return found
}

func convertGraphqlTypeToTs(graphqlType string) string {
	cleanType := strings.TrimSuffix(graphqlType, "!")

	if strings.HasPrefix(cleanType, "[") && strings.HasSuffix(cleanType, "]") {
		innerType := cleanType[1 : len(cleanType)-1]
		return "Array<" + convertGraphqlTypeToTs(innerType) + ">"
	}

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
		return "string"
	case "DateTime":
		return "string"
	case "JSONObject":
		return "Record<string, unknown>"
	default:
		return cleanType
	}
}
