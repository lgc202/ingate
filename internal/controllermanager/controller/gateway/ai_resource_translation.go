package gateway

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/types/known/emptypb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// aiResourceTranslation 由单个 Reconcile Worker 持有最近有效的只读转换结果。
// 保留当前使用的源与消息，不缓存历史代次；候选构建完成后才整体登记。
type aiResourceTranslation struct {
	resources    []*aidataplanev1.Resource
	sources      map[string]aiResourceSource
	modelTargets map[string]aiModelTargetTranslation
	mcpTargets   map[string]aiMCPTargetTranslation
}

// 目标缓存借用 krt 保留的只读网络实例，协议字段按值保存，不持有配置中的可变指针。
// 它们不是事实来源；重启或清空后仍从同一次 publication 的完整期望重建。
type aiModelTargetTranslation struct {
	schema            aidataplane.APISchema
	modelNameOverride string
	network           *networkBackend
	resource          *aidataplanev1.ModelTarget
}

type aiMCPTargetTranslation struct {
	network *networkBackend
	name    string
}

// aiResourceSource 只记录生成一个命名资源所需的输入；领域结果来自只读计算图。
// refs 由本次候选独立拥有，不能向上次消息追加入口引用。
type aiResourceSource struct {
	entry       *listenerEntry
	model       *modelRoute
	ruleIndex   int
	modelTarget *aidataplanev1.ModelTarget
	mcp         *mcpRoute
	mcpBackend  string
	mcpTarget   string
	refs        []aiEntryReference
	resource    *aidataplanev1.Resource
}

type aiEntryReference struct {
	port int32
	name string
}

// compile 扫描已收敛的候选引用，只转换变化的资源；失败不修改上次结果。
// config 必须由同一 snapshot 的 aiConfig 生成；图对象只用作只读依赖身份，
// 新目标仍使用与 Envoy 相同的内容计算。缓存丢失不影响期望重建。
func (t *aiResourceTranslation) compile(
	snapshot publication,
	config aidataplane.Config,
) (aiResourceTranslation, error) {
	c, err := newAIResourceCompilation(snapshot, config, t)
	if err != nil {
		return aiResourceTranslation{}, err
	}

	for _, table := range snapshot.tables {
		if table.ai.Name == "" {
			continue
		}

		if err := c.addEntry(table); err != nil {
			return aiResourceTranslation{}, err
		}
	}

	next := aiResourceTranslation{
		sources:      c.sources,
		modelTargets: c.modelTargets,
		mcpTargets:   c.mcpTargets,
	}
	for _, name := range slices.Sorted(maps.Keys(c.sources)) {
		source := c.sources[name]
		slices.SortFunc(source.refs, compareAIEntryReferences)
		previous := t.sources[name]
		if source.same(previous) {
			source.resource = previous.resource
		} else {
			source.resource = source.compile(name)
			if err := protovalidate.Validate(source.resource); err != nil {
				return aiResourceTranslation{}, fmt.Errorf("validate AI Proxy data-plane resource %q: %w", name, err)
			}
		}

		next.sources[name] = source
		next.resources = append(next.resources, source.resource)
	}

	return next, nil
}

// same 只比较实际进入消息的内容；HTTP 变化、父引用顺序和兄弟规则不使本资源失效。
// 入口引用已排序；模型目标用内容寻址名称比较，避免对不可变 protobuf 反复深拷贝。
func (s aiResourceSource) same(previous aiResourceSource) bool {
	if previous.resource == nil || !slices.Equal(s.refs, previous.refs) {
		return false
	}

	if s.entry != nil {
		return previous.entry != nil && s.entry.listener.Port == previous.entry.listener.Port &&
			s.entry.ResourceName() == previous.entry.ResourceName() && s.entry.listener.Hostname == previous.entry.listener.Hostname
	}

	if s.model != nil {
		if previous.model == nil || s.ruleIndex != previous.ruleIndex ||
			compareMeta(s.model.ObjectMeta, previous.model.ObjectMeta) != 0 ||
			s.modelTarget.GetTargetName() != previous.modelTarget.GetTargetName() {
			return false
		}

		return slices.EqualFunc(s.model.rules[s.ruleIndex].matches, previous.model.rules[previous.ruleIndex].matches,
			func(a, b aidataplane.ModelRoute) bool { return slices.Equal(a.Headers, b.Headers) })
	}

	if previous.mcp == nil || compareMeta(s.mcp.ObjectMeta, previous.mcp.ObjectMeta) != 0 ||
		s.mcp.match.Path != previous.mcp.match.Path || s.mcpBackend != previous.mcpBackend || s.mcpTarget != previous.mcpTarget {
		return false
	}

	return s.mcpBackend == "" || s.mcp.match.BackendPath == previous.mcp.match.BackendPath
}

func (s aiResourceSource) compile(name string) *aidataplanev1.Resource {
	if s.entry != nil {
		return &aidataplanev1.Resource{
			Name: name,
			Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{
				Port:     s.entry.listener.Port,
				HostName: s.entry.ResourceName(),
				Hostname: s.entry.listener.Hostname,
			}},
		}
	}

	refs := make([]*aidataplanev1.EntryReference, 0, len(s.refs))
	for _, ref := range s.refs {
		refs = append(refs, &aidataplanev1.EntryReference{Port: ref.port, HostName: ref.name})
	}

	if s.model != nil {
		compiled := &aidataplanev1.ModelRule{
			Origin:    aiResourceOrigin(s.model.ObjectMeta),
			RuleIndex: uint32(s.ruleIndex),
			EntryRefs: refs,
			Outcome:   &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}},
		}
		for _, match := range s.model.rules[s.ruleIndex].matches {
			condition := &aidataplanev1.ModelMatch{Condition: &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}}
			if len(match.Headers) != 0 {
				condition.Condition = &aidataplanev1.ModelMatch_Exact{Exact: match.Headers[0].Value}
			}

			compiled.Matches = append(compiled.Matches, condition)
		}

		if s.modelTarget != nil {
			compiled.Outcome = &aidataplanev1.ModelRule_Target{Target: s.modelTarget}
		}

		return &aidataplanev1.Resource{Name: name, Payload: &aidataplanev1.Resource_ModelRule{ModelRule: compiled}}
	}

	compiled := &aidataplanev1.MCPRoute{
		Origin:    aiResourceOrigin(s.mcp.ObjectMeta),
		Path:      s.mcp.match.Path,
		EntryRefs: refs,
		Outcome:   &aidataplanev1.MCPRoute_Unavailable{Unavailable: &emptypb.Empty{}},
	}
	if s.mcpBackend != "" {
		compiled.Outcome = &aidataplanev1.MCPRoute_Target{Target: &aidataplanev1.MCPTarget{
			NetworkBackendName: s.mcpBackend,
			BackendPath:        s.mcp.match.BackendPath,
			TargetName:         s.mcpTarget,
		}}
	}

	return &aidataplanev1.Resource{Name: name, Payload: &aidataplanev1.Resource_McpRoute{McpRoute: compiled}}
}

func compareAIEntryReferences(a, b aiEntryReference) int {
	if order := cmp.Compare(a.port, b.port); order != 0 {
		return order
	}

	return cmp.Compare(a.name, b.name)
}
