# GraphQL to TypeScript Generator

This CLI tool generates TypeScript interfaces from GraphQL schemas. It uses a Go-based engine for processing and compiling GraphQL schema files into TypeScript.

## Installation

```bash
npm install -g graphql-ts-generator
```

## Usage

```bash
generate-types -input ./schemas -output ./output/generated-types.ts -skipChecks -debug
```

## Options
```bash
Options:
  -input: Directory containing GraphQL schema files.
  -output: Path for the output TypeScript file.
  -skipChecks: Optional [false]. Skip type mismatch checks.
  -debug: Optional [false]. Add additional logs for interfaces
```