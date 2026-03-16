package errx

import (
	"errors"
	"fmt"
	"net/http"
)

// Rule 表示一条 feature 本地错误到 HTTP-facing Mapping 的绑定规则。
// Rule 只应通过 Map 构造。
type Rule struct {
	match   error
	mapping Mapping
}

// Mapper 是 feature 级 error -> Mapping 的不透明规则集。
// 只能通过 NewMapper 构造；nil 或零值 Mapper 仅保留内建 Lookup 语义。
type Mapper struct {
	rules       []Rule
	fallback    Mapping
	hasFallback bool
}

func (m *Mapper) Map(err error) Mapping {
	if m != nil {
		for _, rule := range m.rules {
			if errors.Is(err, rule.match) {
				return rule.mapping
			}
		}
	}

	if mapping, ok := Lookup(err); ok {
		return mapping
	}

	if m != nil && m.hasFallback {
		return m.fallback
	}

	return Mapping{}
}

// Map 把 feature 本地错误绑定到一个已校验的 HTTP-facing Mapping。
func Map(match error, mapping Mapping) Rule {
	if match == nil {
		panic("errx: match error must not be nil")
	}
	if err := mapping.Validate(); err != nil {
		panic(fmt.Sprintf("errx: mapping invalid: %v", err))
	}

	return Rule{
		match:   match,
		mapping: mapping,
	}
}

// NewMapper 构造 feature 本地 mapper。
// 它会在构造期校验 fallback 与规则配置，避免把契约错误延迟到运行时。
func NewMapper(fallbackCode int64, rules ...Rule) *Mapper {
	fallback := Internal(fallbackCode)
	if err := fallback.Validate(); err != nil {
		panic(fmt.Sprintf("errx: invalid fallback mapping: %v", err))
	}

	clonedRules := append([]Rule(nil), rules...)
	for i, rule := range clonedRules {
		if rule.match == nil {
			panic(fmt.Sprintf("errx: rule %d match error must not be nil", i))
		}
		if err := rule.mapping.Validate(); err != nil {
			panic(fmt.Sprintf("errx: rule %d mapping invalid: %v", i, err))
		}
	}

	return &Mapper{
		rules:       clonedRules,
		fallback:    fallback,
		hasFallback: true,
	}
}

func validStatusCode(statusCode int) bool {
	return statusCode == 499 || (statusCode >= http.StatusBadRequest && statusCode <= 599)
}
