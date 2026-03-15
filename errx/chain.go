package errx

import "strings"

const maxChainDepth = 32

func FormatChain(err error) string {
	if err == nil {
		return ""
	}

	parts := make([]string, 0, 8)
	collectChain(err, &parts, 0)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ==> ")
}

func collectChain(err error, parts *[]string, depth int) {
	if err == nil || depth >= maxChainDepth {
		return
	}

	msg := strings.TrimSpace(err.Error())
	if msg != "" {
		*parts = append(*parts, msg)
	}

	switch x := err.(type) {
	case interface{ Unwrap() []error }:
		for _, cause := range x.Unwrap() {
			collectChain(cause, parts, depth+1)
		}
	case interface{ Unwrap() error }:
		collectChain(x.Unwrap(), parts, depth+1)
	}
}
