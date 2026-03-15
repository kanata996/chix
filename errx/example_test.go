package errx

import (
	"errors"
	"fmt"
)

func ExampleNewMapper() {
	errAccountNotFound := errors.New("account not found")

	mapper := NewMapper(145500,
		MapTo(errAccountNotFound, ErrNotFound),
	)

	mapping := mapper.Map(errAccountNotFound)
	fmt.Println(mapping.StatusCode, mapping.Code, mapping.Message)
	// Output:
	// 404 400004 Not Found
}
