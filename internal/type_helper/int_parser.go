package type_helper

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

// Constraints for supported integer types
type SignedInteger interface {
	~int | ~int32 | ~int64
}

type UnsignedInteger interface {
	~uint | ~uint32 | ~uint64
}

type AnyInteger interface {
	SignedInteger | UnsignedInteger
}

// ParseError represents a custom error for failed parsing attempts.
type ParseError[T AnyInteger] struct {
	Input string
	Err   error
}

func (e *ParseError[T]) Error() string {
	return fmt.Sprintf("error parsing %T from string %q: %v", *new(T), e.Input, e.Err)
}

func (e *ParseError[T]) Unwrap() error {
	return e.Err
}

// ParseIntegerFromString parses a string into any supported signed or unsigned integer type.
func ParseIntegerFromString[T AnyInteger](s string) (T, error) {
	var zero T
	var bitSize int

	switch any(zero).(type) {
	case int32, uint32:
		bitSize = 32
	case int64, uint64:
		bitSize = 64
	default:
		// int or uint (architecture dependent)
		bitSize = 0
	}

	switch any(zero).(type) {
	case int, int32, int64:
		val, err := strconv.ParseInt(s, 10, bitSize)
		if err != nil {
			slog.Error("Error parsing signed integer", slog.String("string", s), slog.String("target_type", fmt.Sprintf("%T", zero)))
			return zero, &ParseError[T]{Input: s, Err: err}
		}
		return T(val), nil

	case uint, uint32, uint64:
		val, err := strconv.ParseUint(s, 10, bitSize)
		if err != nil {
			slog.Error("Error parsing unsigned integer", slog.String("string", s), slog.String("target_type", fmt.Sprintf("%T", zero)))
			return zero, &ParseError[T]{Input: s, Err: err}
		}
		return T(val), nil

	default:
		return zero, errors.New("unsupported integer type")
	}
}

// IsNumber checks if the given value is an integer, floating-point number,
// or a string that can be parsed as a number.
func IsNumber(val any) bool {
	switch v := val.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64, uintptr:
		return true
	case float32, float64:
		return true
	case string:
		// Try integer parsing
		if _, err := strconv.ParseInt(v, 10, 64); err == nil {
			return true
		}
		// Try unsigned integer parsing
		if _, err := strconv.ParseUint(v, 10, 64); err == nil {
			return true
		}
		// Try floating-point parsing
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return true
		}
		return false
	default:
		return false
	}
}

func main() {
	var testInt int = 42
	var testFloat32 float32 = 3.14
	var testFloat64 float64 = 3.14
	var testString123 string = "123"
	var testStringABC string = "ABC"
	//testStructStringField := struct{ Name string }{Name: "Test"}

	fmt.Println("var testInt int = 42", IsNumber(testInt))                  // true
	fmt.Println("var testFloat32 float32 = 3.14", IsNumber(testFloat32))    // true
	fmt.Println("var testFloat64 float64 = 3.14", IsNumber(testFloat64))    // true
	fmt.Println("var testString123 quoted number", IsNumber(testString123)) // true
	fmt.Println("var testStringABC", IsNumber(testStringABC))               // false
	//fmt.Println("inline struct: struct{ Name string }{Name: 'Test'}", IsNumber(testStructStringField)) // false
}
