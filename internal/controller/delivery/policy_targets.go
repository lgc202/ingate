package delivery

import (
	"cmp"
	"slices"

	"github.com/lgc202/ingate/internal/controller/compiler"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// affectedPolicyTargets 同时包含新配置目标和仍存在的旧目标，使策略移除也能获得发布结果
func affectedPolicyTargets(
	active *publishedConfig,
	resources []compiler.ResourceGeneration,
	desired []compiler.CompiledPolicyTarget,
) []compiler.CompiledPolicyTarget {
	resourceIndex := make(map[string]compiler.ResourceGeneration, len(resources))
	for _, resource := range resources {
		resourceIndex[resourceGenerationKey(resource.Kind, resource.Name)] = resource
	}

	resultSet := make(map[compiler.CompiledPolicyTarget]bool, len(desired))
	for _, target := range desired {
		resultSet[target] = true
	}
	if active != nil {
		for _, target := range active.policyTargets {
			policy, hasPolicy := resourceIndex[resourceGenerationKey(target.Policy.Kind, target.Policy.Name)]
			currentTarget, hasTarget := resourceIndex[resourceGenerationKey(target.Target.Kind, target.Target.Name)]
			if hasPolicy && hasTarget {
				resultSet[compiler.CompiledPolicyTarget{Policy: policy, Target: currentTarget}] = true
			}
		}
	}

	result := make([]compiler.CompiledPolicyTarget, 0, len(resultSet))
	for target := range resultSet {
		result = append(result, target)
	}
	slices.SortFunc(result, compareCompiledPolicyTarget)
	return result
}

func resourceGenerationKey(kind gatewayv1.Kind, name string) string {
	return string(kind) + "\x00" + name
}

func compareCompiledPolicyTarget(a, b compiler.CompiledPolicyTarget) int {
	if result := compareResourceGeneration(a.Policy, b.Policy); result != 0 {
		return result
	}
	return compareResourceGeneration(a.Target, b.Target)
}

func compareResourceGeneration(a, b compiler.ResourceGeneration) int {
	if result := cmp.Compare(a.Kind, b.Kind); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	if result := cmp.Compare(string(a.UID), string(b.UID)); result != 0 {
		return result
	}
	return cmp.Compare(a.Generation, b.Generation)
}
