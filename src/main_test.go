package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper function to check if a string is present in the generated file
func fileContains(t *testing.T, filename string, content string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !strings.Contains(string(data), content) {
		t.Errorf("Expected content not found in generated file: %s", content)
	}
}

func TestMainFunction(t *testing.T) {
	inputDir := "./schemas"
	outputFile := "./output/test-generated-types.ts"

	err := os.RemoveAll(filepath.Dir(outputFile))
	if err != nil {
		t.Fatalf("Failed to clean up output directory: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(outputFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{
		"test",
		"-input", inputDir,
		"-output", outputFile,
		"-skipChecks",
		"-debug",
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer reader.Close()

	stdout := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = stdout }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, reader)
		close(done)
	}()

	main()

	writer.Close()
	<-done

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatalf("Output file not found: %v", err)
	}

	fileContains(t, outputFile, "export interface Project")
	fileContains(t, outputFile, "export interface User")
	fileContains(t, outputFile, "getProjects: Array<Project>")
	fileContains(t, outputFile, "createUser: User")

	if !strings.Contains(buf.String(), "TypeScript file generation completed") {
		t.Errorf("Expected completion message not found in output")
	}

	err = os.RemoveAll(filepath.Dir(outputFile))
	if err != nil {
		t.Fatalf("Failed to clean up output directory: %v", err)
	}
}
