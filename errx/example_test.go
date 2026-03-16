package errx

import (
	"errors"
	"fmt"
)

func ExampleNewMapper() {
	errAccountNotFound := errors.New("account not found")

	mapper := NewMapper(145500,
		Map(errAccountNotFound, AsNotFound(404101, "account not found")),
	)

	mapping := mapper.Map(errAccountNotFound)
	fmt.Println(mapping.StatusCode, mapping.Code, mapping.Message)
	// Output:
	// 404 404101 account not found
}
