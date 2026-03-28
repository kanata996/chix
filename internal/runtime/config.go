// 本文件职责：解析 scope 继承、单值覆盖和 mapper 叠加，生成请求执行期配置快照。
// 定位：承载 runtime 的配置语义，避免把关键继承规则埋在 observation 文件中。
package runtime

type scopeConfig struct {
	errorMappers     []ErrorMapper
	observer         Observer
	hasObserver      bool
	extractor        Extractor
	hasExtractor     bool
	successStatus    int
	hasSuccessStatus bool
}

type executionConfig struct {
	errorMappers  []ErrorMapper
	observer      Observer
	extractor     Extractor
	successStatus int
}

type resolvedFailure struct {
	raw    error
	public *HTTPError
}

func (rt *Runtime) executionConfig() executionConfig {
	return executionConfig{
		errorMappers:  append([]ErrorMapper(nil), rt.errorMappers()...),
		observer:      rt.observer(),
		extractor:     rt.extractor(),
		successStatus: rt.successStatus(),
	}
}

func (rt *Runtime) extractor() Extractor {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasExtractor {
			return current.local.extractor
		}
	}
	return DefaultExtractor
}

func (rt *Runtime) observer() Observer {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasObserver {
			return current.local.observer
		}
	}
	return nil
}

func (rt *Runtime) successStatus() int {
	// 单值配置遵循最近一层覆盖更外层。
	for current := rt; current != nil; current = current.parent {
		if current.local.hasSuccessStatus {
			return current.local.successStatus
		}
	}
	return 0
}

func (rt *Runtime) errorMappers() []ErrorMapper {
	var chain []ErrorMapper
	// mapper 链按内层到外层叠加，供 failure 路径保持 route-local 优先级。
	for current := rt; current != nil; current = current.parent {
		chain = append(chain, current.local.errorMappers...)
	}
	return chain
}
