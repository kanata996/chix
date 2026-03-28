// 本文件职责：解析 scope 继承、单值覆盖和 mapper 叠加，生成请求执行期配置快照。
// 定位：承载 runtime 的配置语义，避免把关键继承规则埋在 observation 文件中。
package runtime

// 保存某一层 scope 显式声明的局部配置，并用标记区分零值与未设置。
type scopeConfig struct {
	errorMappers     []ErrorMapper
	observer         Observer
	hasObserver      bool
	extractor        Extractor
	hasExtractor     bool
	successStatus    int
	hasSuccessStatus bool
}

// 表示继承与覆盖规则解析后的执行期配置快照。
type executionConfig struct {
	errorMappers  []ErrorMapper
	observer      Observer
	extractor     Extractor
	successStatus int
}

// 封装失败路径上的原始错误及其对外暴露的 HTTP 错误。
type resolvedFailure struct {
	raw    error
	public *HTTPError
}

// 解析当前 runtime 的配置继承结果，生成供一次执行使用的快照。
func (rt *Runtime) executionConfig() executionConfig {
	return executionConfig{
		errorMappers:  append([]ErrorMapper(nil), rt.errorMappers()...),
		observer:      rt.observer(),
		extractor:     rt.extractor(),
		successStatus: rt.successStatus(),
	}
}

// 按最近一层覆盖规则解析请求上下文提取器；未设置时返回默认实现。
func (rt *Runtime) extractor() Extractor {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasExtractor {
			return current.local.extractor
		}
	}
	return DefaultExtractor
}

// 按最近一层覆盖规则解析观察器；未设置时返回 nil。
func (rt *Runtime) observer() Observer {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasObserver {
			return current.local.observer
		}
	}
	return nil
}

// 按最近一层覆盖规则解析成功状态码；未设置时返回 0。
func (rt *Runtime) successStatus() int {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasSuccessStatus {
			return current.local.successStatus
		}
	}
	return 0
}

// 按内层到外层的优先级拼接错误 mapper 链。
func (rt *Runtime) errorMappers() []ErrorMapper {
	var chain []ErrorMapper
	// mapper 链按内层到外层叠加，供 failure 路径保持 route-local 优先级。
	for current := rt; current != nil; current = current.parent {
		chain = append(chain, current.local.errorMappers...)
	}
	return chain
}
