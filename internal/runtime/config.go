// 本文件职责：解析 scope 继承、单值覆盖和 mapper 叠加，生成请求执行期配置快照。
// 定位：承载 runtime 的配置语义，避免把关键继承规则埋在 observation 文件中。
package runtime

func (rt *Runtime) executionConfig() executionConfig {
	return executionConfig{
		errorMappers:  append([]ErrorMapper(nil), rt.errorMappers()...),
		observer:      rt.observer(),
		extractor:     rt.extractor(),
		successStatus: rt.successStatus(),
	}
}

func (rt *Runtime) extractor() Extractor {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasExtractor {
			return current.local.extractor
		}
	}
	return DefaultExtractor
}

func (rt *Runtime) observer() Observer {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasObserver {
			return current.local.observer
		}
	}
	return nil
}

func (rt *Runtime) successStatus() int {
	for current := rt; current != nil; current = current.parent {
		if current.local.hasSuccessStatus {
			return current.local.successStatus
		}
	}
	return 0
}

func (rt *Runtime) errorMappers() []ErrorMapper {
	var chain []ErrorMapper
	for current := rt; current != nil; current = current.parent {
		chain = append(chain, current.local.errorMappers...)
	}
	return chain
}
