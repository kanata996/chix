package errx

import (
	"errors"
	"fmt"
	"net/http"
)

// Rule 表示一条 feature 本地错误到标准 errx 语义的绑定规则。
// Rule 只应通过 MapTo 构造。
type Rule struct {
	match   error
	mapping Mapping
}

// Mapper 是轻量级的 error -> Mapping 函数。
// NewMapper 返回配置完成的 Mapper；nil Mapper 视为没有 feature 级映射器。
type Mapper func(error) Mapping

func (m Mapper) Map(err error) Mapping {
	if m == nil {
		return Mapping{}
	}
	return m(err)
}

// MapTo 把 feature 本地错误绑定到一个标准 errx 语义。
// target 必须能解析为标准 errx 语义，不接受 transport 生命周期错误。
func MapTo(match error, target error) Rule {
	if match == nil {
		panic("errx: match error must not be nil")
	}

	mapping, ok := lookupSemanticMapping(target)
	if !ok {
		panic("errx: target must resolve to a standard errx semantic")
	}

	return Rule{
		match:   match,
		mapping: mapping,
	}
}

// NewMapper 构造 feature 本地 mapper。
// 它会在构造期校验 fallback 与规则配置，避免把契约错误延迟到运行时。
func NewMapper(fallbackCode int64, rules ...Rule) Mapper {
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

	return func(err error) Mapping {
		for _, rule := range clonedRules {
			if errors.Is(err, rule.match) {
				return rule.mapping
			}
		}

		if mapping, ok := Lookup(err); ok {
			return mapping
		}

		return fallback
	}
}

func validStatusCode(statusCode int) bool {
	return statusCode == 499 || (statusCode >= http.StatusBadRequest && statusCode <= 599)
}
