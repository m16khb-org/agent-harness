package core

import coreinspect "agent-harness/internal/core/inspect"

type InspectInfo = coreinspect.InspectInfo
type SkillInfo = coreinspect.SkillInfo
type IntegrationStatus = coreinspect.IntegrationStatus

func InspectHarness(root, target, home, version, skillName string) InspectInfo {
	return coreinspect.InspectHarness(root, target, home, version, skillName)
}

func ListSkills(root, skillName string) []SkillInfo {
	return coreinspect.ListSkills(root, skillName)
}

func CodexMCPConfigured(path string) bool {
	return coreinspect.CodexMCPConfigured(path)
}

func exists(path string) bool {
	return coreinspect.Exists(path)
}
