# Project Rules

## Go Code Style

### Use `any` instead of `interface{}`

Per the `modernize efaceany` linting rule, always use `any` instead of `interface{}` when defining types, function parameters, or variables.

**Incorrect:**
```go
func Post(client Client, path string, body interface{}) (interface{}, error) {
	var result interface{}
	// ...
}
```

**Correct:**
```go
func Post(client Client, path string, body any) (any, error) {
	var result any
	// ...
}
```

This applies to all occurrences including:
- Function parameters and return types
- Variable declarations
- Map and slice element types
- Interface definitions
